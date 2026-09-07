# Trading Order State Machine

Initial observations captured from the TSLL order page on 2026-03-11.

## CLI result confirmation

The CLI shares one read-only reconciliation path for place, amend and cancel.
It polls pending/completed history at most eight times, 250 ms apart; it never
resubmits a mutation. A disappeared pending order without a canceled history row
returns `unknown`, not `canceled`. Users must inspect history before retrying.

Matching checks symbol, buy/sell direction, market when present, price and
quantity. Multiple matching rows remain `unknown` rather than selecting the first
order. Amendment matching excludes the original order and canceled rows.
Matching without a broker-provided correlation identifier remains best-effort;
these checks are not an idempotency guarantee for concurrent identical orders.

## Current Working Model

The web order page appears to build trading state in layers.

### 1. Enter Order Page

Transition:

- authenticated account page
- stock order page `/stocks/US20220809012/order`

Observed network:

- trading status
- prerequisite
- pending/completed order history
- sellable quantity
- cost-basis elements
- buy-side `order-data` calculations

Interpretation:

- the page assembles enough state to preview both buy and sell before the user submits anything

### 2. Buy Preview Ready

Observed UI:

- `구매`
- `지정가`
- default price prefilled
- quantity empty

Observed side effects:

- two `order-data` POST calculations during load

Interpretation:

- the page primes a default buy preview without waiting for explicit user entry

### 3. Quantity Entered

Observed UI after entering quantity:

- total order amount recalculated
- buy-after estimate updated locally

Observed result:

- UI changed immediately
- no additional preview request was isolated from quantity change alone

Interpretation:

- either quantity recalculation is local after page bootstrapping
- or the relevant network trigger is tied to a different event boundary than simple fill/blur

### 4. Sell Preview Ready

Observed transition:

- switching from `구매` to `판매`
- setting quantity to `1`

Observed UI:

- placeholder changed to `최대 114주 가능`
- summary fields changed to:
  - `현재 수익`
  - `예상 수익률`
  - `예상 손익`
  - `총 금액`

Observed network:

- prerequisite re-fetch
- cost-basis-elements fetch

Interpretation:

- sell mode depends on cost basis and position data more explicitly than buy mode

### 5. Pre-Submit Product Risk Gate

Observed transition:

- buy mode
- quantity set to `1`
- `구매하기` clicked

Observed result:

- no submit mutation captured
- a leveraged or inverse ETP disclosure dialog blocked further progress

Interpretation:

- some products insert an acknowledgement gate between preview and submit
- the submit state machine likely includes:
  - preview-ready
  - product-risk-ack-required
  - prepare
  - confirmation
  - submit

### 6. Server-Side Prepare Rejection

Observed transition:

- buy mode
- quantity `1`
- `구매하기`
- product-risk acknowledgement via `확인했어요`

Observed network:

- `GET /api/v1/account/investment-propensity/eligible/with-contract?financialQualification=USA_STOCK&highRiskTradingCategory=ETF`
- `GET /api/v1/product-eligibility/overseas-leverage-etp/trading`
- `POST /api/v2/wts/trading/order/prepare` -> `422`

Observed UI:

- alert title: `계좌 잔액이 부족해요`
- alert body: `구매를 위해 21,511원을 채울게요.`
- actions:
  - `닫기`
  - `모바일에서 채우기`

Interpretation:

- the first server-side mutation candidate is `order/prepare`
- Toss performs product eligibility checks immediately before `prepare`
- insufficient funds are enforced at `prepare` and surfaced through a dedicated alertdialog, not an inline form error
- the user may then leave the panel into a funding or recovery flow before retrying

### 7. Successful Prepare And Create

Observed transition:

- user manually completed a TSLL buy order in the browser
- user-reported intermediate steps:
  - insufficient-balance alert fired first
  - mobile push was used to top up balance
  - browser later asked whether FX conversion should be executed
  - user approved FX conversion
  - order then proceeded

Observed network:

- `POST /api/v2/wts/trading/order/prepare` -> `200`
- `POST /api/v2/wts/trading/order/create` -> `200`
- `POST /api/v1/trading/settings/toggle` -> `200`

Observed reconciliation:

