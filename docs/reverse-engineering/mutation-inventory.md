# State-changing action inventory

This document separates actions that `tossctl` can safely call today from API
routes that were only discovered during static analysis. A route appearing in
the endpoint catalog does **not** make it callable.

The machine-readable source of truth for callable actions is:

```bash
tossctl ops list
tossctl ops describe <operation>
```

The paper rows below are opt-in operations. Enable them with
`tossctl config experimental paper-trading --enable`; they remain absent from
default ops/MCP discovery while the upstream feature is rolling out.

`ops list` marks callable writes; `ops describe` exposes each write's `mutation` policy containing its risk,
reversibility, authorization mode, preview and confirmation requirements,
opt-in requirements, and verification or unknown-outcome rule. An omitted `execute`
always produces a preview. Preference writes use `state_confirmation`, live
orders use `intent_confirmation`, and isolated paper writes use
`simulation_execute`. Live order tokens bind only the exact intent; they are
not fresh-state, expiring, or single-use credentials.

`requires_fresh_confirmation` means the token binds the currently affected
server state and exact intent, not that every token has a wall-clock expiry or
distributed single-use lock. Watchlist tokens additionally bind the WTS session
and expire after five minutes. Other state-confirmation workflows normally
invalidate a token when their bound state changes. Order tokens are different:
they are deterministic exact-intent confirmations and can be reused, so duplicate
submission remains possible if a caller executes the same token again.
These tokens authorize an exact intent but are not a cross-process single-use
lock. WTS watchlist mutations expose no verified idempotency key or conditional
write contract, so callers must not execute the same preview concurrently.

## Callable now

| Operation | Domain | Risk / reversibility | Execution boundary |
|---|---|---|---|
| `place_order` | Securities | financial / irreversible | official API config opt-in + preview + reusable exact-intent confirmation; response-only, so inspect pending/completed state before retry after a transport error |
| `cancel_order` | Securities | financial / irreversible | official API config opt-in + preview + reusable exact-intent confirmation; response-only, so inspect pending/completed state before retry after a transport error |
| `modify_order` | Securities | financial / irreversible | official API config opt-in + preview + reusable exact-intent confirmation; response-only, so inspect pending/completed state before retry after a transport error |
| `place_conditional_order` | Securities | financial / irreversible | official API config opt-in + preview + reusable exact-intent confirmation; response-only, so inspect conditional-order state before retry after a transport error |
| `cancel_conditional_order` | Securities | financial / irreversible | official API config opt-in + preview + reusable exact-intent confirmation; response-only, so inspect conditional-order state before retry after a transport error |
| `modify_conditional_order` | Securities | financial / irreversible | official API config opt-in + preview + reusable exact-intent confirmation; response-only, so inspect conditional-order state before retry after a transport error |
| `openapi_ip_replace_current` | System | preference / compensating | preview + state-bound confirmation + step verification + rollback on partial failure |
| `price_alert_add` | Securities | preference / reversible | preview + state-bound confirmation + exact tuple post-read |
| `price_alert_remove` | Securities | preference / reversible | preview + state-bound confirmation + exact tuple post-read |
| `hidden_holding_hide` | Securities | preference / reversible | preview + state-bound confirmation + account-scoped post-read |
| `hidden_holding_show` | Securities | preference / reversible | preview + state-bound confirmation + account-scoped post-read |
| `watchlist_group_create` | Securities | preference / reversible | preview + session-bound 5-minute confirmation + folder-list post-read |
| `watchlist_group_rename` | Securities | preference / reversible | preview + session-bound 5-minute confirmation + folder-list post-read |
| `watchlist_group_delete` | Securities | destructive / irreversible | preview binds folder membership + session-bound 5-minute confirmation + explicit irreversible acknowledgement + absence post-read |
| `watchlist_item_add` | Securities | preference / reversible | preview binds exact membership + session-bound 5-minute confirmation + membership post-read |
| `watchlist_item_remove` | Securities | preference / reversible | preview binds exact membership + session-bound 5-minute confirmation + membership post-read |
| `initialize_paper_trading` | Securities (paper) | simulation / unknown | preview + explicit `execute=true`; server initialization can still refuse independently |
| `deposit_paper_cash` | Securities (paper) | simulation / unknown | preview + explicit `execute=true` + paper-balance post-read; no matching withdrawal route is known |
| `place_paper_order` | Securities (paper) | simulation / irreversible-in-simulation | server prepare preview + explicit `execute=true`; dedicated `/paper/` create route only |
| `cancel_paper_order` | Securities (paper) | simulation / irreversible-in-simulation | pending-order preview + explicit `execute=true` + automatic pending-state absence check |
| `cancel_all_paper_orders` | Securities (paper) | simulation / irreversible-in-simulation | target-count preview + explicit `execute=true` + automatic `failed_count`/remaining-state comparison |

