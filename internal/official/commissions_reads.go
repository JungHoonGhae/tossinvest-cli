package official

import (
	"context"
	"net/url"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiCommission mirrors one element of the Commissions array.
// Endpoint: GET /api/v1/commissions
// Schema:
//
//	marketCountry  string — "KR" | "US"
//	commissionRate string — **소수 비율**. "0.00015" 가 0.015% 다.
//	startDate      string — effective from date
//	endDate        string — effective until date
//
// 스케일 주의: spec 1.2.14(2026-08-12)에서 이 필드의 표현이 바뀌었다. 그 전에는
// "수수료율 (%)" 로 0.015 가 곧 0.015% 였고, 이제는 소수 비율이라 0.00015 가
// 0.015% 다 — **같은 요율을 100배 다르게 쓴다.** 출력은 formatPercent(×100)를
// 태우므로 소수 비율 가정이 맞고, 그래서 이 파일의 주석이 원래 틀려 있었다
// ("0.015" (1.5%)). 스케일이 또 뒤집히면 에러 없이 100배 어긋나므로
// commissions_reads_test.go 와 marketdata_test.go 에 단언을 박아뒀다.
//
// 이 레포에는 규약이 다른 형제 필드가 있다: WTS `account commission` 의
// `RatePercent` 는 **이미 퍼센트**다(internal/client/commission_schedule.go).
// 둘을 섞지 말 것.
type apiCommission struct {
	MarketCountry  string `json:"marketCountry"`
	CommissionRate string `json:"commissionRate"`
	StartDate      string `json:"startDate"`
	EndDate        string `json:"endDate"`
}

// Commissions fetches the commission rate schedule applicable to symbol.
// Returns the first element of the array as a single domain.Commission value.
// If the response is empty, returns a zero-value Commission with FetchedAt set.
func (c *Client) Commissions(ctx context.Context, symbol string) (domain.Commission, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	var raw []apiCommission
	if err := c.get(ctx, "/api/v1/commissions", q, &raw); err != nil {
		return domain.Commission{}, err
	}
	return adaptCommissions(symbol, raw), nil
}

// adaptCommissions converts the official Commissions array to domain.Commission.
//
// Mapping rationale:
//
//   - symbol (caller-supplied) → Symbol.
//   - raw[0].commissionRate (decimal string) → CommissionRate: parseDecimal.
//     Only the first element is used; the API returns per-period entries but the
//     domain type represents a single current rate.
//   - TaxRate: not included in this endpoint's response; left 0.
//   - ProductCode, Name: not available; left empty.
func adaptCommissions(symbol string, raw []apiCommission) domain.Commission {
	var rate float64
	if len(raw) > 0 {
		rate = parseDecimal(raw[0].CommissionRate)
	}
	return domain.Commission{
		Symbol:         symbol,
		CommissionRate: rate,
		FetchedAt:      time.Now().UTC(),
		// TaxRate — not in /commissions response
		// ProductCode, Name — not available
	}
}
