# Task 6a Report — 7 Official Read Endpoints + Domain Adapters

## Files Changed

| File | Role |
|------|------|
| `internal/domain/models.go` | `domain.BuyingPower` 신규 타입 추가 |
| `internal/official/client.go` | `Client.accountSeq` 필드 + `WithAccountSeq` Option + `getAcct` 헬퍼 |
| `internal/official/order_reads.go` | BuyingPower 엔드포인트 + 어댑터 |
| `internal/official/order_reads_test.go` | BuyingPower 테스트 (단위 3 + 통합 1) |
| `internal/official/market_reads.go` | ExchangeRate + Prices + Orderbook + Trades + `parseDecimal` |
| `internal/official/market_reads_test.go` | 시세 4 엔드포인트 테스트 (단위 7 + 통합 4) |
| `internal/official/asset_reads.go` | Holdings 엔드포인트 + 어댑터 |
| `internal/official/asset_reads_test.go` | Holdings 테스트 (단위 2 + 통합 2) |
| `internal/official/stock_reads.go` | Stocks 엔드포인트 + 어댑터 |
| `internal/official/stock_reads_test.go` | Stocks 테스트 (단위 2 + 통합 1) |

---

## Per-Endpoint Mapping

### 1. BuyingPower — `GET /api/v1/buying-power`

| API field | Type | → | domain.BuyingPower | WTS ref |
|---|---|---|---|---|
| `cashBuyingPower` | string decimal | → | `CashBuyingPower float64` | `orderableAmountEnvelope.OrderableAmountKr.KRW` |
| `currency` | string enum | → | `Currency string` | WTS 직접 대응 없음 (KRW/USD 구분만 존재) |

**Zero-filled fields:** none (BuyingPower는 단순 2필드 응답).

**New domain type:** `domain.BuyingPower` — 기존 도메인에 closest match 없어 신규 추가.

**Account header:** `X-Tossinvest-Account` 필요. `getAcct` 헬퍼를 통해 `WithAccountSeq`로 설정한 값을 헤더에 주입.

---

### 2. ExchangeRate — `GET /api/v1/exchange-rate`

| API field | Type | → | domain.ExchangeRate | WTS ref |
|---|---|---|---|---|
| `baseCurrency`+`"/"`+`quoteCurrency` | string | → | `Code` | WTS `ExchangeRate.Code` ("USD" 단독) |
| `midRate` | string decimal | → | `Base` | WTS `ExchangeRate.Base` (이전 기준율) |
| `rate` | string decimal | → | `Close` | WTS `ExchangeRate.Close` (현재 환율) |
| `Name` | — | → | `""` | 미제공 — zero |
| `basisPoint`, `rateChangeType`, `validFrom`, `validUntil` | — | → | 미매핑 | 도메인 대응 필드 없음 |

---

### 3. Holdings — `GET /api/v1/holdings`

| API field | Type | → | domain.Position | WTS ref |
|---|---|---|---|---|
| `symbol` | string | → | `Symbol` | `stockSymbol` / `stockCode` |
| `name` | string | → | `Name` | `stockName` |
| `marketCountry` | string | → | `MarketType` | `product.MarketType` |
| `quantity` | string decimal | → | `Quantity` | `item.Quantity` |
| `averagePurchasePrice` | string decimal | → | `AveragePrice` | `coalesceMoney(purchasePrice)` |
| `lastPrice` | string decimal | → | `CurrentPrice` | `coalesceMoney(currentPrice)` |
| `marketValue.amount` | string decimal | → | `MarketValue` | `coalesceMoney(evaluatedAmount)` |
| `profitLoss.amount` | string decimal | → | `UnrealizedPnL` | `coalesceMoney(profitLossAmount)` |
| `profitLoss.rate` | string decimal | → | `ProfitRate` | `coalesceMoney(profitLossRate)` |
| `dailyProfitLoss.amount` | string decimal | → | `DailyProfitLoss` | `coalesceMoney(dailyProfitLossAmount)` |
| `dailyProfitLoss.rate` | string decimal | → | `DailyProfitRate` | `coalesceMoney(dailyProfitLossRate)` |

**Zero-filled:** `ProductCode`, `MarketCode` (미제공). `AveragePriceUSD` 등 USD-specific 필드 — official HoldingsItem은 currency별 단일값 제공.

---

### 4. Prices — `GET /api/v1/prices`

| API field | → | domain.Quote | WTS ref |
|---|---|---|---|
| `symbol` | → | `Symbol` | `stockPriceResult.ProductCode` |
| `lastPrice` | → | `Last` | `stockPriceResult.Close` |
| `currency` | → | `Currency` | 직접 |

**Zero-filled:** `Name`, `Market`, `MarketCode`, `Change`, `ChangeRate`, `Volume`, `Open/High/Low`, `High52w/Low52w` 등 — `/prices`는 최소 현재가만 제공.

---

