package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
)

// runServer feeds newline-delimited JSON-RPC request lines through Serve and
// returns the decoded responses (one per line the server emitted).
func runServer(t *testing.T, client *official.Client, lines ...string) []map[string]any {
	t.Helper()
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	s := NewServer(client, "test", "0.0.0")
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
			t.Fatalf("decoding response %q: %v", raw, err)
		}
		resps = append(resps, m)
	}
	return resps
}

// dummyClient builds an official.Client pointed at an httptest server that
// serves the token endpoint plus any provided path→body responses (dummy data).
func dummyClient(t *testing.T, routes map[string]string) *official.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
			return
		}
		if body, ok := routes[r.URL.Path]; ok {
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return official.New(
		official.Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
	)
}

func resultOf(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	res, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("response has no result object: %v", resp)
	}
	return res
}

// toolText extracts the text of the first content block from a tools/call result.
func toolText(t *testing.T, resp map[string]any) (string, bool) {
	t.Helper()
	res := resultOf(t, resp)
	isErr, _ := res["isError"].(bool)
	content, ok := res["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("result has no content: %v", res)
	}
	block := content[0].(map[string]any)
	return block["text"].(string), isErr
}

func TestInitializeEchoesProtocolVersion(t *testing.T) {
	c := dummyClient(t, nil)
	resps := runServer(t, c,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
	)
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d", len(resps))
	}
	res := resultOf(t, resps[0])
	if res["protocolVersion"] != "2025-03-26" {
		t.Errorf("protocolVersion = %v, want echoed 2025-03-26", res["protocolVersion"])
	}
	if _, ok := res["capabilities"].(map[string]any)["tools"]; !ok {
		t.Errorf("capabilities missing tools: %v", res["capabilities"])
	}
}

func TestNotificationProducesNoResponse(t *testing.T) {
	c := dummyClient(t, nil)
	resps := runServer(t, c,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
	)
	// Only the ping (which has an id) should get a response.
	if len(resps) != 1 {
		t.Fatalf("want 1 response (ping only), got %d: %v", len(resps), resps)
	}
	if resps[0]["id"].(float64) != 2 {
		t.Errorf("response id = %v, want 2", resps[0]["id"])
	}
}

func TestToolsListExposesThreeCatalogTools(t *testing.T) {
	c := dummyClient(t, nil)
	resps := runServer(t, c, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	tools := resultOf(t, resps[0])["tools"].([]any)
	got := map[string]bool{}
	for _, tl := range tools {
		got[tl.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"list_operations", "describe_operation", "call_operation"} {
		if !got[want] {
			t.Errorf("tools/list missing %q; got %v", want, got)
		}
	}
	if len(tools) != 3 {
		t.Errorf("want exactly 3 always-on tools, got %d", len(tools))
	}
}

func TestListOperationsFiltersByQuery(t *testing.T) {
	c := dummyClient(t, nil)
	resps := runServer(t, c,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_operations","arguments":{"query":"orderbook"}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if isErr {
		t.Fatalf("unexpected error result: %s", text)
	}
	var payload struct {
		Count      int `json:"count"`
		Operations []struct {
			ID string `json:"id"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Count != 1 || payload.Operations[0].ID != "orderbook" {
		t.Fatalf("query orderbook: got %+v", payload)
	}
}

func TestDescribeUnknownOperationIsToolError(t *testing.T) {
	c := dummyClient(t, nil)
	resps := runServer(t, c,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"describe_operation","arguments":{"operation":"nope"}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if !isErr {
		t.Fatalf("want isError for unknown operation, got: %s", text)
	}
	if !strings.Contains(text, "unknown operation") {
		t.Errorf("error text = %q", text)
	}
}

func TestCallOperationAccountsDispatches(t *testing.T) {
	c := dummyClient(t, map[string]string{
		"/api/v1/accounts": `{"result":[{"accountNo":"123-45","accountSeq":7,"accountType":"BROKERAGE"}]}`,
	})
	resps := runServer(t, c,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"accounts"}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "123-45") {
		t.Errorf("account payload missing dummy data: %s", text)
	}
}

func TestCallOperationMissingRequiredParam(t *testing.T) {
	c := dummyClient(t, nil)
	resps := runServer(t, c,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"buying_power"}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if !isErr {
		t.Fatalf("want isError for missing currency, got: %s", text)
	}
	if !strings.Contains(text, "currency") {
		t.Errorf("error should name the missing param: %q", text)
	}
}

func TestUnknownMethodReturnsRPCError(t *testing.T) {
	c := dummyClient(t, nil)
	resps := runServer(t, c, `{"jsonrpc":"2.0","id":9,"method":"does/not/exist"}`)
	if _, ok := resps[0]["error"]; !ok {
		t.Fatalf("want JSON-RPC error, got: %v", resps[0])
	}
	code := resps[0]["error"].(map[string]any)["code"].(float64)
	if int(code) != codeMethodNotFound {
		t.Errorf("code = %v, want %d", code, codeMethodNotFound)
	}
}
