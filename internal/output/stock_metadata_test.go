package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func sampleStockMetadata() []domain.StockMetadata {
	nxtSuspended := false
	listDate := "1975-06-11"
	return []domain.StockMetadata{{
		Symbol: "005930", Name: "삼성전자", EnglishName: "Samsung Electronics",
		ISINCode: "KR7005930003", MarketCode: "KOSPI",
		SecurityType: "STOCK", CommonShare: true, Status: "ACTIVE",
		Currency: "KRW", SharesOutstanding: "5919637922", ListDate: &listDate,
		KoreanMarketDetail: &domain.KoreanMarketDetail{
			NXTSupported: true, NXTTradingSuspended: &nxtSuspended,
		},
		FetchedAt: time.Date(2026, 9, 1, 3, 4, 5, 0, time.UTC),
	}}
}

func TestWriteStockMetadataTableShowsIdentityAndStatus(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteStockMetadata(&buf, FormatTable, sampleStockMetadata()); err != nil {
		t.Fatalf("WriteStockMetadata: %v", err)
	}
	for _, want := range []string{"005930", "삼성전자", "KOSPI", "STOCK", "ACTIVE", "5919637922"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("table missing %q:\n%s", want, buf.String())
		}
	}
}

func TestWriteStockMetadataCSVHasStableCompleteColumns(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteStockMetadata(&buf, FormatCSV, sampleStockMetadata()); err != nil {
		t.Fatalf("WriteStockMetadata: %v", err)
	}
	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("decode CSV: %v", err)
	}
	wantHeader := []string{
		"symbol", "name", "english_name", "isin_code", "market_code",
		"security_type", "common_share", "status", "currency", "shares_outstanding",
		"list_date", "delist_date", "leverage_factor", "liquidation_trading",
		"nxt_supported", "krx_trading_suspended", "nxt_trading_suspended", "fetched_at",
	}
	if len(records) != 2 || !reflect.DeepEqual(records[0], wantHeader) {
		t.Fatalf("CSV shape = %#v, want header %#v and one row", records, wantHeader)
	}
	if records[1][3] != "KR7005930003" || records[1][9] != "5919637922" || records[1][14] != "true" {
		t.Fatalf("CSV metadata lost: %#v", records[1])
	}
}

func TestWriteStockMetadataJSONPreservesExactMetadata(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteStockMetadata(&buf, FormatJSON, sampleStockMetadata()); err != nil {
		t.Fatalf("WriteStockMetadata: %v", err)
	}

	var got []domain.StockMetadata
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 1 || got[0].ISINCode != "KR7005930003" || got[0].SharesOutstanding != "5919637922" {
		t.Fatalf("metadata lost in JSON: %+v", got)
	}
	if got[0].KoreanMarketDetail == nil || !got[0].KoreanMarketDetail.NXTSupported {
		t.Fatalf("Korean market detail lost in JSON: %+v", got[0])
	}
	for _, want := range []string{`"delist_date": null`, `"leverage_factor": null`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("nullable field missing from stable JSON shape: want %s\n%s", want, buf.String())
		}
	}
}
