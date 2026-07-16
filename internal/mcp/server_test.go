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

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

// runServer feeds request lines through a server whose trading is fully
// disabled (the default config posture).
func runServer(t *testing.T, client *official.Client, lines ...string) []map[string]any {
	t.Helper()
	return runServerPolicy(t, client, config.Trading{}, lines...)
}

// runServerPolicy is like runServer but with an explicit trading policy, used to
// exercise the write gate.
func runServerPolicy(t *testing.T, client *official.Client, policy config.Trading, lines ...string) []map[string]any {
	t.Helper()
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	tradingSvc := trading.NewService(policy, OfficialBroker{Client: client})
	s := NewServer(client, nil, tradingSvc, "test", "0.0.0")
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

func TestListOperationsIncludesGatedWrites(t *testing.T) {
	c := dummyClient(t, nil)
	resps := runServer(t, c,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_operations","arguments":{"query":"place_order"}}}`,
	)
	text, _ := toolText(t, resps[0])
	var payload struct {
		Operations []struct {
			ID    string `json:"id"`
			Write bool   `json:"write"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Operations) != 1 || !payload.Operations[0].Write {
		t.Fatalf("place_order should be listed as a write op: %+v", payload.Operations)
	}
}

func TestPlaceOrderPreviewDoesNotExecute(t *testing.T) {
	// No routes: a live POST would 404. Preview must not hit the network.
	c := dummyClient(t, nil)
	resps := runServer(t, c,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"place_order","params":{"symbol":"AAPL","side":"buy","market":"us","quantity":1,"price":100,"currency_mode":"USD"}}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if isErr {
		t.Fatalf("preview should not error: %s", text)
	}
	var preview struct {
		Kind          string `json:"kind"`
		ConfirmToken  string `json:"confirm_token"`
		MutationReady bool   `json:"mutation_ready"`
	}
	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Kind != "place" || preview.ConfirmToken == "" {
		t.Fatalf("unexpected preview: %s", text)
	}
	if preview.MutationReady {
		t.Errorf("mutation_ready must be false when config disables trading")
	}
}

func TestPlaceOrderExecuteBlockedWhenDisabled(t *testing.T) {
	c := dummyClient(t, nil) // disabled policy (default)
	resps := runServer(t, c,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"place_order","params":{"symbol":"AAPL","side":"buy","market":"us","quantity":1,"price":100,"currency_mode":"USD","execute":true,"confirm":"whatever"}}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if !isErr {
		t.Fatalf("execute with trading disabled must be an error, got: %s", text)
	}
}

func TestPlaceOrderExecuteSubmitsWhenEnabled(t *testing.T) {
	c := dummyClient(t, map[string]string{
		"/api/v1/accounts": `{"result":[{"accountNo":"123-45","accountSeq":7,"accountType":"BROKERAGE"}]}`,
		"/api/v1/orders":   `{"result":{"orderId":"ORD-777"}}`,
	})
	policy := config.Trading{Place: true, AllowLiveOrderActions: true}

	// Step 1: preview to obtain the confirm token.
	preResp := runServerPolicy(t, c, policy,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"place_order","params":{"symbol":"AAPL","side":"buy","market":"us","quantity":1,"price":100,"currency_mode":"USD"}}}}`,
	)
	preText, _ := toolText(t, preResp[0])
	var preview struct {
		ConfirmToken  string `json:"confirm_token"`
		MutationReady bool   `json:"mutation_ready"`
	}
	if err := json.Unmarshal([]byte(preText), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.MutationReady {
		t.Fatalf("mutation_ready should be true with enabling policy: %s", preText)
	}

	// Step 2: execute with the token.
	line := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"place_order","params":{"symbol":"AAPL","side":"buy","market":"us","quantity":1,"price":100,"currency_mode":"USD","execute":true,"confirm":"` + preview.ConfirmToken + `"}}}}`
	execResp := runServerPolicy(t, c, policy, line)
	execText, isErr := toolText(t, execResp[0])
	if isErr {
		t.Fatalf("execute with valid token should succeed: %s", execText)
	}
	if !strings.Contains(execText, "ORD-777") {
		t.Errorf("expected order id in result: %s", execText)
	}
}

func TestPlaceOrderExecuteWrongTokenFails(t *testing.T) {
	c := dummyClient(t, map[string]string{
		"/api/v1/accounts": `{"result":[{"accountNo":"123-45","accountSeq":7,"accountType":"BROKERAGE"}]}`,
		"/api/v1/orders":   `{"result":{"orderId":"ORD-777"}}`,
	})
	policy := config.Trading{Place: true, AllowLiveOrderActions: true}
	resps := runServerPolicy(t, c, policy,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"place_order","params":{"symbol":"AAPL","side":"buy","market":"us","quantity":1,"price":100,"currency_mode":"USD","execute":true,"confirm":"bad-token"}}}}`,
	)
	text, isErr := toolText(t, resps[0])
	if !isErr {
		t.Fatalf("execute with wrong confirm token must fail, got: %s", text)
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

// TestToolResultCapsOversizedPayload verifies an oversized read (a synthetic
// dummy news briefing, well past maxResultBytes) comes back under the cap, still
// as valid JSON, and says out loud that items were dropped — a silent truncation
// would let the model read a partial list as complete.
func TestToolResultCapsOversizedPayload(t *testing.T) {
	items := make([]map[string]any, 0, 40)
	for i := 0; i < 40; i++ {
		news := make([]map[string]any, 0, 40)
		for j := 0; j < 40; j++ {
			news = append(news, map[string]any{
				"title":      strings.Repeat("더미 헤드라인 ", 6),
				"agencyName": "더미통신",
				"source":     "https://example.invalid/dummy",
				"createdAt":  "2026-01-01T00:00:00",
			})
		}
		items = append(items, map[string]any{
			"category": map[string]any{"type": "DUMMY_THEME", "keywords": []string{"더미", "합성"}},
			"news":     news,
		})
	}
	body, err := json.Marshal(map[string]any{"result": map[string]any{"createdAt": "2026-01-01T00:00:00", "items": items}})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	wts := tossclient.New(tossclient.Config{
		InfoBaseURL: srv.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})
	resps := driveWTS(t, wts,
		initLine,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"news_briefing"}}}`,
	)
	res, _ := resps[1]["result"].(map[string]any)
	if isErr, _ := res["isError"].(bool); isErr {
		t.Fatalf("unexpected tool error: %v", res)
	}
	content, _ := res["content"].([]any)
	text, _ := content[0].(map[string]any)["text"].(string)

	if len(text) > maxResultBytes {
		t.Errorf("result is %d bytes, want <= %d", len(text), maxResultBytes)
	}
	var decoded any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("capped result is not valid JSON: %v", err)
	}
	if !strings.Contains(text, omittedKey) || !strings.Contains(text, "items omitted") {
		t.Errorf("capped result must state that items were dropped, got %q", text[:200])
	}
}

// TestToolResultKeepsSmallPayload verifies a payload under the cap is passed
// through byte-for-byte (no placeholder, no reordering).
func TestToolResultKeepsSmallPayload(t *testing.T) {
	payload := map[string]any{"items": []any{map[string]any{"symbol": "005930"}, map[string]any{"symbol": "000660"}}}
	want, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	res, rerr := toolResult(payload, false)
	if rerr != nil {
		t.Fatalf("toolResult: %v", rerr)
	}
	got := res.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
	if got != string(want) {
		t.Errorf("small payload was modified:\n got %s\nwant %s", got, want)
	}
}
