package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// Dummy key material only.
const (
	dummyIssuedKey    = "tsck_live_DUMMYKEY0000000000000"
	dummyIssuedSecret = "tssk_live_DUMMYSECRET000000000"
)

func TestOpenAPIKeyIssueFlowContract(t *testing.T) {
	var completeBody map[string]any
	pushed := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/open-api/reissue/cert/push", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("push method %s", r.Method)
		}
		pushed = true
		_, _ = io.WriteString(w, `{"result":null}`)
	})
	mux.HandleFunc("/api/v2/open-api/reissue/cert/status", func(w http.ResponseWriter, r *http.Request) {
		if !pushed {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":{"code":"unauthorized.session.invalid-state"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"result":"COMPLETED"}`)
	})
	mux.HandleFunc("/api/v2/open-api/reissue/cert/complete", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&completeBody)
		_, _ = io.WriteString(w, `{"result":{"client":{"clientId":"`+dummyIssuedKey+`","clientSecret":"`+dummyIssuedSecret+`"}}}`)
	})
	c := newOpenAPITestClient(t, mux)
	ctx := context.Background()

	if err := c.StartOpenAPIKeyIssue(ctx, OpenAPIReissue); err != nil {
		t.Fatal(err)
	}
	st, err := c.OpenAPIKeyIssueStatus(ctx, OpenAPIReissue)
	if err != nil || !st.Approved() {
		t.Fatalf("want approved, got %+v, %v", st, err)
	}
	key, err := c.CompleteOpenAPIKeyIssue(ctx, OpenAPIReissue)
	if err != nil {
		t.Fatal(err)
	}
	if key.APIKey != dummyIssuedKey || key.SecretKey != dummyIssuedSecret {
		t.Fatalf("unexpected key pair %+v", key)
	}
	terms, _ := completeBody["termsKeys"].([]any)
	if len(terms) != 2 || terms[0] != 1222.0 || terms[1] != 1223.0 {
		t.Fatalf("complete must agree to terms 1222,1223, sent %v", completeBody)
	}
}

func TestOpenAPIKeyIssueStatusAcceptsObjectShape(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/open-api/registration/cert/status", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"result":{"status":"REQUESTED"}}`)
	})
	c := newOpenAPITestClient(t, mux)
	st, err := c.OpenAPIKeyIssueStatus(context.Background(), OpenAPIRegistration)
	if err != nil || st.Status != "REQUESTED" || st.Approved() || st.Rejected() {
		t.Fatalf("want pending REQUESTED, got %+v, %v", st, err)
	}
}

// A masked secret (as GET /api/v1/openapi/client returns it) must never be
// taken as an issued secret.
func TestCompleteOpenAPIKeyIssueRejectsMaskedSecret(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/open-api/registration/cert/complete", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"result":{"clientId":"`+dummyIssuedKey+`","clientSecret":"tssk_live_********"}}`)
	})
	c := newOpenAPITestClient(t, mux)
	if _, err := c.CompleteOpenAPIKeyIssue(context.Background(), OpenAPIRegistration); !errors.Is(err, ErrIssuedKeyMissing) {
		t.Fatalf("want ErrIssuedKeyMissing, got %v", err)
	}
}

func TestOpenAPIKeyTermsUsesCertHost(t *testing.T) {
	cert := http.NewServeMux()
	cert.HandleFunc("/api/v1/account/frontend/terms/list", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"result":[{"termsKey":1222,"title":"오픈 API 서비스 이용 약관","contentsUrl":"https://example.invalid/terms","required":true}]}`)
	})
	srv := httptest.NewServer(cert)
	t.Cleanup(srv.Close)
	c := New(Config{HTTPClient: srv.Client(), CertBaseURL: srv.URL, Session: &session.Session{Cookies: map[string]string{"SESSION": "test-session"}}})
	terms, err := c.OpenAPIKeyTerms(context.Background())
	if err != nil || len(terms) != 1 || terms[0].Key != 1222 || !terms[0].Required {
		t.Fatalf("unexpected terms %+v, %v", terms, err)
	}
}

func TestOpenAPIIssueKindRejectsUnknown(t *testing.T) {
	if _, err := OpenAPIIssueKind("other").path("push"); err == nil {
		t.Fatal("unknown kind must be rejected")
	}
}
