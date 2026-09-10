package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/history"
)

func TestMCPProjectsResultBeforeReturningIt(t *testing.T) {
	s := NewServer(nil, nil, Services{History: history.New(filepath.Join(t.TempDir(), "history.sqlite"), nil)}, "test", "dev")
	s.SetAuthStatus(AuthStatus{WTS: BackendStatus{Connected: true}})
	result, rpcErr := s.handleToolsCall(context.Background(), json.RawMessage(`{"name":"call_operation","arguments":{"operation":"auth_status","fields":["wts.connected"]}}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "official") || !strings.Contains(string(b), `connected`) {
		t.Fatalf("projected result: %s", b)
	}
	result, rpcErr = s.handleToolsCall(context.Background(), json.RawMessage(`{"name":"call_operation","arguments":{"operation":"history_list"}}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	b, _ = json.Marshal(result)
	if !strings.Contains(string(b), `"text":"[]"`) {
		t.Fatalf("local history: %s", b)
	}
	result, rpcErr = s.handleToolsCall(context.Background(), json.RawMessage(`{"name":"call_operation","arguments":{"operation":"history_sync","fields":["bad..path"]}}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	b, _ = json.Marshal(result)
	if !strings.Contains(string(b), "invalid field path") {
		t.Fatalf("invalid fields must fail before auth/dispatch: %s", b)
	}
}
