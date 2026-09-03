package ops

import (
	"net/http"
	"testing"
)

func TestIndexDetailOperationMonitorsPriceAndSessionMetadata(t *testing.T) {
	t.Parallel()
	op, ok := NewCatalog().Get("index_detail")
	if !ok {
		t.Fatal("index_detail operation missing")
	}
	if op.Probe == nil || op.Probe.Name != "index-prices" || len(op.ExtraProbes) != 1 {
		t.Fatalf("index_detail probes = %#v / %#v", op.Probe, op.ExtraProbes)
	}
	if err := op.Probe.Check(http.StatusOK, []byte(`{"result":{"open":1,"high":2,"low":0,"close":1,"base":1}}`)); err != nil {
		t.Fatalf("verified index price schema rejected: %v", err)
	}
	if err := op.Probe.Check(http.StatusOK, []byte(`{"result":{"close":1}}`)); err == nil {
		t.Fatal("incomplete index price schema was accepted")
	}
	probe := op.ExtraProbes[0]
	if probe.Name != "index-info" || probe.Method != http.MethodGet || newRequestPath(probe.URL) != "/api/v2/index-infos/KGG01P" {
		t.Fatalf("index info probe = %#v", probe)
	}
	good := []byte(`{"result":{"code":"KGG01P","priceFeedType":{"code":"REAL_TIME","description":"실시간"},"tradingStartAt":"2026-09-03T09:00:00+09:00","tradingEndAt":"2026-09-03T15:20:00+09:00","isMarketOpen":false}}`)
	if err := probe.Check(http.StatusOK, good); err != nil {
		t.Fatalf("verified index info schema rejected: %v", err)
	}
	if err := probe.Check(http.StatusOK, []byte(`{"result":{"code":"KGG01P"}}`)); err == nil {
		t.Fatal("incomplete index session schema was accepted")
	}
}
