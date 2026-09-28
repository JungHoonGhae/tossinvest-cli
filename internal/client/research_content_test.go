package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEarningTranscriptPreservesTimesAndNullableEnrichment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/company-events/42/transcripts/paragraph-inferences" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		w.Write([]byte(`{"result":[{"eventId":42,"paragraphId":"p1","originalText":"Original","elapsedStartSeconds":1.25,"elapsedEndSeconds":3.5,"translated":{"id":"t1","translation":[{"index":0,"speaker":"Speaker","text":"번역"}]},"summarized":{"id":"s1","summarization":{"category":"intro","mappedCategory":"Introduction","text":"Summary","originalText":"Original summary"}}},{"eventId":42,"paragraphId":"p2","originalText":"Pending","elapsedStartSeconds":3.5,"elapsedEndSeconds":4,"translated":null,"summarized":null}]}`))
	}))
	defer server.Close()
	got, err := testClientFor(server).GetEarningTranscript(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || len(got.Paragraphs) != 2 {
		t.Fatalf("result %#v", got)
	}
	p := got.Paragraphs[0]
	if p.StartSeconds != 1.25 || p.EndSeconds != 3.5 || p.Translation.Lines[0].Text != "번역" || p.Summary.MappedCategory != "Introduction" {
		t.Fatalf("paragraph %#v", p)
	}
	if got.Paragraphs[1].Translation != nil || got.Paragraphs[1].Summary != nil {
		t.Fatal("pending enrichment was fabricated")
	}
}
func TestEarningTranscriptRejectsDriftButAcceptsEmpty(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{"result":[]}`, true}, {`{}`, false}, {`{"result":null}`, false}, {`{"result":{}}`, false},
		{`{"result":[{"eventId":43,"paragraphId":"p","elapsedStartSeconds":0,"elapsedEndSeconds":1}]}`, false},
		{`{"result":[{"eventId":42,"paragraphId":"p","elapsedStartSeconds":2,"elapsedEndSeconds":1}]}`, false},
		{`{"result":[{"eventId":42,"paragraphId":"p"}]}`, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(tc.body)) }))
			defer server.Close()
			got, err := testClientFor(server).GetEarningTranscript(context.Background(), 42)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			if tc.ok && (got.Available || got.Paragraphs == nil) {
				t.Fatal("empty state not preserved")
			}
		})
	}
}
func TestEarningReportPreservesIndependentPublicationStates(t *testing.T) {
	for _, analysis := range []string{`null`, `{"reportId":7,"overall":{"id":1,"title":"Overview","content":["Text"]},"pros":[],"cons":[]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/api/v2/company-events/42/report" {
				t.Errorf("request %s %s", r.Method, r.URL)
			}
			w.Write([]byte(`{"result":{"eventId":42,"analysis":` + analysis + `,"watchPoint":{"points":[{"title":"Watch","bullets":["Point"]}],"sources":[{"type":"NEWS","sourceId":"n1","title":"Source","provider":{"name":"Agency","subTitle":null},"createdAt":"today"}],"createdAt":"yesterday","updatedAt":"today"}}}`))
		}))
		got, err := testClientFor(server).GetEarningReport(context.Background(), 42)
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		if (got.Analysis == nil) != (analysis == "null") {
			t.Fatalf("analysis %#v", got.Analysis)
		}
		if got.WatchPoints.Sources[0].ID != "n1" || got.WatchPoints.Sources[0].Provider != "Agency" || got.WatchPoints.Sources[0].Subtitle != nil {
			t.Fatalf("sources %#v", got.WatchPoints.Sources)
		}
	}
	for _, body := range []string{`{}`, `{"result":null}`, `{"result":{"eventId":41,"analysis":null,"watchPoint":null}}`, `{"result":{"eventId":42,"watchPoint":null}}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) }))
		_, err := testClientFor(server).GetEarningReport(context.Background(), 42)
		server.Close()
		if err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
func TestWatchlistNewsBindsExistingFolderAndRejectsFallback(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(map[bool]string{true: "exists", false: "missing"}[exists], func(t *testing.T) {
			newsCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("method %s", r.Method)
				}
				if strings.HasSuffix(r.URL.Path, "/groups") {
					if r.URL.Query().Get("ids") != "7" {
						t.Error("wrong folder check")
					}
					if exists {
						w.Write([]byte(`{"result":{"watchlists":[{"id":7,"name":"Example","type":"USER_MADE","itemCount":0,"items":[]}]}}`))
					} else {
						w.Write([]byte(`{"result":{"watchlists":[]}}`))
					}
					return
				}
				newsCalls++
				if r.URL.Path != "/api/v1/new-watchlists/recommend/news" || r.URL.Query().Get("watchlistId") != "7" {
					t.Errorf("news query %s", r.URL)
				}
				w.Write([]byte(`{"result":{"title":"News","description":"For folder","pageSize":20,"newsList":[{"newsId":"n1","title":"Headline","agencyName":"Agency","newsType":"NEWS","createdAt":"today","relatedStockDtos":[{"name":"Example","fluctuation":0}]}]}}`))
			}))
			defer server.Close()
			got, err := testClientFor(server).GetWatchlistNews(context.Background(), 7)
			if exists {
				if err != nil {
					t.Fatal(err)
				}
				if got.FolderID != 7 || len(got.News) != 1 || got.News[0].RelatedStocks[0].Fluctuation != 0 {
					t.Fatalf("news %#v", got)
				}
			} else if err == nil || newsCalls != 0 {
				t.Fatal("missing folder fell back to unscoped news")
			}
		})
	}
}
func TestIndexReasoningUsesCertAndDoesNotFabricateTerms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/reasoning/indices/KGG01P/detail" || r.Method != "GET" || r.URL.RawQuery != "" {
			t.Errorf("request %s %s", r.Method, r.URL)
		}
		w.Write([]byte(`{"result":{"signalId":"s","reasoning":{"issue":{"assetCode":"KGG01P","assetName":"Index","assetType":"INDEX","description":{"data":["Evidence"]}},"news":{"data":[{"id":"n","title":"Source"}]}},"relatedReasoning":null}}`))
	}))
	defer server.Close()
	got, err := testClientFor(server).GetAISignalDetail(context.Background(), "KGG01P", "index")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || got.Terms != nil || got.ProductType != "INDEX" || len(got.News) != 1 {
		t.Fatalf("detail %#v", got)
	}
	data, _ := json.Marshal(got)
	if !strings.Contains(string(data), `"terms":null`) {
		t.Fatal("missing terms must remain unknown")
	}
}
