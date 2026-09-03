package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListPortfolioFoldersUsesVerifiedGroupedHoldingsContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/dashboard/asset/sections/all" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("accountKey"); got != "selected-account" {
			t.Fatalf("accountKey = %q", got)
		}
		var body struct {
			Types []string `json:"types"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Types) != 1 || body.Types[0] != "FOLDER_OVERVIEW_V2" {
			t.Fatalf("types = %#v", body.Types)
		}
		_, _ = w.Write([]byte(`{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{` +
			`"principalAmount":{"krw":1000,"usd":1},"evaluatedAmount":{"krw":1200,"usd":1.2},` +
			`"evaluatedAmountAfterFees":{"krw":1190,"usd":1.19},"profitLossAmount":{"krw":200,"usd":0.2},` +
			`"profitLossAmountAfterFees":{"krw":190,"usd":0.19},"dailyProfitLossAmount":{"krw":50,"usd":0.05},` +
			`"profitLossRate":{"krw":0.2,"usd":0.2},"profitLossRateAfterFees":{"krw":0.19,"usd":0.19},` +
			`"dailyProfitLossRate":{"krw":0.05,"usd":0.05},"folders":[{` +
			`"folderKey":"private-folder-key","folderName":"Long term","folderType":"DEFAULT","detailType":null,"isDefault":true,` +
			`"principalAmount":{"krw":1000,"usd":1},"evaluatedAmount":{"krw":1200,"usd":1.2},` +
			`"evaluatedAmountAfterFees":{"krw":1190,"usd":1.19},"profitLossAmount":{"krw":200,"usd":0.2},` +
			`"profitLossAmountAfterFees":{"krw":190,"usd":0.19},"dailyProfitLossAmount":{"krw":50,"usd":0.05},` +
			`"profitLossRate":{"krw":0.2,"usd":0.2},"profitLossRateAfterFees":{"krw":0.19,"usd":0.19},` +
			`"dailyProfitLossRate":{"krw":0.05,"usd":0.05},"items":[{` +
			`"key":"private-item-key","stockCode":"US.TEST","stockIsin":"US0000000001","stockSymbol":"TEST","stockName":"Test Corp",` +
			`"quantity":2,"tradableQuantity":1,"unsettledQuantity":1,"currentPrice":{"krw":600,"usd":0.6},` +
			`"basePrice":{"krw":575,"usd":0.575},"purchasePrice":{"krw":500,"usd":0.5},"purchaseAmount":{"krw":1000,"usd":1},` +
			`"evaluatedAmount":{"krw":1200,"usd":1.2},"evaluatedAmountAfterFees":{"krw":1190,"usd":1.19},` +
			`"profitLossAmount":{"krw":200,"usd":0.2},"profitLossAmountAfterFees":{"krw":190,"usd":0.19},` +
			`"dailyProfitLossAmount":{"krw":50,"usd":0.05},"profitLossRate":{"krw":0.2,"usd":0.2},` +
			`"profitLossRateAfterFees":{"krw":0.19,"usd":0.19},"dailyProfitLossRate":{"krw":0.05,"usd":0.05},` +
			`"commission":{"krw":10,"usd":0.01},"commissionRate":0.01,"buyCommission":{"krw":4,"usd":0.004},` +
			`"sellCommission":{"krw":6,"usd":0.006},"marketCode":"NASDAQ","marketDivision":"us","type":"STOCK",` +
			`"shareHoldingsType":"us","delisting":false,"unlisting":false,"archiving":false,"errorPricing":false,"nxtSupported":false` +
			`}]}],"hiddenStock":{"count":2,"all":false,"amount":300},"usePolling":false,"pricingErrorMessage":null}}]}}`))
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).ListPortfolioFolders(context.Background(), " selected-account ")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountScope == "" || got.AccountScope == "selected-account" || got.SectionType != "FOLDER_OVERVIEW_V2" {
		t.Fatalf("unsafe scope or section = %#v", got)
	}
	if got.EvaluatedAmountAfterFees.KRW != 1190 || got.ProfitLossRateAfterFees.USD != 0.19 || got.Hidden.Count != 2 {
		t.Fatalf("overview = %#v", got)
	}
	if len(got.Folders) != 1 || got.Folders[0].Name != "Long term" || !got.Folders[0].Default || len(got.Folders[0].Items) != 1 {
		t.Fatalf("folders = %#v", got.Folders)
	}
	if got.Folders[0].Key != "private-folder-key" || got.Folders[0].Items[0].Key != "private-item-key" {
		t.Fatalf("internal mutation keys were not retained: %#v", got.Folders[0])
	}
	item := got.Folders[0].Items[0]
	if item.Symbol != "TEST" || item.TradableQuantity != 1 || item.EvaluatedAmountAfterFees.KRW != 1190 || item.Commission.USD != 0.01 {
		t.Fatalf("item = %#v", item)
	}
}

func TestListPortfolioFoldersResolvesPrimaryAccountWhenOmitted(t *testing.T) {
	t.Parallel()
	var folderAccount string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/account/list":
			_, _ = w.Write([]byte(`{"result":{"accountList":[{"key":"primary-account"}],"primaryKey":"primary-account"}}`))
		case "/api/v2/dashboard/asset/sections/all":
			folderAccount = r.Header.Get("accountKey")
			_, _ = w.Write([]byte(`{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[],"hiddenStock":{"count":0,"all":false,"amount":0},"evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0}}}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	got, err := testClientFor(server).ListPortfolioFolders(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if folderAccount != "primary-account" || got.AccountScope == "" || got.AccountScope == "primary-account" {
		t.Fatalf("account header=%q scope=%q", folderAccount, got.AccountScope)
	}
	if got.Folders == nil {
		t.Fatal("empty folder result must be an empty array, not nil")
	}
}

