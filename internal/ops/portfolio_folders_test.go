package ops

import (
	"net/http"
	"testing"
)

func TestPortfolioFoldersOperationDeclaresVerifiedAccountScopedContract(t *testing.T) {
	t.Parallel()
	op, ok := NewCatalog().Get("portfolio_folders")
	if !ok {
		t.Fatal("portfolio_folders operation missing")
	}
	if op.Write || op.Backend != "wts" || op.Domain != "securities" || op.Category != "portfolio" {
		t.Fatalf("operation metadata = %#v", op)
	}
	if op.Probe == nil || op.Probe.Name != "portfolio-folders" || op.Probe.Method != http.MethodPost || !op.Probe.AccountScoped {
		t.Fatalf("probe = %#v", op.Probe)
	}
	if got := newRequestPath(op.Probe.URL); got != "/api/v2/dashboard/asset/sections/all" {
		t.Fatalf("probe path = %q", got)
	}
	if op.Probe.Body != `{"types":["FOLDER_OVERVIEW_V2"]}` {
		t.Fatalf("probe body = %s", op.Probe.Body)
	}
	good := []byte(`{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[],"hiddenStock":{"count":0,"all":false,"amount":0},"evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0}}}]}}`)
	if err := op.Probe.Check(http.StatusOK, good); err != nil {
		t.Fatalf("verified folder schema rejected: %v", err)
	}
	bad := []byte(`{"result":{"sections":[{"type":"SORTED_OVERVIEW","data":{"folders":[]}}]}}`)
	if err := op.Probe.Check(http.StatusOK, bad); err == nil {
		t.Fatal("wrong section contract was accepted")
	}
	incompleteFolder := []byte(`{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[{"folderName":"x"}],"hiddenStock":{"count":0},"evaluatedAmountAfterFees":{},"profitLossAmountAfterFees":{}}}]}}`)
	if err := op.Probe.Check(http.StatusOK, incompleteFolder); err == nil {
		t.Fatal("folder without items array was accepted")
	}
	silentZero := []byte(`{"result":{"sections":[{"type":"FOLDER_OVERVIEW_V2","data":{"folders":[{"folderName":"x","folderType":"DEFAULT","items":[],"evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0}}],"hiddenStock":{"count":0,"all":false},"evaluatedAmountAfterFees":{"krw":0,"usd":0},"profitLossAmountAfterFees":{"krw":0,"usd":0}}}]}}`)
	if err := op.Probe.Check(http.StatusOK, silentZero); err == nil {
		t.Fatal("hidden amount omission that would render as zero was accepted")
	}
	if len(op.ProbeRefs) != 1 || op.ProbeRefs[0] != "account-list" {
		t.Fatalf("probe refs = %#v", op.ProbeRefs)
	}
}
