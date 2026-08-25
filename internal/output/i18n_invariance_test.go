package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/doctor"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/version"
)

// TestJSONOutputLanguageInvariant is the safety net for the localization work
// in this package: --json output must be byte-for-byte identical regardless
// of the active locale, because it is produced via encoding/json on domain
// structs with static `json:"..."` tags that never route through i18n.T.
// If a future change accidentally sends a JSON field through i18n.T, this
// test fails.
func TestJSONOutputLanguageInvariant(t *testing.T) {
	restore := setTestLang(t)
	defer restore()

	renderers := []struct {
		name   string
		render func(w *bytes.Buffer) error
	}{
		{
			name: "positions",
			render: func(w *bytes.Buffer) error {
				return WritePositions(w, FormatJSON, testPositions)
			},
		},
		{
			name: "quote",
			render: func(w *bytes.Buffer) error {
				return WriteQuote(w, FormatJSON, testQuote)
			},
		},
		{
			name: "orders",
			render: func(w *bytes.Buffer) error {
				return WriteOrders(w, FormatJSON, testInvarianceOrders)
			},
		},
		{
			name: "watchlist",
			render: func(w *bytes.Buffer) error {
				return WriteWatchlist(w, FormatJSON, testWatchlistItems)
			},
		},
		{
			name: "accounts",
			render: func(w *bytes.Buffer) error {
				return WriteAccounts(w, FormatJSON, testInvarianceAccounts, "acct-primary")
			},
		},
		{
			name: "accountSummary",
			render: func(w *bytes.Buffer) error {
				return WriteAccountSummary(w, FormatJSON, testInvarianceAccountSummary)
			},
		},
		{
			name: "configStatus",
			render: func(w *bytes.Buffer) error {
				return WriteConfigStatus(w, FormatJSON, testInvarianceConfigStatus)
			},
		},
		{
			name: "doctorReport",
			render: func(w *bytes.Buffer) error {
				return WriteDoctorReport(w, FormatJSON, testInvarianceDoctorReport)
			},
		},
		{
			name: "trades",
			render: func(w *bytes.Buffer) error {
				return WriteTrades(w, FormatJSON, testInvarianceTradeList)
			},
		},
		{
			name: "priceLimits",
			render: func(w *bytes.Buffer) error {
				return WritePriceLimits(w, FormatJSON, testInvariancePriceLimits)
			},
		},
		{
			name: "stockWarnings",
			render: func(w *bytes.Buffer) error {
				return WriteStockWarnings(w, FormatJSON, testInvarianceStockWarnings)
			},
		},
		{
			name: "tradingHours",
			render: func(w *bytes.Buffer) error {
				return WriteTradingHours(w, FormatJSON, testInvarianceTradingHours)
			},
		},
		{
			name: "dividends",
			render: func(w *bytes.Buffer) error {
				return WriteDividends(w, FormatJSON, testInvarianceDividends)
			},
		},
		{
			name: "communityRanking",
			render: func(w *bytes.Buffer) error {
				return WriteCommunityRanking(w, FormatJSON, testInvarianceCommunityRanking)
			},
		},
		{
			name: "profitOverview",
			render: func(w *bytes.Buffer) error {
				return WriteProfitOverview(w, FormatJSON, dummyProfit())
			},
		},
		{
			name: "profitPeriod",
			render: func(w *bytes.Buffer) error {
				return WritePeriodProfit(w, FormatJSON, testInvariancePeriodProfit)
			},
		},
		{
			name: "profitDaily",
			render: func(w *bytes.Buffer) error {
				return WriteDailyProfit(w, FormatJSON, testInvarianceDailyProfit)
			},
		},
		{
			name: "overseasTransferIncome",
			render: func(w *bytes.Buffer) error {
				return WriteOverseasTransferIncome(w, FormatJSON, dummyTransferIncome())
			},
		},
	}

	for _, r := range renderers {
		t.Run(r.name, func(t *testing.T) {
			var enBuf, koBuf bytes.Buffer

			i18n.SetLang("en")
			if err := r.render(&enBuf); err != nil {
				t.Fatalf("render (en) failed: %v", err)
			}

			i18n.SetLang("ko")
			if err := r.render(&koBuf); err != nil {
				t.Fatalf("render (ko) failed: %v", err)
			}

			if enBuf.String() != koBuf.String() {
				t.Fatalf("JSON output differs by locale for %s:\nen=%q\nko=%q", r.name, enBuf.String(), koBuf.String())
			}
		})
	}
}

