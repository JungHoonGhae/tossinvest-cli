package ops

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestAccountAccessStatusOperationIsCallableAndOwnsDependencyProbes(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/user/last-login-info":
			_, _ = w.Write([]byte(`{"result":{"channel":"W","osName":"macOS","agentName":"Chrome","timestamp":"2026-09-03T12:34:56+09:00"}}`))
		case "/api/v1/margin/cert/frozen-account":
			if got := r.Header.Get("accountKey"); got != "selected-test" {
				t.Fatalf("frozen accountKey = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":{"isFrozen":true,"startDate":"2026-09-01","endDate":"2026-09-30"}}`))
		case "/api/v2/account/unlock/accident-account/count":
			if got := r.Header.Get("accountKey"); got != "selected-test" {
				t.Fatalf("accident count accountKey = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":2}`))
		default:
			http.NotFound(w, r)
		}
	}))
	catalog := NewCatalog()
	op, ok := catalog.Get("account_access_status")
	if !ok {
		t.Fatal("account_access_status operation missing")
	}
	if op.Backend != "wts" || op.Domain != "securities" || op.Write || op.Probe == nil || len(op.ExtraProbes) != 2 || !slices.Contains(op.ProbeRefs, "account-list") {
		t.Fatalf("operation metadata = %#v", op)
	}
	gotAny, err := catalog.Call(context.Background(), deps, op.ID, map[string]any{"account": "selected-test"})
	if err != nil {
		t.Fatal(err)
	}
	got := gotAny.(domain.AccountAccessStatus)
	if got.AccountScope == "" || got.AccountScope == "selected-test" || got.LastLogin.AgentName != "Chrome" || !got.Margin.Frozen || got.AccidentAccountCount != 2 {
		t.Fatalf("result = %#v", got)
	}

	probes := append([]ProbeSpec{*op.Probe}, op.ExtraProbes...)
	want := map[string][]byte{
		"account-last-login":     []byte(`{"result":{"channel":"W","osName":"macOS","agentName":"Chrome","timestamp":"2026-09-03T12:34:56+09:00"}}`),
		"account-margin-frozen":  []byte(`{"result":{"isFrozen":false,"startDate":null,"endDate":null}}`),
		"account-accident-count": []byte(`{"result":0}`),
	}
	for _, probe := range probes {
		body, found := want[probe.Name]
		if !found {
			t.Errorf("unexpected probe %q", probe.Name)
			continue
		}
		delete(want, probe.Name)
		if probe.Name != "account-last-login" && !probe.AccountScoped {
			t.Errorf("%s must inject accountKey", probe.Name)
		}
		if err := probe.Check(http.StatusOK, body); err != nil {
			t.Errorf("%s rejected verified schema: %v", probe.Name, err)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing probes: %v", want)
	}
}
