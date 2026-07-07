package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestCancelConditionalOrderIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/conditional-orders/co-1":
			if r.Header.Get("X-Tossinvest-Account") != "1" {
				t.Errorf("account header: got %q", r.Header.Get("X-Tossinvest-Account"))
			}
			_, _ = w.Write([]byte(`{"result":null}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(1),
	)
	if err := c.CancelConditionalOrder(context.Background(), "co-1"); err != nil {
		t.Fatalf("CancelConditionalOrder: %v", err)
	}
}
