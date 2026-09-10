package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/history"
)

func TestHistoryOfflineAndJSONFlagsThroughRoot(t *testing.T) {
	dir := t.TempDir()
	// Offline history must not depend even on a parseable session/config file.
	for _, name := range []string{"session.json", "config.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("invalid"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--config-dir", dir, "history", "list", "--fields", "id,collected_at", "--compact"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "[]\n" {
		t.Fatalf("offline output: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "history.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("read created database: %v", err)
	}
	cmd = newRootCmd()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--config-dir", t.TempDir(), "ops", "list", "--query", "history_", "--fields", "count,operations.id", "--compact"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Count      int              `json:"count"`
		Operations []map[string]any `json:"operations"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 5 || len(result.Operations) != 5 || strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("catalog projection: %s", out.String())
	}
	for _, op := range result.Operations {
		if len(op) != 1 || op["id"] == nil {
			t.Fatalf("projection leaked extra fields: %+v", op)
		}
	}
}

type historyFixture struct{ quantity float64 }

func (*historyFixture) ListAccounts(context.Context) ([]domain.Account, string, error) {
	return []domain.Account{{ID: "fixture-account"}}, "fixture-account", nil
}
func (*historyFixture) ConfirmationKey(string) []byte { return []byte("fixture-only-session") }
func (f *historyFixture) ListPositions(context.Context) ([]domain.Position, error) {
	return []domain.Position{{Symbol: "AAPL", ProductCode: "US-AAPL", Name: "Apple", Quantity: f.quantity, MarketValue: f.quantity * 100}}, nil
}
func (*historyFixture) ListTransactions(context.Context, string, time.Time, time.Time, string, int, int) (domain.TransactionPage, error) {
	return domain.TransactionPage{LastPage: true, Items: []domain.Transaction{{SortKey: "tx-one", Date: "2026-09-01", StockCode: "AAPL", StockName: "Apple", Currency: "USD", Amount: 10, Quantity: 0.5}}}, nil
}

func TestSavedHistoryThroughCLIAndMCP(t *testing.T) {
	t.Setenv("TOSSCTL_OPENAPI_KEY", "")
	t.Setenv("TOSSCTL_OPENAPI_SECRET", "")
	dir := t.TempDir()
	fixture := &historyFixture{quantity: 1}
	service := history.New(filepath.Join(dir, "history.sqlite"), fixture)
	intent := history.SyncOptions{From: "2026-09-01", To: "2026-09-11", Market: "us"}
	for _, quantity := range []float64{1, 2} {
		fixture.quantity = quantity
		preview, err := service.Sync(context.Background(), intent, false, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = service.Sync(context.Background(), intent, true, preview.ConfirmToken); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"history", "positions", "--snapshot", "1", "--fields", "snapshot.id,positions.symbol,positions.quantity", "--compact"}, `"quantity":1`},
		{[]string{"history", "search", "Apple", "--snapshot", "1", "--output", "json"}, `"currency": "USD"`},
		{[]string{"history", "search", "Apple", "--snapshot", "1", "--output", "JSON"}, `"currency": "USD"`},
		{[]string{"history", "transactions", "--snapshot", "1", "--output", "csv"}, "snapshot_id,collected_at,date,market,currency"},
		{[]string{"history", "compare", "1", "2", "--output", "json"}, `"quantity_delta": 1`},
		{[]string{"history", "positions", "--snapshot", "1"}, "WTS historical holdings"},
		{[]string{"ops", "call", "history_compare", "--params", `{"before":1,"after":2}`, "--fields", "changes.symbol,changes.quantity_delta", "--compact"}, `"quantity_delta":1`},
	} {
		cmd := newRootCmd()
		var out, stderr bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs(append([]string{"--config-dir", dir}, tc.args...))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v: %s", tc.args, out.String())
		}
	}
	cmd := newRootCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config-dir", dir, "mcp"})
	cmd.SetIn(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"history_positions","params":{"snapshot":1},"fields":["snapshot.id","positions.symbol"]}}}` + "\n"))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "AAPL") || strings.Contains(out.String(), "quantity") || strings.Contains(out.String(), `"isError":true`) {
		t.Fatalf("MCP stored history: %s", out.String())
	}
}

func TestJSONFlagErrorsPrecedeCommandExecution(t *testing.T) {
	for _, args := range [][]string{{"history", "list", "--fields", "id..name"}, {"history", "sync", "--output", "csv", "--fields", "id"}, {"mcp", "--compact"}, {"push", "listen", "--fields", "type"}} {
		cmd := newRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{"--config-dir", t.TempDir()}, args...))
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestMCPStartsWithoutCredentialsForLocalHistory(t *testing.T) {
	t.Setenv("TOSSCTL_OPENAPI_KEY", "")
	t.Setenv("TOSSCTL_OPENAPI_SECRET", "")
	cmd := newRootCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"call_operation","arguments":{"operation":"history_list"}}}` + "\n"))
	cmd.SetArgs([]string{"--config-dir", t.TempDir(), "mcp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"text":"[]"`) || strings.Contains(out.String(), `"isError":true`) {
		t.Fatalf("MCP local result: %s", out.String())
	}
}