func TestListPortfolioFoldersFailsClosedOnInvalidResponses(t *testing.T) {
	t.Parallel()
	const validOverview = `"evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0},"hiddenStock":{"count":0,"all":false,"amount":0}`
	const validFolder = `"folderName":"x","folderType":"DEFAULT","evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0}`
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantErrSub string
	}{
		{name: "missing section", status: http.StatusOK, body: `{"result":{"sections":[]}}`, wantErrSub: "FOLDER_OVERVIEW_V2 section not found"},
		{name: "null section data", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":null}]}}`, wantErrSub: "data must be an object"},
		{name: "missing folders", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{` + validOverview + `}}]}}`, wantErrSub: "missing folders"},
		{name: "missing valuation", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[],"profitLossAmountAfterFees":{"krw":0,"usd":0},"hiddenStock":{"count":0,"all":false,"amount":0}}}]}}`, wantErrSub: "missing evaluatedAmountAfterFees"},
		{name: "missing valuation currency", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[],"evaluatedAmountAfterFees":{"krw":0},"profitLossAmountAfterFees":{"krw":0,"usd":0},"hiddenStock":{"count":0,"all":false,"amount":0}}}]}}`, wantErrSub: "evaluatedAmountAfterFees.usd"},
		{name: "missing hidden amount", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[],"evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0},"hiddenStock":{"count":0,"all":false}}}]}}`, wantErrSub: "hiddenStock.amount"},
		{name: "missing hidden all", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[],"evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0},"hiddenStock":{"count":0,"amount":0}}}]}}`, wantErrSub: "hiddenStock.all"},
		{name: "missing folder name", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[{"folderType":"DEFAULT","evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0},"items":[]}],` + validOverview + `}}]}}`, wantErrSub: "folders[0].folderName"},
		{name: "missing folder type", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[{"folderName":"x","evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0},"items":[]}],` + validOverview + `}}]}}`, wantErrSub: "folders[0].folderType"},
		{name: "missing folder after-fee amount", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[{"folderName":"x","folderType":"DEFAULT","profitLossAmountAfterFees":{"krw":0,"usd":0},"items":[]}],` + validOverview + `}}]}}`, wantErrSub: "folders[0].evaluatedAmountAfterFees"},
		{name: "missing folder items", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[{` + validFolder + `}],` + validOverview + `}}]}}`, wantErrSub: "folders[0].items"},
		{name: "null folder items", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[{` + validFolder + `,"items":null}],` + validOverview + `}}]}}`, wantErrSub: "folders[0].items"},
		{name: "missing item identity", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[{` + validFolder + `,"items":[{}]}],` + validOverview + `}}]}}`, wantErrSub: "missing stockCode"},
		{name: "missing item after-fee amount", status: http.StatusOK, body: `{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[{` + validFolder + `,"items":[{"stockCode":"A005930","profitLossAmountAfterFees":{"krw":0,"usd":0}}]}],` + validOverview + `}}]}}`, wantErrSub: "folders[0].items[0].evaluatedAmountAfterFees"},
		{name: "malformed JSON", status: http.StatusOK, body: `{"result":`, wantErrSub: "unexpected end"},
		{name: "upstream failure", status: http.StatusBadGateway, body: `{"error":"synthetic"}`, wantErrSub: "502"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)

			_, err := testClientFor(server).ListPortfolioFolders(context.Background(), "selected-account")
			if err == nil || !strings.Contains(err.Error(), tc.wantErrSub) {
				t.Fatalf("error=%v, want substring %q", err, tc.wantErrSub)
			}
		})
	}
}
