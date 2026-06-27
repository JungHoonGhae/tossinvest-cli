package official

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestAdaptWarningsUnit verifies the pure adapter for Warnings.
func TestAdaptWarningsUnit(t *testing.T) {
	raw := []apiStockWarning{
		{
			WarningType: "INVESTMENT_WARNING",
			StartDate:   "2026-03-26",
			EndDate:     "2026-03-27",
			Exchange:    "KRX",
		},
	}
	got := adaptWarnings("005930", raw)
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("len(Warnings): want 1, got %d", len(got.Warnings))
	}
	w := got.Warnings[0]
	if w.Type != "INVESTMENT_WARNING" {
		t.Fatalf("Type: want INVESTMENT_WARNING, got %q", w.Type)
	}
	if w.Title != "" {
		t.Fatalf("Title: expected empty, got %q", w.Title)
	}
	if w.Text != "" {
		t.Fatalf("Text: expected empty, got %q", w.Text)
	}
	if w.Level != "" {
		t.Fatalf("Level: expected empty, got %q", w.Level)
	}
	// Raw must be non-nil and contain the warningType field.
	if w.Raw == nil {
		t.Fatal("Raw: expected non-nil")
	}
	var decoded map[string]any
	if err := json.Unmarshal(w.Raw, &decoded); err != nil {
		t.Fatalf("Raw unmarshal: %v", err)
	}
	if decoded["warningType"] != "INVESTMENT_WARNING" {
		t.Fatalf("Raw.warningType: want INVESTMENT_WARNING, got %v", decoded["warningType"])
	}
	if got.ProductCode != "" {
		t.Fatalf("ProductCode: expected empty, got %q", got.ProductCode)
	}
}

// TestAdaptWarningsEmpty verifies empty warnings slice handling.
func TestAdaptWarningsEmpty(t *testing.T) {
	got := adaptWarnings("AAPL", nil)
	if len(got.Warnings) != 0 {
		t.Fatalf("Warnings: want empty, got %d", len(got.Warnings))
	}
}

// TestWarningsIntegration tests Warnings() against an httptest server.
func TestWarningsIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/stocks/005930/warnings":
			_, _ = w.Write([]byte(`{"result":[{"warningType":"INVESTMENT_WARNING","startDate":"2026-03-26","endDate":"2026-03-27","exchange":"KRX"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)

	got, err := c.Warnings(context.Background(), "005930")
	if err != nil {
		t.Fatal(err)
	}
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("len(Warnings): want 1, got %d", len(got.Warnings))
	}
	if got.Warnings[0].Type != "INVESTMENT_WARNING" {
		t.Fatalf("Type: want INVESTMENT_WARNING, got %q", got.Warnings[0].Type)
	}
}
