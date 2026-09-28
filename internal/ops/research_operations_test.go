package ops

import (
	"context"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"net/http"
	"testing"
)

func TestResearchReadsUseSharedCatalogAndMonitorDependencies(t *testing.T) {
	cat := NewCatalog()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("non-read request %s", r.Method)
		}
		switch r.URL.Path {
		case "/api/v1/company-events/42/transcripts/paragraph-inferences":
			w.Write([]byte(`{"result":[]}`))
		case "/api/v2/company-events/42/report":
			w.Write([]byte(`{"result":{"eventId":42,"analysis":null,"watchPoint":null}}`))
		case "/api/v1/new-watchlists/groups":
			w.Write([]byte(`{"result":{"watchlists":[{"id":7,"name":"Example","type":"USER_MADE","itemCount":0,"items":[]}]}}`))
		case "/api/v1/new-watchlists/recommend/news":
			if r.URL.Query().Get("watchlistId") != "7" {
				t.Error("missing folder")
			}
			w.Write([]byte(`{"result":{"title":"News","newsList":[],"pageSize":20}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	for _, id := range []string{"earning_call_transcript", "earning_call_report", "watchlist_news"} {
		op, ok := cat.Get(id)
		if !ok || op.Write || op.Backend != "wts" || op.Probe == nil {
			t.Fatalf("operation %s missing read/probe contract", id)
		}
		if id == "watchlist_news" && (!op.Probe.WatchlistGroupScoped || len(op.ProbeRefs) == 0) {
			t.Fatal("folder dependencies missing")
		}
	}
	result, err := cat.Call(context.Background(), deps, "earning_call_transcript", map[string]any{"event_id": 42})
	if err != nil {
		t.Fatal(err)
	}
	if result.(domain.EarningTranscript).Available {
		t.Fatal("empty transcript fabricated")
	}
	result, err = cat.Call(context.Background(), deps, "earning_call_report", map[string]any{"event_id": 42})
	if err != nil {
		t.Fatal(err)
	}
	if result.(domain.EarningReport).Analysis != nil {
		t.Fatal("unavailable report fabricated")
	}
	result, err = cat.Call(context.Background(), deps, "watchlist_news", map[string]any{"folder_id": 7})
	if err != nil {
		t.Fatal(err)
	}
	if result.(domain.WatchlistNews).FolderID != 7 {
		t.Fatal("wrong folder")
	}
}
