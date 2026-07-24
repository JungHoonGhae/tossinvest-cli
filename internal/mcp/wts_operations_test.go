package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// driveWTS feeds request lines through a server built over the given WTS client
// (no official client) and returns the parsed JSON-RPC responses.
func driveWTS(t *testing.T, wts *tossclient.Client, lines ...string) []map[string]any {
	t.Helper()
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	// Mirror cmd/tossctl/mcp.go: the router is always built (with a sessionless
	// client when there is no session) and the auth snapshot — not nilness — is
	// what tells Catalog.Call whether a web session exists. With no official
	// client the router degrades to a pure WTS passthrough.
	routedWTS := wts
	if routedWTS == nil {
		routedWTS = tossclient.New(tossclient.Config{})
	}
	routed := hybrid.New(routedWTS, nil, hybrid.Policy{Prefer: "wts"}, io.Discard)
	s := NewServer(nil, routed, nil, "test", "0.0.0")
	s.SetAuthStatus(AuthStatus{WTS: BackendStatus{Connected: wts != nil}})
	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var resps []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if raw == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("unmarshal %q: %v", raw, err)
		}
		resps = append(resps, m)
	}
	return resps
}

const initLine = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`

// TestCallOperationWTSDispatches verifies a WTS-only operation (stock_ranking)
// dispatches to the web-session client and returns its decoded payload.
func TestCallOperationWTSDispatches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rankings/realtime/stock" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":{"data":[{"code":"A005930","symbol":"005930","name":"삼성전자","market":{"displayName":"KOSPI"}}]}}`))
	}))
	defer srv.Close()

	wts := tossclient.New(tossclient.Config{
		InfoBaseURL: srv.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})

	resps := driveWTS(t, wts,
		initLine,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"stock_ranking","params":{"size":1}}}}`,
	)
	if len(resps) != 2 {
		t.Fatalf("want 2 responses, got %d", len(resps))
	}
	res, _ := resps[1]["result"].(map[string]any)
	if res == nil {
		t.Fatalf("no result: %v", resps[1])
	}
	if isErr, _ := res["isError"].(bool); isErr {
		t.Fatalf("unexpected tool error: %v", res)
	}
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("empty content: %v", res)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "005930") {
		t.Errorf("ranking payload missing symbol: %q", text)
	}
}

// TestWTSOperationWithoutSessionErrors verifies a WTS op returns a clear
// "run tossctl auth login" error when no web session is present (WTS == nil).
func TestWTSOperationWithoutSessionErrors(t *testing.T) {
	resps := driveWTS(t, nil,
		initLine,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"stock_ranking","params":{}}}}`,
	)
	res, _ := resps[1]["result"].(map[string]any)
	if isErr, _ := res["isError"].(bool); !isErr {
		t.Fatalf("want tool error for missing session, got %v", res)
	}
	content, _ := res["content"].([]any)
	text, _ := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "auth login") {
		t.Errorf("error should mention `tossctl auth login`, got %q", text)
	}
}

// TestCompletedOrdersRejectsBadDate verifies the date-parse branch returns a
// clear error (before any network call). A dummy WTS client is enough because
// parsing fails first.
func TestCompletedOrdersRejectsBadDate(t *testing.T) {
	wts := tossclient.New(tossclient.Config{Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
	resps := driveWTS(t, wts,
		initLine,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"completed_orders","params":{"from":"2024/01/01"}}}}`,
	)
	res, _ := resps[1]["result"].(map[string]any)
	if isErr, _ := res["isError"].(bool); !isErr {
		t.Fatalf("want tool error for bad date, got %v", res)
	}
	content, _ := res["content"].([]any)
	text, _ := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "YYYY-MM-DD") {
		t.Errorf("error should mention date format, got %q", text)
	}
}

// TestListOperationsIncludesWTS verifies the WTS reads are discoverable via
// list_operations (they carry backend "wts").
func TestListOperationsIncludesWTS(t *testing.T) {
	resps := driveWTS(t, nil,
		initLine,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_operations","arguments":{"query":"ai_signals"}}}`,
	)
	res, _ := resps[1]["result"].(map[string]any)
	content, _ := res["content"].([]any)
	text, _ := content[0].(map[string]any)["text"].(string)
	var payload struct {
		Operations []struct {
			ID      string `json:"id"`
			Backend string `json:"backend"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(payload.Operations) != 1 || payload.Operations[0].ID != "ai_signals" || payload.Operations[0].Backend != "wts" {
		t.Fatalf("want ai_signals (backend wts), got %+v", payload.Operations)
	}
}

// TestAuthStatusNoAuthRequired verifies auth_status is callable with no
// backends connected (Backend "none" bypasses the auth gate) and reports
// disconnected.
func TestAuthStatusNoAuthRequired(t *testing.T) {
	resps := driveWTS(t, nil,
		initLine,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"auth_status","params":{}}}}`,
	)
	res, _ := resps[1]["result"].(map[string]any)
	if isErr, _ := res["isError"].(bool); isErr {
		t.Fatalf("auth_status should not error without auth, got %v", res)
	}
	content, _ := res["content"].([]any)
	text, _ := content[0].(map[string]any)["text"].(string)
	var got AuthStatus
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("unmarshal auth_status: %v", err)
	}
	if got.WTS.Connected || got.Official.Connected {
		t.Errorf("want both disconnected with no auth, got %+v", got)
	}
}
