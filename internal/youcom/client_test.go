package youcom

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return newClient("test-key", WithBaseURL(srv.URL))
}

func TestRequestConstruction(t *testing.T) {
	var gotMethod, gotPath, gotKey string
	var gotBody researchRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-API-Key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":{"content":"ok"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.Research(context.Background(), "삼성전자 전망", "standard"); err != nil {
		t.Fatalf("Research: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	// srv.URL has no path component beyond "/", so baseURL == srv.URL means
	// the request path is exactly "/".
	if gotPath != "/" {
		t.Errorf("path = %q, want /", gotPath)
	}
	if gotKey != "test-key" {
		t.Errorf("X-API-Key = %q, want test-key", gotKey)
	}
	if gotBody.Input != "삼성전자 전망" {
		t.Errorf("input = %q, want 삼성전자 전망", gotBody.Input)
	}
	if gotBody.ResearchEffort != "standard" {
		t.Errorf("research_effort = %q, want standard", gotBody.ResearchEffort)
	}
}

func TestResearchEffortDefault(t *testing.T) {
	var gotBody researchRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":{"content":"ok"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.Research(context.Background(), "query", ""); err != nil {
		t.Fatalf("Research: %v", err)
	}
	if gotBody.ResearchEffort != EffortLite {
		t.Errorf("default effort = %q, want %q", gotBody.ResearchEffort, EffortLite)
	}
}

func TestResearchEffortValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for an invalid effort")
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Research(context.Background(), "query", "ultra")
	if err == nil {
		t.Fatal("expected error for invalid effort, got nil")
	}
	if !strings.Contains(err.Error(), "ultra") {
		t.Errorf("error should mention the bad value: %v", err)
	}
}

func TestResearchEffortAllValidValues(t *testing.T) {
	for _, effort := range []string{EffortLite, EffortStandard, EffortDeep, EffortExhaustive} {
		effort := effort
		t.Run(effort, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"output":{"content":"ok"}}`))
			}))
			defer srv.Close()
			c := newTestClient(t, srv)
			if _, err := c.Research(context.Background(), "query", effort); err != nil {
				t.Errorf("effort %q should be accepted: %v", effort, err)
			}
		})
	}
}

func TestResearchMappingContentAndSources(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":{"content":"삼성전자 실적 요약","sources":[{"url":"https://example.com/a","title":"A","snippets":["s1","s2"]},{"url":"https://example.com/b","title":"B"}]}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	res, err := c.Research(context.Background(), "query", "lite")
	if err != nil {
		t.Fatalf("Research: %v", err)
	}
	if res.Output.Content != "삼성전자 실적 요약" {
		t.Errorf("content = %q", res.Output.Content)
	}
	if len(res.Output.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(res.Output.Sources))
	}
	if res.Output.Sources[0].URL != "https://example.com/a" || res.Output.Sources[0].Title != "A" {
		t.Errorf("source[0] = %+v", res.Output.Sources[0])
	}
	if len(res.Output.Sources[0].Snippets) != 2 {
		t.Errorf("source[0].Snippets = %v, want 2 items", res.Output.Sources[0].Snippets)
	}
	if len(res.Output.Sources[1].Snippets) != 0 {
		t.Errorf("source[1].Snippets = %v, want none", res.Output.Sources[1].Snippets)
	}
}

func TestResearchMappingSourcesAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":{"content":"only content, no sources field"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	res, err := c.Research(context.Background(), "query", "lite")
	if err != nil {
		t.Fatalf("Research: %v", err)
	}
	if res.Output.Content == "" {
		t.Error("expected non-empty content")
	}
	if res.Output.Sources != nil {
		t.Errorf("Sources = %v, want nil", res.Output.Sources)
	}
}

func TestResearchMappingEmptyOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":{}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	res, err := c.Research(context.Background(), "query", "lite")
	if err != nil {
		t.Fatalf("Research should not error on an empty output: %v", err)
	}
	if res.Output.Content != "" || res.Output.Sources != nil {
		t.Errorf("expected zero-value Output, got %+v", res.Output)
	}
}

func TestResearchErrorStatusCodes(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{http.StatusUnauthorized, ErrAuth},
		{http.StatusForbidden, ErrAuth},
		{http.StatusUnprocessableEntity, ErrInvalidRequest},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusInternalServerError, ErrServer},
	}
	for _, c := range cases {
		c := c
		t.Run(http.StatusText(c.code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.code)
				_, _ = w.Write([]byte(`{"error":"details"}`))
			}))
			defer srv.Close()

			cl := newTestClient(t, srv)
			_, err := cl.Research(context.Background(), "query", "lite")
			if err == nil {
				t.Fatalf("expected error for status %d", c.code)
			}
			if !errors.Is(err, c.want) {
				t.Errorf("status %d: err = %v, want wrapping %v", c.code, err, c.want)
			}
			if strings.Contains(err.Error(), "test-key") {
				t.Errorf("error must never echo the API key: %v", err)
			}
		})
	}
}

func TestResearchMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Research(context.Background(), "query", "lite")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("err = %v, want wrapping ErrMalformed", err)
	}
}

func TestResearchTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":{"content":"too slow"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := c.Research(ctx, "query", "lite")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("err = %v, want wrapping ErrTimeout", err)
	}
}

func TestNewClientMissingAPIKey(t *testing.T) {
	old, hadOld := os.LookupEnv(EnvAPIKey)
	os.Unsetenv(EnvAPIKey)
	defer func() {
		if hadOld {
			os.Setenv(EnvAPIKey, old)
		}
	}()

	_, err := NewClient()
	if err == nil {
		t.Fatal("expected error when YDC_API_KEY is unset")
	}
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Errorf("err = %v, want wrapping ErrMissingAPIKey", err)
	}
	if !strings.Contains(err.Error(), "YDC_API_KEY") {
		t.Errorf("error should mention YDC_API_KEY for guidance: %v", err)
	}
}

func TestNewClientReadsAPIKeyFromEnv(t *testing.T) {
	old, hadOld := os.LookupEnv(EnvAPIKey)
	os.Setenv(EnvAPIKey, "from-env")
	defer func() {
		if hadOld {
			os.Setenv(EnvAPIKey, old)
		} else {
			os.Unsetenv(EnvAPIKey)
		}
	}()

	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.apiKey != "from-env" {
		t.Errorf("apiKey = %q, want from-env", c.apiKey)
	}
}

// TestResearchIntegration hits the real You.com Research API once, at the
// cheapest ("lite") effort level. It is skipped unless YDC_API_KEY is set in
// the environment (see /home/mg/Desktop/you.com_projects/.env), so it never
// runs — and never costs anything — in normal offline CI/dev runs.
func TestResearchIntegration(t *testing.T) {
	if os.Getenv(EnvAPIKey) == "" {
		t.Skip("YDC_API_KEY not set; skipping live You.com Research integration test")
	}

	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	res, err := c.Research(ctx, "What is the current price of Samsung Electronics stock?", EffortLite)
	if err != nil {
		t.Fatalf("live Research call failed: %v", err)
	}
	if res.Output.Content == "" {
		t.Error("expected non-empty content from a live Research call")
	}
}
