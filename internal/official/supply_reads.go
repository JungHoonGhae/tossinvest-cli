package official

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// 종목 수급 (공식 Open API 1.2.13 신규, 2026-08-07).
//
// 다섯 경로가 파라미터(symbol/count/until)도 응답 봉투({nextUntil, records})도
// 같다. 그래서 요청·페이징은 supplySeries 하나가 처리하고 레코드 파싱만 갈린다 —
// 다섯 벌을 복붙하면 커서 처리 버그를 다섯 번 고쳐야 한다.

// nullable 숫자는 문자열 포인터로 받는다. 공식 API 는 금액·수량을 문자열로 주고
// (정밀도 보존), 당일 잠정 기록에서는 아예 null 을 준다. 0 으로 접으면 "순매수 0"
// 과 "아직 집계 안 됨" 이 구분되지 않는다.
type apiVolume struct {
	BuyVolume    string `json:"buyVolume"`
	SellVolume   string `json:"sellVolume"`
	NetBuyVolume string `json:"netBuyVolume"`
}

func (v *apiVolume) domain() *domain.TradingVolume {
	if v == nil {
		return nil
	}
	return &domain.TradingVolume{
		Buy:    parseDecimal(v.BuyVolume),
		Sell:   parseDecimal(v.SellVolume),
		NetBuy: parseDecimal(v.NetBuyVolume),
	}
}

type apiInstitutionVolume struct {
	apiVolume
	Breakdown *struct {
		FinancialInvestment       *apiVolume `json:"financialInvestment"`
		Insurance                 *apiVolume `json:"insurance"`
		Trust                     *apiVolume `json:"trust"`
		Bank                      *apiVolume `json:"bank"`
		OtherFinancialInstitution *apiVolume `json:"otherFinancialInstitution"`
		PensionFund               *apiVolume `json:"pensionFund"`
		PrivateEquityFund         *apiVolume `json:"privateEquityFund"`
	} `json:"breakdown"`
}

type apiCreditDetail struct {
	NewQuantity     string `json:"newQuantity"`
	ReturnQuantity  string `json:"returnQuantity"`
	BalanceQuantity string `json:"balanceQuantity"`
	BalanceRate     string `json:"balanceRate"`
	TradingRate     string `json:"tradingRate"`
}

func (d *apiCreditDetail) domain() *domain.CreditDetail {
	if d == nil {
		return nil
	}
	return &domain.CreditDetail{
		NewQuantity:     parseDecimal(d.NewQuantity),
		ReturnQuantity:  parseDecimal(d.ReturnQuantity),
		BalanceQuantity: parseDecimal(d.BalanceQuantity),
		BalanceRate:     parseDecimal(d.BalanceRate),
		TradingRate:     parseDecimal(d.TradingRate),
	}
}

// apiSupplyRecord is the union of all five record shapes. The server only sets
// the fields belonging to the series being fetched, so the rest stay nil and
// the adapter carries that nil through.
type apiSupplyRecord struct {
	Date      string `json:"date"`
	UpdatedAt string `json:"updatedAt"`

	Individual       *apiVolume            `json:"individual"`
	Foreigner        *apiVolume            `json:"foreigner"`
	Institution      *apiInstitutionVolume `json:"institution"`
	OtherCorporation *apiVolume            `json:"otherCorporation"`
	ForeignerHolding *struct {
		HoldingQuantity string `json:"holdingQuantity"`
		HoldingRate     string `json:"holdingRate"`
		LimitQuantity   string `json:"limitQuantity"`
	} `json:"foreignerHolding"`
	CFD *struct {
		BuyBalanceQuantity  string `json:"buyBalanceQuantity"`
		BuyBalanceRate      string `json:"buyBalanceRate"`
		SellBalanceQuantity string `json:"sellBalanceQuantity"`
		SellBalanceRate     string `json:"sellBalanceRate"`
	} `json:"cfd"`

	ShortSellingVolume     *string `json:"shortSellingVolume"`
	ShortSellingAmount     *string `json:"shortSellingAmount"`
	ShortSellingVolumeRate *string `json:"shortSellingVolumeRate"`
	ShortSellingAmountRate *string `json:"shortSellingAmountRate"`

	MarginLoan *apiCreditDetail `json:"marginLoan"`
	StockLoan  *apiCreditDetail `json:"stockLoan"`

	ExecutionQuantity *string `json:"executionQuantity"`
	RepaymentQuantity *string `json:"repaymentQuantity"`
	BalanceQuantity   *string `json:"balanceQuantity"`
	BalanceAmount     *string `json:"balanceAmount"`

	Arbitrage    *apiVolume `json:"arbitrage"`
	NonArbitrage *apiVolume `json:"nonArbitrage"`
}

type apiSupplyResponse struct {
	NextUntil *string           `json:"nextUntil"`
	Records   []apiSupplyRecord `json:"records"`
}

