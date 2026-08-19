# Push Events (SSE)

Verified 2026-04-23 from the authenticated dashboard with the tossctl session.

## Channel

Toss Securities **web (WTS)** does **not** use WebSocket — this is about the web session only. The official Open API gained a WebSocket surface on 2026-08-19 (`wss://openapi-ws.tossinvest.com/ws/v1`, AsyncAPI 3.0 spec); see `docs/reverse-engineering/change-analysis/2026-08-19.md`. Server → client push is delivered via Server-Sent Events (SSE) on a single long-lived HTTP GET.

- Endpoint: `GET https://sse-message.tossinvest.com/api/v1/wts-notification`
- Required headers: `Accept: text/event-stream`, `Cache-Control: no-cache`
- Auth: standard session cookies (`SESSION`, `XSRF-TOKEN`, `UTK`, `LTK`, `FTK`, `BTK`, `browserSessionId`, `deviceId`) — same set used for `wts-api` calls
- Response: `Content-Type: text/event-stream; charset=UTF-8`, chunked

A second push channel exists for OS-level Web Push (VAPID) via `GET https://wts-cert-api.tossinvest.com/api/v1/personalize/wts/browser-push/vapid` + a subscribe POST. Out of scope for the in-CLI listener.

## Stream Shape

On connect the server emits a `retry: 3600000` directive, then alternates SSE comment heartbeats (`:heartbeat`) with event frames. The server seems to re-emit the `retry` directive roughly every 2 seconds alongside heartbeats.

Frames follow the standard SSE format:

```
id: <32-hex-char opaque id>
data: <JSON payload>

```

`data` is a single-line JSON object with at minimum `type` and `key`.

## Observed Event Types

| `data.type` | Trigger | Payload |
| --- | --- | --- |
| `pending-order-refresh` | prepare/create/cancel/amend — any pending-list change | `{"key":"1"}` |
| `purchase-price-refresh` | buying-power / average-price change for a stock | `{"msg":{"stockCode":"..."},"key":"1"}` |
| `share-holdings` | holdings delta after a fill or external reconciliation | `{"msg":{"stockCode":"..."}}` |
| `web-push` | rich human-readable notification (order success, etc.) | `{"msg":{"title":"...","message":"...","iconUrl":"...","buttonInfo":{"name":"...","url":"..."},"contentId":"..."}}` |
| `order-refresh` | order state change (generic) | not captured live yet |
| `price-refresh` | ticker price refresh | not captured live yet |
| `icon-refresh` | icon asset refresh | not captured live yet |
| `setting-refresh` | user preference change | not captured live yet |

Cancel of a single pending order emits `pending-order-refresh` + `purchase-price-refresh` each three times (prepare/cancel/reconcile). A successful buy fill adds `share-holdings` and `web-push`. Successful sell fills produce the same pattern (including `web-push`) at execution time, but not at the order-registration moment.

Example frames captured live during a DELL buy:

```
{"type":"purchase-price-refresh","msg":{"stockCode":"US20181228002"},"key":"1"}
{"type":"pending-order-refresh","key":"1"}
{"type":"share-holdings","msg":{"stockCode":"US20181228002"}}
{"type":"web-push","msg":{"title":"델 테크놀로지스 1주 구매 성공","message":"주당 $213.4(315,319원)에 구매했어요.","iconUrl":"https://static.toss.im/icons/png/4x/icn-success-color.png","buttonInfo":{"name":"내역 보기","url":"/account/orders?..."},"contentId":"securities:WEBPUSH-..."}}
```

## Server-Initiated Reconnect (`event: connection-close`)

Toss emits a named SSE event `connection-close` every few minutes. A client that ignores this event and waits for the TCP stream to drop ends up in a loop of rapid disconnect/reconnect because the server is already handing the subscription off to a new connection.

When an SSE frame's `event:` field equals `connection-close`, immediately open a fresh stream without growing the backoff. The tossctl Go listener does this and bypasses the exponential backoff penalty for these graceful hand-offs.

## Semantics

Toss uses SSE as a **thin notification** channel — events signal "something changed, re-fetch the relevant resource" rather than carrying the payload. Consumers must pair each event type with a follow-up REST call to get the actual state.

| Event type | Re-fetch |
| --- | --- |
| `pending-order-refresh` | `/api/v1/trading/orders/histories/all/pending` |
| `purchase-price-refresh` | `/api/v1/trading/orders/calculate/<stockCode>/average-price` + orderable-amount / cost-basis endpoints as needed |
| `share-holdings` | portfolio position for the changed stockCode |
| `web-push` | no follow-up needed — the payload is already human-readable |

Unknown event types seen in the wild should be logged verbatim so the taxonomy can grow.

## Known Unknowns

- `order-refresh`, `price-refresh`, `icon-refresh`, `setting-refresh` — not yet observed live.
- Fill notifications for US during regular trading hours — captured only on pre-market order placements; outside-hours flow may differ.
- Whether Toss enforces a single-SSE-per-session limit — not yet directly confirmed.
- Long-idle behavior across the advertised `retry: 3600000` (1 h) window — in practice the server keeps emitting `connection-close` events well before that deadline.
- Payload size ceiling — no frame larger than ~400 bytes has been observed.
