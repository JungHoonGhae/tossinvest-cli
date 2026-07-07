package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteConditionalOrdersTable(t *testing.T) {
	l := domain.ConditionalOrderList{
		Orders: []domain.ConditionalOrder{
			{ID: "co-1", Type: "SINGLE", Status: "WATCHING", Symbol: "005930", Quantity: 10, OrderType: "LIMIT", ExpireDate: "2026-12-31"},
		},
	}
	var buf bytes.Buffer
	if err := WriteConditionalOrders(&buf, FormatTable, l); err != nil {
		t.Fatalf("WriteConditionalOrders: %v", err)
	}
	if !strings.Contains(buf.String(), "co-1") {
		t.Errorf("missing id: %q", buf.String())
	}
}

func TestWriteConditionalOrderJSON(t *testing.T) {
	o := domain.ConditionalOrder{ID: "co-1", Type: "SINGLE", Symbol: "005930"}
	var buf bytes.Buffer
	if err := WriteConditionalOrder(&buf, FormatJSON, o); err != nil {
		t.Fatalf("WriteConditionalOrder JSON: %v", err)
	}
	if !strings.Contains(buf.String(), `"id": "co-1"`) {
		t.Errorf("json missing: %q", buf.String())
	}
}

func TestWriteConditionalOrderTableWithSecond(t *testing.T) {
	sec := &domain.ConditionalOrderCondition{Type: "PROFIT_RATE", TargetProfitRate: 5.5}
	o := domain.ConditionalOrder{
		ID: "co-2", Type: "OCO", Symbol: "005930", Quantity: 5, OrderType: "MARKET",
		First:  domain.ConditionalOrderCondition{Type: "STOP", TriggerPrice: 68000},
		Second: sec,
	}
	var buf bytes.Buffer
	if err := WriteConditionalOrder(&buf, FormatTable, o); err != nil {
		t.Fatalf("WriteConditionalOrder: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "STOP") || !strings.Contains(out, "PROFIT_RATE") {
		t.Errorf("table missing both legs: %q", out)
	}
}