### 5. Stocks — `GET /api/v1/stocks`

| API field | → | domain.Quote | WTS ref |
|---|---|---|---|
| `symbol` | → | `Symbol` | `stockInfoResult.Symbol` |
| `name` | → | `Name` | `stockInfoResult.Name` |
| `market` | → | `MarketCode` | `stockInfoResult.Market.Code` |
| `currency` | → | `Currency` | `stockInfoResult.Currency` |
| `status` | → | `Status` | `stockInfoResult.Status` |

**Zero-filled:** `Last`, `Change`, `Volume` 등 시세 필드 — `/stocks`는 참조 메타데이터 전용. `/prices`와 조합 필요.

---

### 6. Orderbook — `GET /api/v1/orderbook`

| API field | → | domain.OrderBook | WTS ref |
|---|---|---|---|
| `asks[].price` | → | `Offers[].Price` | WTS 호가 매도 side |
| `asks[].volume` | → | `Offers[].Volume` | 동일 |
| `bids[].price` | → | `Bids[].Price` | 직접 |
| `bids[].volume` | → | `Bids[].Volume` | 직접 |
| computed sum | → | `TotalOffer`, `TotalBid` | API 직접 제공 없음, 합산 계산 |

**Zero-filled:** `ProductCode`, `Name`, `Close` (미제공 in response body).

---

### 7. Trades — `GET /api/v1/trades`

| API field | → | domain.Trade | WTS ref |
|---|---|---|---|
| `timestamp` | → | `Time` | `domain.Trade.Time` |
| `price` | → | `Price` | 직접 |
| `volume` | → | `Volume` | 직접 |

**Zero-filled:** `TradeType`, `Base`, `CumulativeVolume` — official Trade schema 미제공.
`ProductCode`, `Name` — TradeList에 없음.

---

## TDD Evidence

```
# RED phase (each endpoint file) → build failed with undefined errors
# GREEN phase → all tests pass

go test ./internal/official/ -count=1
ok  github.com/JungHoonGhae/tossinvest-cli/internal/official  0.247s

go test ./... (full project — all pass)
```

**Test counts (new):**
- BuyingPower: 3 unit + 1 integration = 4
- ExchangeRate: 1 unit + 1 integration = 2
- Prices: 2 unit + 1 integration = 3
- Orderbook: 1 unit + 1 integration = 2
- Trades: 2 unit + 1 integration = 3
- Holdings: 2 unit + 2 integration = 4
- Stocks: 2 unit + 1 integration = 3
- **Total new: 21 tests**

(Pre-existing: 26 → total: 47)

---

## Commits

| SHA | Subject |
|-----|---------|
| `ec04bbb` | feat(official): buying-power 조회 + 어댑터 |
| `0ffdcf8` | feat(official): exchange-rate 조회 + 어댑터 *(prices/orderbook/trades 포함)* |
| `c0bd533` | feat(official): holdings 조회 + 어댑터 |
| `ebdfa00` | feat(official): stocks 조회 + 어댑터 |

**Note:** prices/orderbook/trades는 market_reads.go에 함께 작성되어 exchange-rate 커밋에 포함됨.

---

## Self-Review

- 모든 어댑터 함수는 pure (I/O 없음) — 단위 테스트로 완전 검증 가능
- `parseDecimal` 단일 정의 (market_reads.go) — 패키지 전체에서 공유
- 테스트 데이터: 합성 심볼(005930, AAPL, TSLA) + 더미 금액만 사용
- 기존 26 테스트 모두 유지
- `go build ./...` 클린

---

## Concerns

1. **X-Tossinvest-Account 헤더**: `BuyingPower`와 `Holdings`는 이 헤더가 필수이나, 메서드 시그니처에 accountSeq 파라미터를 넣지 않고 `WithAccountSeq` Client 옵션으로 처리함. 실제 사용 시 Client 생성 때 계좌 번호를 설정해야 함. 미설정 시 서버가 400 반환할 수 있음.

2. **market_reads.go 커밋 분리 미흡**: 4 엔드포인트(exchange-rate, prices, orderbook, trades)가 동일 커밋에 포함됨. "엔드포인트 별 커밋" 요건을 완전 충족하지 못함.

3. **Holdings USD-specific 필드**: official HoldingsItem은 currency별 단일값을 제공. WTS 어댑터는 KRW/USD 포인터 쌍을 coalesce. `AveragePriceUSD` 등 USD 전용 필드는 zero — 해외 종목 보유 시 추가 enrichment 필요.

4. **Trades.TradeType 누락**: official Trade schema에 BUY/SELL 방향 없음. WTS 체결 내역과 달리 방향 정보 불가.

5. **Prices/Stocks 중복 역할**: `/prices`는 최소 시세(Last+Currency), `/stocks`는 메타데이터(Name+Market). 완전한 Quote를 얻으려면 두 엔드포인트 조합이 필요 — 이는 official API 설계 의도에 따른 것.
