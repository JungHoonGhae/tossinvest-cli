package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// 더미 값 — 실시세 아님.
func sampleCrypto() domain.CryptoPrices {
	return domain.CryptoPrices{Prices: []domain.CryptoPrice{{
		ProductCode: "VWAP.KRW-BTC", Symbol: "BTC",
		Close: 101000000, Change: 1000000, ChangeRate: 1.0,
		High: 102000000, Low: 99000000,
		Premium: -400000, PremiumRate: -0.4,
	}}}
}

func TestWriteCryptoPricesTableKeepsPremiumSign(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCryptoPrices(&buf, FormatTable, sampleCrypto()); err != nil {
		t.Fatalf("WriteCryptoPrices: %v", err)
	}
	out := buf.String()
	// 음수 프리미엄을 절대값으로 내면 "국내가 비싸다" 로 뒤집혀 읽힌다.
	if !strings.Contains(out, "-0.40%") {
		t.Errorf("premium sign lost:\n%s", out)
	}
	// 등락률은 이미 퍼센트다. ×100 이 끼면 100.00% 로 나온다.
	if !strings.Contains(out, "+1.00%") {
		t.Errorf("change rate wrong scale:\n%s", out)
	}
}

func TestWriteCryptoPricesEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCryptoPrices(&buf, FormatTable, domain.CryptoPrices{}); err != nil {
		t.Fatalf("WriteCryptoPrices: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("empty result printed nothing")
	}
}

func TestWriteCryptoPricesCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCryptoPrices(&buf, FormatCSV, sampleCrypto()); err != nil {
		t.Fatalf("WriteCryptoPrices: %v", err)
	}
	if !strings.Contains(buf.String(), "-0.4") {
		t.Errorf("CSV premium_rate missing sign:\n%s", buf.String())
	}
}