// TestCSVOutputLanguageInvariant is the CSV counterpart of the JSON
// invariance test: --csv output goes through encoding/csv with a fixed
// column-key header slice, never i18n.T.
func TestCSVOutputLanguageInvariant(t *testing.T) {
	restore := setTestLang(t)
	defer restore()

	renderers := []struct {
		name   string
		render func(w *bytes.Buffer) error
	}{
		{
			name: "positions",
			render: func(w *bytes.Buffer) error {
				return WritePositions(w, FormatCSV, testPositions)
			},
		},
		{
			name: "quote",
			render: func(w *bytes.Buffer) error {
				return WriteQuote(w, FormatCSV, testQuote)
			},
		},
		{
			name: "orders",
			render: func(w *bytes.Buffer) error {
				return WriteOrders(w, FormatCSV, testInvarianceOrders)
			},
		},
		{
			name: "transactions",
			render: func(w *bytes.Buffer) error {
				return WriteTransactions(w, FormatCSV, testTransactions)
			},
		},
		{
			name: "accounts",
			render: func(w *bytes.Buffer) error {
				return WriteAccounts(w, FormatCSV, testInvarianceAccounts, "acct-primary")
			},
		},
		{
			name: "accountSummary",
			render: func(w *bytes.Buffer) error {
				return WriteAccountSummary(w, FormatCSV, testInvarianceAccountSummary)
			},
		},
		{
			name: "configStatus",
			render: func(w *bytes.Buffer) error {
				return WriteConfigStatus(w, FormatCSV, testInvarianceConfigStatus)
			},
		},
		{
			name: "trades",
			render: func(w *bytes.Buffer) error {
				return WriteTrades(w, FormatCSV, testInvarianceTradeList)
			},
		},
		{
			name: "priceLimits",
			render: func(w *bytes.Buffer) error {
				return WritePriceLimits(w, FormatCSV, testInvariancePriceLimits)
			},
		},
		{
			name: "tradingHours",
			render: func(w *bytes.Buffer) error {
				return WriteTradingHours(w, FormatCSV, testInvarianceTradingHours)
			},
		},
		{
			name: "stockWarnings",
			render: func(w *bytes.Buffer) error {
				return WriteStockWarnings(w, FormatCSV, testInvarianceStockWarnings)
			},
		},
		{
			name: "dividends",
			render: func(w *bytes.Buffer) error {
				return WriteDividends(w, FormatCSV, testInvarianceDividends)
			},
		},
		{
			name: "communityRanking",
			render: func(w *bytes.Buffer) error {
				return WriteCommunityRanking(w, FormatCSV, testInvarianceCommunityRanking)
			},
		},
		{
			name: "profitDaily",
			render: func(w *bytes.Buffer) error {
				return WriteDailyProfit(w, FormatCSV, testInvarianceDailyProfit)
			},
		},
		{
			name: "overseasTransferIncome",
			render: func(w *bytes.Buffer) error {
				return WriteOverseasTransferIncome(w, FormatCSV, dummyTransferIncome())
			},
		},
	}

	for _, r := range renderers {
		t.Run(r.name, func(t *testing.T) {
			var enBuf, koBuf bytes.Buffer

			i18n.SetLang("en")
			if err := r.render(&enBuf); err != nil {
				t.Fatalf("render (en) failed: %v", err)
			}

			i18n.SetLang("ko")
			if err := r.render(&koBuf); err != nil {
				t.Fatalf("render (ko) failed: %v", err)
			}

			if enBuf.String() != koBuf.String() {
				t.Fatalf("CSV output differs by locale for %s:\nen=%q\nko=%q", r.name, enBuf.String(), koBuf.String())
			}
		})
	}
}