- `GET /api/v1/trading/orders/histories/all/pending`
- `GET /api/v1/trading/orders/histories/PENDING?...`
- `GET /api/v1/trading/orders/calculate/US20220809012/orderable-quantity/sell?forceFetch=true`
- `GET /api/v1/trading/orders/calculate/US20220809012/average-price?forceFetch=true`

Observed UI:

- modal:
  - `TSLL`
  - `구매 완료`
  - `1주 가격 21,208원`
  - `구매 수량 1주`
  - `총 구매 금액 21,208원`
- completed-history row:
  - `3.11 TSLL 주당 21,208원 1주 구매 완료`

Interpretation:

- `prepare` is not merely validation; it gates the transition to `create`
- `create` is the live order mutation endpoint
- a successful retry after insufficient funds may require a funding branch and an FX-consent branch that were not fully isolated in network capture
- successful reconciliation must check both pending and completed history because a marketable order can disappear from pending immediately

### 8. Pending Order Visible

Observed transition:

- user manually placed a TSLL buy order at a low limit price

Observed state:

- CLI order:
  - `orderPrice: 1000`
  - `quantity: 1`
  - `pendingQuantity: 1`
  - `status: 체결대기`
- order page `주문내역 > 대기` showed a pending TSLL row with:
  - `주당 1,000원`
  - `1주 구매`
  - inline actions `수정`, `취소`
  - page action `전체 취소`

Interpretation:

- not all successful `create` calls fill immediately
- pending-state reconciliation must remain part of the post-create flow

### 8b. Place Body Captured

Observed transition:

- a later TSLL buy was submitted at `500원`
- the order remained pending after `prepare -> create`

Captured request bodies:

- `order/prepare`
  - `stockCode: US20220809012`
  - `market: NSQ`
  - `currencyMode: KRW`
  - `tradeType: buy`
  - `price: 0.3395`
  - `quantity: 1`
  - `orderPriceType: 00`
  - `allowAutoExchange: true`
  - `marginTrading: false`
  - `withOrderKey: true`
- `order/create`
  - same core fields as `prepare`
  - plus `extra.close`
  - plus `extra.closeKrw`
  - plus `extra.exchangeRate`
  - plus `extra.orderMethod`

Observed reconciliation:

- pending order became `orderNo: 16`
- `orderPrice: 500`
- `orderUsdPrice: 0.3395`
- `status: 체결대기`

Interpretation:

- place follows the same `prepare adds order key, final mutation omits it` pattern as amend and single cancel
- the order price sent to the broker is USD precision even though the dialog input is KRW
- a narrow CLI implementation can reconstruct the body from:
  - normalized order intent
  - stock-info market code
  - stock close
  - USD base exchange rate

### 9. Bulk Cancel Succeeded

Observed transition:

- user clicked `전체 취소` while the TSLL pending order was visible

Observed network:

- `POST /api/v3/wts/trading/order/bulk-cancel/prepare` -> `200`
- `POST /api/v3/wts/trading/order/bulk-cancel` -> `200`
- repeated `GET /api/v1/trading/orders/histories/PENDING?...`

Observed UI:

- `대기중인 주문이 없어요`

Observed CLI:

- `orders list` returned `[]`

Interpretation:

- bulk cancel has its own preflight and mutation stages
- the order panel reconciles against pending history after cancellation
- single-order cancel may still be a separate flow, but bulk cancel is sufficient for an initial cancel implementation

### 10. Amend Succeeded And Pending Order Reconciled

Observed transition:

- user opened the pending TSLL order from `주문내역 > 대기`
- user clicked `수정`
- order price changed from `1,000원` to `500원`

Observed network:

- `GET /api/v3/trading/order/4/available-actions?stockCode=US20220809012&tradeType=buy&orderPriceType=00&fractional=false&isReservationOrder=false` -> `200`
- `POST /api/v2/wts/trading/order/correct/prepare/2026-03-11/4` -> `200`
- `POST /api/v2/wts/trading/order/correct/2026-03-11/4` -> `200`
- repeated `GET /api/v1/trading/orders/histories/PENDING?...`

Observed UI:

- pending row remained visible after amend
- page snapshot still showed inline `수정` and `취소`
- amended price was visible as `500원`

Observed CLI:

