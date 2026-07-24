package ops

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
)

// These tests cover the "auto" backend: operations the hybrid router serves
// with either credential (official first, web-session fallback). They are the
// first tests to drive Catalog.Call's handler path — everything before this
// only asserted registry invariants.

// officialServer serves the oauth token plus one data path, and reports how
// many times the data path was hit.
func officialServer(t *testing.T, dataPath string, handler http.HandlerFunc) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case dataPath:
			hits++
			handler(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func officialClientFor(t *testing.T, srv *httptest.Server) *official.Client {
	t.Helper()
	return official.New(
		official.Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "token.json"),
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
	)
}

// TestAutoOperationServedByOfficialAlone is the regression guard for the
// official-only user: `orderbook` used to require official credentials
// outright, and routing it through the hybrid must not turn that into a
// "run tossctl auth login" refusal.
func TestAutoOperationServedByOfficialAlone(t *testing.T) {
	srv, hits := officialServer(t, "/api/v1/orderbook", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"symbol":"005930","bids":[],"asks":[]}}`))
	})
	off := officialClientFor(t, srv)

	// No web session: the router still needs a non-nil embedded client, exactly
	// as cmd/tossctl/mcp.go wires it.
	routed := hybrid.New(tossclient.New(tossclient.Config{}), off,
		hybrid.Policy{Prefer: "auto", Fallback: true}, &bytes.Buffer{})

	deps := &Deps{
		Client: off,
		WTS:    routed,
		Auth:   AuthStatus{Official: BackendStatus{Connected: true}},
	}

	_, err := NewCatalog().Call(context.Background(), deps, "orderbook", map[string]any{"symbol": "005930"})
	if err != nil {
		t.Fatalf("auto operation must work with official credentials alone; got %v", err)
	}
	if *hits != 1 {
		t.Fatalf("official data path: want 1 hit, got %d", *hits)
	}
}

// TestAutoOperationFallsBackToWTS proves the whole point of the change: when
// official fails in a fallback-eligible way, the router reaches for the web
// session instead of surfacing the official error. The stderr notice is the
// observable signal that the fallback fired.
func TestAutoOperationFallsBackToWTS(t *testing.T) {
	srv, _ := officialServer(t, "/api/v1/orderbook", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // ErrServer => fallback-eligible
	})
	off := officialClientFor(t, srv)

	var stderr bytes.Buffer
	routed := hybrid.New(tossclient.New(tossclient.Config{}), off,
		hybrid.Policy{Prefer: "auto", Fallback: true}, &stderr)

	deps := &Deps{
		Client: off,
		WTS:    routed,
		Auth: AuthStatus{
			Official: BackendStatus{Connected: true},
			WTS:      BackendStatus{Connected: true},
		},
	}

	// The WTS leg has no fixtures behind it, so the call itself is expected to
	// fail — what matters is that the router got there at all.
	_, _ = NewCatalog().Call(context.Background(), deps, "orderbook", map[string]any{"symbol": "005930"})

	if !strings.Contains(stderr.String(), "falling back") {
		t.Fatalf("official 500 must trigger the WTS fallback; stderr was %q", stderr.String())
	}
}

// TestAutoOperationGating covers Call's new "auto" branch.
func TestAutoOperationGating(t *testing.T) {
	catalog := NewCatalog()

	t.Run("no credentials names both logins", func(t *testing.T) {
		deps := &Deps{} // nothing connected
		_, err := catalog.Call(context.Background(), deps, "orderbook", map[string]any{"symbol": "005930"})
		if err == nil {
			t.Fatal("want an error when neither backend is connected")
		}
		for _, want := range []string{"tossctl openapi login", "tossctl auth login"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should mention %q; got %v", want, err)
			}
		}
	})

	t.Run("web session alone is enough", func(t *testing.T) {
		deps := &Deps{
			WTS:  hybrid.New(tossclient.New(tossclient.Config{}), nil, hybrid.Policy{Prefer: "auto"}, &bytes.Buffer{}),
			Auth: AuthStatus{WTS: BackendStatus{Connected: true}},
		}
		_, err := catalog.Call(context.Background(), deps, "orderbook", map[string]any{"symbol": "005930"})
		// The call fails (no WTS fixtures), but it must not be refused by the gate.
		if err != nil && strings.Contains(err.Error(), "needs either official") {
			t.Fatalf("web session alone must pass the auto gate; got %v", err)
		}
	})
}

// TestWTSGatingUsesAuthSnapshot guards the switch from a nil check to the auth
// snapshot: the router is now built even without a session, so nilness alone no
// longer means "no web session".
func TestWTSGatingUsesAuthSnapshot(t *testing.T) {
	deps := &Deps{
		// Router present but no session behind it.
		WTS:  hybrid.New(tossclient.New(tossclient.Config{}), nil, hybrid.Policy{Prefer: "auto"}, &bytes.Buffer{}),
		Auth: AuthStatus{}, // WTS.Connected == false
	}

	var wtsOp string
	for _, op := range NewCatalog().List("", 0) {
		if op.Backend == "wts" {
			wtsOp = op.ID
			break
		}
	}
	if wtsOp == "" {
		t.Fatal("expected at least one wts-backed operation in the registry")
	}

	_, err := NewCatalog().Call(context.Background(), deps, wtsOp, nil)
	if err == nil || !strings.Contains(err.Error(), "tossctl auth login") {
		t.Fatalf("wts operation without a session must ask for `tossctl auth login`; got %v", err)
	}
}