// TestTableOutputIsLocalized is a sanity check for the flip side of the
// invariant above: human table output (FormatTable) IS expected to change
// with locale, since that's the entire point of this package's i18n work.
// This guards against someone "fixing" the invariance tests by routing
// everything (including table headers) through a single hardcoded language.
func TestTableOutputIsLocalized(t *testing.T) {
	restore := setTestLang(t)
	defer restore()

	i18n.SetLang("en")
	var enBuf bytes.Buffer
	if err := WritePositions(&enBuf, FormatTable, testPositions); err != nil {
		t.Fatalf("render (en) failed: %v", err)
	}

	i18n.SetLang("ko")
	var koBuf bytes.Buffer
	if err := WritePositions(&koBuf, FormatTable, testPositions); err != nil {
		t.Fatalf("render (ko) failed: %v", err)
	}

	if enBuf.String() == koBuf.String() {
		t.Fatalf("expected table headers to differ between en and ko locales, got identical output")
	}
}

// testInvarianceOrders is synthetic order data (no real account data) used
// only to exercise the order-list renderer across locales.
var testInvarianceOrders = []domain.Order{
	{
		ID:             "ORD-TEST-0001",
		Symbol:         "005930",
		Name:           "삼성전자",
		Market:         "KOSPI",
		Side:           "BUY",
		Status:         "PENDING",
		Quantity:       10,
		FilledQuantity: 0,
		Price:          70000,
		OrderDate:      "2026-01-01",
	},
}

// testInvarianceAccounts is synthetic account metadata (no real account
// data) used only to exercise the accounts renderer across locales.
var testInvarianceAccounts = []domain.Account{
	{
		ID:          "ACCT-TEST-0001",
		DisplayName: "Test Account",
		Name:        "Test Account",
		Type:        "STOCK",
		Currency:    "KRW",
		Markets:     []string{"KR", "US"},
		Primary:     true,
	},
}

// testInvarianceAccountSummary is synthetic summary data (no real account
// data) used only to exercise the account-summary renderer across locales.
var testInvarianceAccountSummary = domain.AccountSummary{
	TotalAssetAmount:      12345678,
	EvaluatedProfitAmount: 123456,
	ProfitRate:            0.0123,
	OrderableAmountKRW:    1000000,
	OrderableAmountUSD:    500,
	Markets: map[string]domain.AccountMarketSummary{
		"KR": {
			Market:                "KR",
			PendingBuyOrderAmount: 0,
			EvaluatedAmount:       1000000,
			PrincipalAmount:       900000,
			EvaluatedProfitAmount: 100000,
			ProfitRate:            0.111,
			TotalAssetAmount:      1000000,
			OrderableAmountKRW:    500000,
			OrderableAmountUSD:    0,
		},
	},
}

// testInvarianceConfigStatus is synthetic config data (no real account data)
// used only to exercise the config-status renderer across locales.
var testInvarianceConfigStatus = config.Status{
	ConfigFile:          "/tmp/tossctl-test-config.json",
	Exists:              true,
	Schema:              "https://example.com/schema.json",
	SchemaVersion:       config.SchemaVersion,
	SourceSchemaVersion: config.SchemaVersion,
	LegacyFields:        []string{"legacy_field_a"},
	Trading: config.Trading{
		Place:                 false,
		Sell:                  false,
		Fractional:            false,
		Cancel:                false,
		Amend:                 false,
		AllowLiveOrderActions: false,
		DangerousAutomation:   config.DangerousAutomation{AcceptFXConsent: false},
	},
	UpdateCheck: config.UpdateCheck{Enabled: true},
}

