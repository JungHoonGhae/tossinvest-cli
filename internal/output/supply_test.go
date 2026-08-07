package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func fp(v float64) *float64 { return &v }

// 미집계(nil)와 0 을 같은 칸으로 찍으면 수급 신호가 정반대로 읽힌다.
func TestWriteSupplyDistinguishesNilFromZero(t *testing.T) {
	s := domain.SupplySeries{Symbol: "005930", Kind: domain.SupplyInvestor, Records: []domain.SupplyRecord{
		{Date: "2026-01-05", Individual: nil, Institution: &domain.TradingVolume{NetBuy: 0}},
	}}
	var buf bytes.Buffer
	if err := WriteSupplySeries(&buf, FormatTable, s); err != nil {
		t.Fatalf("WriteSupplySeries: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "-") {
		t.Errorf("nil not rendered as '-':\n%s", out)
	}
	if !strings.Contains(out, " 0 ") && !strings.Contains(out, "0\n") {
		t.Errorf("zero net-buy lost:\n%s", out)
	}
}

// 커서가 있으면 알려줘야 한다 — 없으면 더 볼 수 있는지 알 방법이 없다.
func TestWriteSupplyShowsCursor(t *testing.T) {
	s := domain.SupplySeries{Symbol: "005930", Kind: domain.SupplyShort, NextUntil: "2026-01-01",
		Records: []domain.SupplyRecord{{Date: "2026-01-05", ShortVolume: fp(1000)}}}
	var buf bytes.Buffer
	if err := WriteSupplySeries(&buf, FormatTable, s); err != nil {
		t.Fatalf("WriteSupplySeries: %v", err)
	}
	if !strings.Contains(buf.String(), "2026-01-01") {
		t.Errorf("cursor not surfaced:\n%s", buf.String())
	}
}

func TestWriteSupplyEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSupplySeries(&buf, FormatTable, domain.SupplySeries{Kind: domain.SupplyProgram}); err != nil {
		t.Fatalf("WriteSupplySeries: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("empty series printed nothing")
	}
}

// 다섯 종류 모두 표·CSV 형식이 있어야 한다. 하나라도 빠지면 런타임에 포맷 문자열이
// 그대로 찍힌다.
func TestWriteSupplyAllKindsRender(t *testing.T) {
	for _, k := range []domain.SupplyKind{
		domain.SupplyInvestor, domain.SupplyShort, domain.SupplyCredit,
		domain.SupplyLending, domain.SupplyProgram,
	} {
		s := domain.SupplySeries{Symbol: "005930", Kind: k, Records: []domain.SupplyRecord{{Date: "2026-01-05"}}}
		for _, f := range []Format{FormatTable, FormatCSV, FormatJSON} {
			var buf bytes.Buffer
			if err := WriteSupplySeries(&buf, f, s); err != nil {
				t.Fatalf("%s/%s: %v", k, f, err)
			}
			if strings.Contains(buf.String(), "%!") {
				t.Errorf("%s/%s: format verb leaked:\n%s", k, f, buf.String())
			}
			if buf.Len() == 0 {
				t.Errorf("%s/%s: empty output", k, f)
			}
		}
	}
}