// supplyPaths maps a kind to its endpoint. Adding a sixth series later means
// one line here plus its record fields — nothing else.
var supplyPaths = map[domain.SupplyKind]string{
	domain.SupplyInvestor: "investor-trading",
	domain.SupplyShort:    "short-selling",
	domain.SupplyCredit:   "credit-trades",
	domain.SupplyLending:  "securities-lending",
	domain.SupplyProgram:  "program-trades",
}

// SupplyKinds lists the supported series, for callers that need to validate or
// enumerate them (CLI flag help, MCP parameter description).
func SupplyKinds() []domain.SupplyKind {
	return []domain.SupplyKind{
		domain.SupplyInvestor, domain.SupplyShort,
		domain.SupplyCredit, domain.SupplyLending, domain.SupplyProgram,
	}
}

// Supply fetches one supply series for a symbol. count is optional (0 uses the
// API default of 10); until is the cursor from a previous page's NextUntil.
//
// KR only — these series are KRX disclosures and the API has no equivalent for
// US symbols.
func (c *Client) Supply(ctx context.Context, symbol string, kind domain.SupplyKind, count int, until string) (domain.SupplySeries, error) {
	path, ok := supplyPaths[kind]
	if !ok {
		return domain.SupplySeries{}, fmt.Errorf("unknown supply kind %q", kind)
	}

	q := url.Values{}
	if count > 0 {
		q.Set("count", strconv.Itoa(count))
	}
	if until != "" {
		q.Set("until", until)
	}

	var raw apiSupplyResponse
	endpoint := "/api/v1/stocks/" + url.PathEscape(symbol) + "/" + path
	if err := c.get(ctx, endpoint, q, &raw); err != nil {
		return domain.SupplySeries{}, err
	}

	out := domain.SupplySeries{Symbol: symbol, Kind: kind, FetchedAt: time.Now().UTC()}
	if raw.NextUntil != nil {
		out.NextUntil = *raw.NextUntil
	}
	for _, r := range raw.Records {
		out.Records = append(out.Records, adaptSupplyRecord(r))
	}
	return out, nil
}

func adaptSupplyRecord(r apiSupplyRecord) domain.SupplyRecord {
	rec := domain.SupplyRecord{
		Date:              r.Date,
		UpdatedAt:         r.UpdatedAt,
		Individual:        r.Individual.domain(),
		Foreigner:         r.Foreigner.domain(),
		OtherCorporation:  r.OtherCorporation.domain(),
		MarginLoan:        r.MarginLoan.domain(),
		StockLoan:         r.StockLoan.domain(),
		Arbitrage:         r.Arbitrage.domain(),
		NonArbitrage:      r.NonArbitrage.domain(),
		ShortVolume:       decimalPtr(r.ShortSellingVolume),
		ShortAmount:       decimalPtr(r.ShortSellingAmount),
		ShortVolumeRate:   decimalPtr(r.ShortSellingVolumeRate),
		ShortAmountRate:   decimalPtr(r.ShortSellingAmountRate),
		LendingExecution:  decimalPtr(r.ExecutionQuantity),
		LendingRepayment:  decimalPtr(r.RepaymentQuantity),
		LendingBalanceQty: decimalPtr(r.BalanceQuantity),
		LendingBalanceAmt: decimalPtr(r.BalanceAmount),
	}
	if r.Institution != nil {
		rec.Institution = r.Institution.apiVolume.domain()
		if b := r.Institution.Breakdown; b != nil {
			rec.InstitutionSplit = &domain.InstitutionBreakdown{
				FinancialInvestment:       b.FinancialInvestment.domain(),
				Insurance:                 b.Insurance.domain(),
				Trust:                     b.Trust.domain(),
				Bank:                      b.Bank.domain(),
				OtherFinancialInstitution: b.OtherFinancialInstitution.domain(),
				PensionFund:               b.PensionFund.domain(),
				PrivateEquityFund:         b.PrivateEquityFund.domain(),
			}
		}
	}
	if f := r.ForeignerHolding; f != nil {
		rec.ForeignerHolding = &domain.ForeignerHolding{
			HoldingQuantity: parseDecimal(f.HoldingQuantity),
			HoldingRate:     parseDecimal(f.HoldingRate),
			LimitQuantity:   parseDecimal(f.LimitQuantity),
		}
	}
	if cf := r.CFD; cf != nil {
		rec.CFD = &domain.CFDBalance{
			BuyBalanceQuantity:  parseDecimal(cf.BuyBalanceQuantity),
			BuyBalanceRate:      parseDecimal(cf.BuyBalanceRate),
			SellBalanceQuantity: parseDecimal(cf.SellBalanceQuantity),
			SellBalanceRate:     parseDecimal(cf.SellBalanceRate),
		}
	}
	return rec
}

// decimalPtr keeps null distinct from zero: a nil string stays a nil float, so
// "not reported yet" never renders as 0.
func decimalPtr(s *string) *float64 {
	if s == nil {
		return nil
	}
	v := parseDecimal(*s)
	return &v
}