// testInvarianceDoctorReport is synthetic diagnostics data (no real account
// data) used only to exercise the doctor renderer across locales.
var testInvarianceDoctorReport = doctor.Report{
	Version:   version.Info{Version: "0.0.0-test", Commit: "deadbeef", Date: "2026-01-01"},
	GoVersion: "go1.25",
	OS:        "darwin",
	Arch:      "arm64",
	Paths: config.Paths{
		ConfigDir:   "/tmp/config",
		CacheDir:    "/tmp/cache",
		ConfigFile:  "/tmp/config/config.json",
		SessionFile: "/tmp/config/session.json",
		LineageFile: "/tmp/config/lineage.json",
	},
	Config: testInvarianceConfigStatus,
	Auth: doctor.AuthReport{
		PythonBinary: "/usr/bin/python3",
		HelperDir:    "/tmp/auth-helper",
		Checks: []doctor.Check{
			{Name: "session", Status: doctor.CheckOK, Summary: "session is valid"},
		},
	},
	Checks: []doctor.Check{
		{Name: "config", Status: doctor.CheckOK, Summary: "config is valid"},
	},
	OpenAPISummary: "openapi: enabled, prefer=auto",
}

// testInvarianceTradeList is synthetic tick data (no real account data) used
// only to exercise the trades renderer across locales.
var testInvarianceTradeList = domain.TradeList{
	ProductCode: "A005930",
	Symbol:      "005930",
	Name:        "Test Corp",
	Trades: []domain.Trade{
		{Time: "09:00:01", Price: 70000, Volume: 10, TradeType: "BUY", CumulativeVolume: 10},
		{Time: "09:00:05", Price: 70100, Volume: 5, TradeType: "SELL", CumulativeVolume: 15},
	},
	FetchedAt: time.Unix(0, 0).UTC(),
}

// testInvariancePriceLimits is synthetic price-band data (no real account
// data) used only to exercise the price-limits renderer across locales.
var testInvariancePriceLimits = domain.PriceLimits{
	ProductCode: "A005930",
	Symbol:      "005930",
	Name:        "Test Corp",
	Date:        "2026-01-01",
	UpperLimit:  91000,
	LowerLimit:  49000,
}

// testInvarianceStockWarnings is synthetic warning-badge data (no real
// account data) used only to exercise the stock-warnings renderer across
// locales.
var testInvarianceStockWarnings = domain.StockWarnings{
	ProductCode: "A005930",
	Symbol:      "005930",
	Name:        "Test Corp",
	Warnings: []domain.StockWarning{
		{Type: "CAUTION", Title: "Caution", Text: "synthetic test warning"},
	},
	FetchedAt: time.Unix(0, 0).UTC(),
}

// testInvarianceTradingHours is synthetic session-window data (no real
// account data) used only to exercise the trading-hours renderer across
// locales.
var testInvarianceTradingHours = domain.TradingHours{
	KR:     domain.MarketSession{Date: "2026-01-01", StartTime: "09:00", EndTime: "15:30"},
	US:     domain.MarketSession{Date: "2026-01-01", StartTime: "22:30", EndTime: "05:00"},
	NextKR: domain.MarketSession{Date: "2026-01-02", StartTime: "09:00", EndTime: "15:30"},
	NextUS: domain.MarketSession{Date: "2026-01-02", StartTime: "22:30", EndTime: "05:00"},
}

// testInvarianceDividends is synthetic dividend data (no real account data)
// used only to exercise the dividends renderer across locales, including
// the KRW-suffix fix.
var testInvarianceDividends = domain.Dividends{
	Year:          2026,
	ByPaymentDate: false,
	Summary: domain.DividendSummary{
		Total:     domain.DividendAmount{KRW: 1234567, USD: 123.45},
		Paid:      domain.DividendAmount{KRW: 1000000, USD: 100},
		Estimated: domain.DividendAmount{KRW: 234567, USD: 23.45},
		Tax:       &domain.DividendAmount{KRW: 15000, USD: 1.5},
	},
	Regions: []domain.DividendRegion{
		{Region: "kr", Summary: domain.DividendSummary{Total: domain.DividendAmount{KRW: 1000000}}},
		{Region: "us", Summary: domain.DividendSummary{Total: domain.DividendAmount{USD: 123.45}}},
	},
	Monthly: []domain.DividendMonth{
		{Month: 1, Summary: domain.DividendSummary{Total: domain.DividendAmount{KRW: 100000}}},
		{Month: 2, Summary: domain.DividendSummary{Total: domain.DividendAmount{KRW: 0, USD: 0}}},
	},
	FetchedAt: time.Unix(0, 0).UTC(),
}

