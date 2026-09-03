package ops

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMutationInventoryListsEveryCallableWrite(t *testing.T) {
	t.Parallel()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	data, err := os.ReadFile(filepath.Join(repo, "docs", "reverse-engineering", "mutation-inventory.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	for _, operation := range NewCatalog().List("", 0) {
		if operation.Write && !strings.Contains(doc, "`"+operation.ID+"`") {
			t.Errorf("callable write %q is missing from mutation inventory", operation.ID)
		}
	}
}

func TestOfficialFinancialMutationsDeclareUnknownTransportOutcome(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	for _, id := range []string{
		"place_order", "cancel_order", "modify_order",
		"place_conditional_order", "cancel_conditional_order", "modify_conditional_order",
	} {
		op, ok := catalog.Get(id)
		if !ok || op.Mutation == nil {
			t.Fatalf("%s mutation metadata missing", id)
		}
		verification := strings.ToLower(op.Mutation.Verification)
		for _, required := range []string{"official response only", "transport error", "inspect", "before retrying"} {
			if !strings.Contains(verification, required) {
				t.Errorf("%s verification %q is missing %q", id, verification, required)
			}
		}
	}
}
