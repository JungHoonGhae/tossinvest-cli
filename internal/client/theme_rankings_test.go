package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetThemeRankings(t *testing.T) {
	const body = `{"result":{"type":"tics","name":"tics_fluctuation_v2","dateTime":"2026-06-29T12:30:35","data":[
		{"productCode":null,"ticsId":"685","title":"배터리제조","value":"+11.1%","preciseValue":"+11.18%","ranking":1,"totalCount":21,"riseCompanyCount":13,"priceValue":"21개 중 13개 종목 상승"},
		{"productCode":null,"ticsId":"100","title":"조선","value":"-3.2%","preciseValue":"-3.20%","ranking":2,"totalCount":10,"riseCompanyCount":1,"priceValue":"10개 중 1개 종목 상승"},
		{"ticsId":"7","title":"보험","value":"+0.5%","preciseValue":"+0.50%","ranking":3,"totalCount":5,"riseCompanyCount":3}
	]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/v1/tics/rankings") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	r, err := testClientFor(srv).GetThemeRankings(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetThemeRankings: %v", err)
	}
	if r.Name != "tics_fluctuation_v2" {
		t.Errorf("name = %q", r.Name)
	}
	if len(r.Items) != 3 {
		t.Fatalf("want 3 items, got %d", len(r.Items))
	}
	g := r.Items[0]
	if g.Title != "배터리제조" || g.ChangeRate != 11.18 || g.RiseCompanyCount != 13 || g.TotalCount != 21 || g.Ranking != 1 || g.TicsID != "685" {
		t.Errorf("item0 mismatch: %+v", g)
	}
	if g.Summary != "21개 중 13개 종목 상승" {
		t.Errorf("summary = %q", g.Summary)
	}
	if r.Items[1].ChangeRate != -3.2 {
		t.Errorf("negative rate parse: got %v", r.Items[1].ChangeRate)
	}
}

func TestGetThemeRankingsSizeLimit(t *testing.T) {
	const body = `{"result":{"data":[
		{"ranking":1,"title":"a","preciseValue":"+1%"},
		{"ranking":2,"title":"b","preciseValue":"+2%"},
		{"ranking":3,"title":"c","preciseValue":"+3%"}
	]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	r, err := testClientFor(srv).GetThemeRankings(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 2 {
		t.Fatalf("size limit: want 2, got %d", len(r.Items))
	}
	if r.Items[0].Title != "a" || r.Items[1].Title != "b" {
		t.Errorf("expected top-2 by order, got %+v", r.Items)
	}
}

func TestParsePercent(t *testing.T) {
	cases := map[string]float64{
		"+11.18%":   11.18,
		"-3.20%":    -3.2,
		"0.0%":      0,
		"+1,234.5%": 1234.5,
		"":          0,
		"  +5% ":    5,
	}
	for in, want := range cases {
		if got := parsePercent(in); got != want {
			t.Errorf("parsePercent(%q) = %v, want %v", in, got, want)
		}
	}
}
