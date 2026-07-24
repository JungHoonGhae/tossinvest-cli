package client

import "testing"

func TestNormalizeChartInterval(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"1m", "min:1", false},
		{"3m", "min:3", false},
		{"5m", "min:5", false},
		{"10m", "min:10", false},
		{"15m", "min:15", false},
		{"30m", "min:30", false},
		{"60m", "min:60", false},
		{"60min", "min:60", false},
		{"5MIN", "min:5", false}, // case-insensitive
		{" 3m ", "min:3", false}, // trimmed
		{"", "", true},
		{"2m", "", true},  // not in whitelist
		{"7m", "", true},  // not in whitelist
		{"1h", "", true},  // wrong unit
		{"abc", "", true}, // not a number
	}
	for _, c := range cases {
		got, err := normalizeChartInterval(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("normalizeChartInterval(%q): wantErr=%v, gotErr=%v", c.in, c.wantErr, err)
		}
		if got != c.want {
			t.Errorf("normalizeChartInterval(%q): want %q, got %q", c.in, c.want, got)
		}
	}
}

func TestDeriveSecurityType(t *testing.T) {
	cases := []struct {
		productCode string
		want        string
	}{
		{"A005930", "kr-s"},  // 삼성전자 — A + 6 digits
		{"A294400", "kr-s"},  // KODEX 인버스
		{"AAPL", "us-s"},     // 미국 티커 — A 로 시작하지만 영문 포함
		{"TSLA", "us-s"},     // A 로 시작 안 함
		{"MSFT", "us-s"},     // A 로 시작 안 함
		{"A", "us-s"},        // 너무 짧음 → kr 휴리스틱 실패 → us
		{"A12345", "us-s"},   // A + 5 digits — 너무 짧음 (>=7 요구)
		{"A1234567", "kr-s"}, // A + 7 digits → kr
		{"US00000000000", "us-s"},
	}
	for _, c := range cases {
		got := deriveSecurityType(c.productCode)
		if got != c.want {
			t.Errorf("deriveSecurityType(%q): want %q, got %q", c.productCode, c.want, got)
		}
	}
}