- the pending order remained in `체결대기`
- `orderPrice` became `500`
- `pending_buy_order_amount` dropped from `1001` to `500`

Interpretation:

- amend follows the same `prepare -> mutation -> reconciliation` pattern as place and bulk cancel
- a successful amend does not imply a fill or a cancel; it returns to the pending state
- the `correct/.../4` path key did not match the later pending order number exposed by the CLI, so amend may replace or rekey the pending order record
- implementations should re-fetch pending orders after amend and avoid assuming the pre-amend identifier remains stable

### 10b. Amend Body Captured

Observed transition:

- pending order `13` at `600원`
- `수정` dialog opened
- price changed to `700원`
- `수정하기` clicked

Observed request bodies:

- `correct/prepare` included:
  - `market: NSQ`
  - `currencyMode: KRW`
  - `tradeType: buy`
  - `price: 0.4753`
  - `quantity: 1`
  - `orderPriceType: 00`
  - `withOrderKey: true`
- `correct` repeated the same body except `withOrderKey`

Observed reconciliation:

- pending order survived as a new row with `orderNo: 14`
- price became `700`
- status remained `체결대기`

Interpretation:

- amend payload uses USD price precision even when the dialog input is KRW
- the body can be reconstructed from:
  - pending order raw fields
  - stock-info market code
  - USD base exchange rate
- amend reconciliation should expect identifier rollover from the original order number to a new pending order number

### 11. Inline Cancel Succeeded

Observed transition:

- user opened the amended TSLL pending order from `주문내역 > 대기`
- user clicked inline `취소`

Observed network:

- `GET /api/v3/trading/order/5/available-actions?stockCode=US20220809012&tradeType=buy&orderPriceType=00&fractional=false&isReservationOrder=false` -> `200`
- `POST /api/v2/wts/trading/order/cancel/prepare/2026-03-11/5` -> `200`
- `POST /api/v3/wts/trading/order/cancel/2026-03-11/5` -> `200`
- repeated `GET /api/v1/trading/orders/histories/PENDING?...`

Observed UI:

- `대기중인 주문이 없어요`

Observed CLI:

- `orders list` returned `[]`

Interpretation:

- inline cancel is distinct from bulk cancel and uses its own `prepare -> cancel -> reconcile` chain
- unlike amend, the cancel path key matched the currently visible pending order identifier
- the client should still confirm cancel success through pending-history reconciliation rather than trusting the mutation response alone

### 11b. Inline Cancel Body Captured

Observed transition:

- pending order `14` remained open at `700원`
- the browser showed a confirmation dialog `TSLL 구매를 취소할까요?`
- `취소하기` was pressed from that dialog

Captured request bodies:

- `cancel/prepare`
  - `stockCode: US20220809012`
  - `tradeType: buy`
  - `quantity: 1`
  - `isAfterMarketOrder: false`
  - `isReservationOrder: false`
  - `withOrderKey: true`
- `cancel`
  - same body except `withOrderKey`

Interpretation:

- inline cancel needs a request body built from the current pending order metadata
- cancel follows the same `prepare adds order key, final mutation omits it` pattern as amend
- cancel reconciliation still ends in the same empty-pending state after the confirmation dialog completes

## Provisional State Graph

```text
entered-page
  -> buy-preview-bootstrapped
  -> buy-quantity-updated
  -> sell-preview-bootstrapped
  -> product-risk-ack-required
  -> prepare-rejected-insufficient-balance
  -> top-up-or-fx-approval-required?
  -> prepare-succeeded
  -> create-succeeded
  -> accepted-pending
  -> amend-available-actions-loaded
  -> correct-prepared
  -> correct-succeeded
  -> pending-order-reconciled
  -> cancel-available-actions-loaded
  -> single-cancel-prepared
  -> single-cancel-succeeded
  -> bulk-cancel-prepared
  -> bulk-cancel-succeeded
  -> filled-and-completed-history-visible
```

## Not Yet Captured

- request body for successful `order/prepare`
- request body for successful `order/create`
- explicit funding or FX approval requests between insufficient-balance rejection and successful retry
- request bodies for `bulk-cancel/prepare` and `bulk-cancel`
- rejection transition
- request bodies for `cancel/prepare` and `cancel`
- amend rejection transition
- ambiguous timeout transition