// testInvarianceCommunityRanking is synthetic leaderboard data (no real
// account data) used only to exercise the community-ranking renderer across
// locales, including the KRW-suffix fix.
var testInvarianceCommunityRanking = domain.CommunityRanking{
	Type: "TOP_10_PROFIT_ROSS_AMOUNT",
	Users: []domain.CommunityUser{
		{Rank: 1, Nickname: "test-user-1", UserProfileID: 1001, ProfitAmountKRW: 9876543, ProfitRate: 0.256},
		{Rank: 2, Nickname: "test-user-2", UserProfileID: 1002, ProfitAmountKRW: 5432100, ProfitRate: 0.123},
	},
	FetchedAt: time.Unix(0, 0).UTC(),
}

var testInvariancePeriodProfit = domain.PeriodProfit{
	Type: "sales", From: "20260101", To: "20260725",
	EarningAmount:  domain.DualCurrency{KRW: -1421, USD: usdPtr(-1.02)},
	EarningRate:    domain.DualCurrency{KRW: -66.9},
	PurchaseAmount: domain.DualCurrency{KRW: 2124},
	FetchedAt:      time.Unix(0, 0).UTC(),
}

var testInvarianceDailyProfit = domain.DailyProfit{
	From: "20260101", To: "20260725", Currency: "KRW",
	Stocks: []domain.DailyProfitStock{
		{Date: "2026-07-15", Symbol: "005930", Name: "삼성전자", Quantity: 10,
			ProfitLoss: domain.DualCurrency{KRW: -1000}, ProfitRate: -12.13, SellAmount: domain.DualCurrency{KRW: 1000}, BuyAmount: domain.DualCurrency{KRW: 2000}},
	},
	FetchedAt: time.Unix(0, 0).UTC(),
}

// TestDividendsTableNoRawKeys guards against i18n.T() rendering a raw
// catalog key when the resolved value is intentionally empty (e.g.
// output.dividends.monthSuffix is "" in en.json since English month
// numbers need no suffix, vs. "월" in ko.json). Before the T() fix, a
// present-but-empty catalog value was treated as a miss and T() returned
// the literal key string, which leaked into the English dividends table's
// "By month" rows (e.g. "1output.dividends.monthSuffix  100,000 KRW").
func TestDividendsTableNoRawKeys(t *testing.T) {
	restore := setTestLang(t)
	defer restore()

	i18n.SetLang("en")
	var enBuf bytes.Buffer
	if err := WriteDividends(&enBuf, FormatTable, testInvarianceDividends); err != nil {
		t.Fatalf("render (en) failed: %v", err)
	}
	enOut := enBuf.String()
	if strings.Contains(enOut, "output.dividends.monthSuffix") {
		t.Fatalf("en table leaked raw i18n key output.dividends.monthSuffix:\n%s", enOut)
	}
	if strings.Contains(enOut, "output.") {
		t.Fatalf("en table leaked a raw i18n key (contains \"output.\"):\n%s", enOut)
	}
	if !strings.Contains(enOut, "   1  100,000 KRW") {
		t.Fatalf("en table expected a bare month number (\"   1  100,000 KRW\") for month 1, got:\n%s", enOut)
	}

	i18n.SetLang("ko")
	var koBuf bytes.Buffer
	if err := WriteDividends(&koBuf, FormatTable, testInvarianceDividends); err != nil {
		t.Fatalf("render (ko) failed: %v", err)
	}
	koOut := koBuf.String()
	if strings.Contains(koOut, "output.dividends.monthSuffix") {
		t.Fatalf("ko table leaked raw i18n key output.dividends.monthSuffix:\n%s", koOut)
	}
	if !strings.Contains(koOut, "   1월  100,000원") {
		t.Fatalf("ko table expected \"1월\" for month 1, got:\n%s", koOut)
	}
}

// setTestLang saves/restores the active i18n language around a test so
// SetLang calls in this file don't leak into other tests in the package.
func setTestLang(t *testing.T) func() {
	t.Helper()
	prev := i18n.Lang()
	return func() {
		i18n.SetLang(prev)
	}
}