Financial writes are never safe to invoke automatically. A human must approve
every live order. Deleting a watchlist folder also requires
`acknowledge_irreversible=true` (CLI: `--acknowledge-irreversible`).

## Verified or partially verified, not callable yet

These are candidates for guarded implementation after the remaining request,
response, and live isolation contracts are verified.

| Area | Discovered actions | Current evidence / missing boundary |
|---|---|---|
| Securities portfolio folders | create, rename, delete, move, change folder items | Exact WTS method, path and body were recovered. The parsed internal model retains opaque keys in memory for future writes, but public CLI/JSON/CSV/MCP output omits them. Live post-write behavior is not verified. Delete is irreversible. |
| Watchlist ordering | reorder groups, move items between groups, reorder items | Method and route are present in the WTS bundle; exact body and response contract still need capture. |
| Holdings ordering | update flat holdings sort order | Route is present; key semantics and post-write representation still need verification. |
| Notification / AI agreement | update notification preferences and analysis agreement | Read state is implemented. The write routes need exact per-setting payload and post-read mapping before exposure. |
| Trading settings | simple-trade toggle, KRX/NXT venue choice, ATS notification | Read state is implemented. Prepare/signature steps and account scope differ by setting and are not yet fully verified. These can affect order behavior. |
| Paper education session | open, heartbeat, close, complete | The normal session protocol is statically recovered, but `/paper/init` currently returns an opaque 500 and `session/open` explicitly refuses uninitialized progress. No education-completion bypass is exposed. |

## High-impact actions intentionally withheld

The following discovered areas stay inventory-only even if a route is found.
They require a separate threat model, exact signing/authentication contract,
fresh human consent, and a reliable post-action receipt before implementation:

- Securities account closure and password/signing steps.
- Cash withdrawal, account transfer, external bank registration, and account
  administration.
- US dividend-option or voluntary corporate-action elections.
- Password, privacy, identity, terms, and contact-channel changes.
- Lending agreements or other regulated service applications.

General Toss Banking/MyData mobile actions are a separate domain. The current
WTS connector is a Toss Securities web-session connector; it must not be used
to imply that mobile Banking credentials, encrypted envelopes, or permissions
have been implemented.

## Automation mandates are not a global bypass

Future live-account automation will use a separate `bounded_mandate`
authorization mode. A mandate must bind allowed markets/products/sides, per
order and daily limits, an expiry, strategy identity, idempotency, audit
receipts, and a kill switch. It may explicitly pre-authorize named server
consents such as FX confirmation. It cannot bypass server-side eligibility,
identity verification, education, or terms acceptance. Ad-hoc live order
operations remain manual confirmation operations. Verified paper operations
use `simulation_execute`: preview by default and explicit `execute=true`, with
no live confirmation token or live-order config opt-in. Their client accepts
only dedicated `/paper/` routes and results carry `environment: paper`.

## Guardrail rules for future writes

1. Verify the exact method, host, path, headers, body, response, and account
   scope before adding a callable operation.
2. Read the affected state first and bind it into a short confirmation token.
3. Default to preview. Live and preference execution requires `execute=true`
   plus the token from a preview of the exact intent/state. Isolated simulation
   execution uses explicit `execute=true` without reusing live authorization.
4. Add config opt-in for financial or broadly consequential actions.
5. Require a separate irreversible acknowledgement when exact restoration is
   impossible.
6. Re-read the affected state after the request. Treat a transport error as
   reconciled success only when the desired state is observed unambiguously.
7. Register read-only dependency probes for unofficial WTS contracts.
8. Never let a generic `ops call` or MCP transport weaken a typed command's
   guardrails.
9. Do not claim exactly-once execution from a local confirmation token. Add a
   server idempotency key or conditional mutation contract before concurrent
   automation is allowed.
