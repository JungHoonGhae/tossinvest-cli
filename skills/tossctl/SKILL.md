---
name: tossctl
description: >-
  Use tossctl — the unofficial Toss Securities (토스증권) CLI and MCP server — to
  query and trade a Toss Securities account from the terminal or an agent. Use
  this skill WHENEVER a task involves Toss Securities / 토스증권 / tossctl /
  tossinvest: portfolio, holdings, orders, market data, quotes, order book,
  sectors, themes, AI signals, screener, watchlist (관심종목), realized profit
  (실현손익), dividends, tax, securities lending (대주), stock accumulation
  (주식모으기), conditional or automated orders — any read or trade against a
  Toss Securities account, even when the user does not name tossctl. It covers
  operation discovery (the MCP list→describe→call catalog, or the CLI), auth
  setup, the read-first + trading-gated safety model, and the rule to never
  expose real account numbers or balances.
---

# tossctl

`tossctl` is an unofficial CLI (and stdio MCP server) for **Toss Securities
(토스증권)**. It reuses a logged-in Toss web session (WTS) and/or official Open
API credentials so you can read account/market data and place a limited set of
trades from a terminal or an agent. It is read-first by design; trading is off
unless explicitly enabled.

Two surfaces expose the **same operation catalog**:

- **MCP** (preferred for agents): three tools — `list_operations`,
  `describe_operation`, `call_operation`.
- **CLI**: `tossctl <group> <command>` (e.g. `tossctl portfolio holdings`), plus
  `tossctl ops list|describe|call` for the raw catalog.

Prefer the catalog over guessing endpoints or paths. The catalog is the single
source of truth for what exists and how to call it.

## 1. Check auth first

Every task starts by confirming which credentials are connected, because most
operations need one.

- MCP: `call_operation` with `{"operation":"auth_status"}`.
- CLI: `tossctl auth status`.

Credential channels (an operation's `backend` field says which it needs):

| backend | needs | how to connect |
|---|---|---|
| `wts` | Toss **web session** | `tossctl auth login` (phone-approval or QR; the user approves on their phone) |
| `official` | Toss **Open API** key | `tossctl openapi login` |
| `auto` | either (official first, web-session fallback) | whichever is available |
| `none` | nothing (local) | — |

If a needed channel is missing, tell the user the exact command to run. **You
cannot complete the login for the user** — it needs their identity info and a
phone/OAuth approval. Hand it back: "run `tossctl auth login` and approve on your
phone." A WTS session can be short-lived unless the user confirms "keep me signed
in on this device" during approval.

## 2. Discover the operation

Do not hardcode endpoints. Find the operation id, read its schema, then call it.

1. **Find it** — `list_operations` (optionally with a `query` to filter), or
   `tossctl ops list`. The MCP result may be truncated (it notes
   `_omitted_items`); when you need the complete list, use `tossctl ops list`,
   which is authoritative.
2. **Read its schema** — `describe_operation {"operation":"<id>"}` or
   `tossctl ops describe <id>` for required/optional params, backend, and
   whether it is a write.
3. **Call it** — `call_operation {"operation":"<id>", ...params}` or
   `tossctl ops call <id> --param value`. `call_operation` accepts optional
   fields to select result JSON paths before size limits.

See `references/operations.md` for the full catalog grouped by area (generated
from the live registry — treat it as a map, but confirm each operation's current
schema with `describe_operation`).

## 3. Reads are safe; writes are gated

Reads (holdings, prices, profit, watchlist, …) return data directly. Writes are
deliberately hard to do by accident:

- **Every write returns a preview first.** To actually execute, you pass
  `execute: true` and then a **confirm token** returned by the preview. One call
  never both previews and commits.
- **Order writes** (place/cancel/modify, conditional orders) additionally require
  the user to have **opted into trading** in `config.json`. If trading is not
  enabled, a preview still works but execution is refused — tell the user to
  enable it rather than trying to force it.
- **Destructive or irreversible writes** may require an explicit
  acknowledgement flag.
- Inspect an operation's mutation policy (risk, reversibility, opt-in,
  verification) **before** executing. Never chain preview→execute automatically
  for a financial mutation; surface the preview and let the user decide.

Paper trading (`simulation_execute`) is isolated and needs `execute: true`
without a live confirm token — use it to demonstrate order flows safely.

## 4. Never expose real account data

This is a hard rule, not a preference. Account numbers, holder names, balances,
and realized amounts are private.

- Do not print real account numbers, holder names, or balances into shared
  output — chat you expect to be logged, commits, PRs, issues, test fixtures.
- Many operations mask by default (account numbers show masked unless
  `full=true`); keep them masked unless the user explicitly asks to see them for
  their own use, and even then do not persist the values.
- When building tests or docs, use **synthetic/dummy data**, never a real
  response captured live.

## 5. Common tasks → where to look

| The user wants… | Start with |
|---|---|
| Current holdings / account value | `holdings`, `account_summary`, `account_overview` |
| A grouped, fee-aware portfolio | `portfolio_folders`, `portfolio_performance`, `portfolio_snapshot(s)` |
| Realized profit / dividends / tax | `profit_overview`, `profit_period`, `profit_daily`, `dividends`, `tax_overseas` |
| A price / quote / order book | `prices`, `stocks`, `quote_*`, `orderbook`, `candles`, `trades` |
| Market structure (sectors, themes, rankings) | `sectors`, `sector_*`, `theme_rankings`, `stock_ranking`, `investor_rankings` |
| AI signals / reasoning | `ai_signals`, `ai_signal_detail`, `quote_reasoning`, `news_briefing` |
| Watchlist (관심종목) | `watchlist`, `watchlist_groups`, `watchlist_news` |
| Screening | `screener_presets`, `screener_filter_ranges` |
| Securities lending (대주) | `lending_expected`, `lending_top_revenue` |
| Stock accumulation (주식모으기) | `accumulation_plans`, `accumulation_status` |
| Place/cancel/modify an order | `place_order`, `cancel_order`, `modify_order` (gated; preview first) |
| Search a stock by name/ticker | `search_stocks` → feed the product code to other ops |

When unsure which operation fits, `list_operations` with a `query` (e.g.
`"dividend"`, `"order book"`, `"관심종목"`) narrows it fast.

## Caveats

- tossctl depends on Toss's **internal web APIs**, which can change without
  notice. If an operation starts failing, re-check auth, then
  `describe_operation`; the catalog is regenerated from the live app.
- This is **not** general Toss Banking/Card/MyData — those use a separate mobile
  app session and are not callable here. tossctl is Securities (WTS) + the
  official Open API only.
- The Open API covers a subset of what the web session can read; `auto` and
  `wts` operations fill the gaps. `backend` on each operation tells you which
  channel answers.
