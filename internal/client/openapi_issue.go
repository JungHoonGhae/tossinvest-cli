package client

// openapi_issue.go — issue (registration) or reissue the Open API key through
// the WTS session, the same flow the Toss web uses (bundle modules 84228 and
// 89707, read 2026-11-07):
//
//	POST /api/v2/open-api/{kind}/cert/push      → certificate approval push to the Toss app
//	GET  /api/v2/open-api/{kind}/cert/status    → poll until approved
//	POST /api/v2/open-api/{kind}/cert/complete  {termsKeys:[1222,1223]} → new key + secret
//
// The status endpoint answers 403 unauthorized.session.invalid-state until a
// push has started the flow. A reissue revokes the previous key the moment it
// completes, so callers must confirm that with the user first.
//
// The secret is only ever returned here: GET /api/v1/openapi/client masks it
// (tssk_live_****, verified 2026-11-07), so it cannot be recovered later.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// OpenAPIKeyTermsKeys are the terms the web sends with cert/complete:
// 1222 오픈 API 서비스 이용 약관, 1223 개인(신용)정보 수집·이용 동의.
var OpenAPIKeyTermsKeys = []int{1222, 1223}

// OpenAPIIssueKind selects first issuance or reissue (which revokes the old key).
type OpenAPIIssueKind string

const (
	OpenAPIRegistration OpenAPIIssueKind = "registration"
	OpenAPIReissue      OpenAPIIssueKind = "reissue"
)

// Literal paths (not concatenated) so the WTS inventory can own each one.
var openAPIIssuePaths = map[OpenAPIIssueKind]map[string]string{
	OpenAPIRegistration: {
		"push":     "/api/v2/open-api/registration/cert/push",
		"status":   "/api/v2/open-api/registration/cert/status",
		"complete": "/api/v2/open-api/registration/cert/complete",
	},
	OpenAPIReissue: {
		"push":     "/api/v2/open-api/reissue/cert/push",
		"status":   "/api/v2/open-api/reissue/cert/status",
		"complete": "/api/v2/open-api/reissue/cert/complete",
	},
}

func (k OpenAPIIssueKind) path(step string) (string, error) {
	if p, ok := openAPIIssuePaths[k][step]; ok {
		return p, nil
	}
	return "", fmt.Errorf("unknown open api issue step %q/%q", k, step)
}

// OpenAPITerm is one term the user agrees to when a key is issued.
type OpenAPITerm struct {
	Key      int    `json:"termsKey"`
	Title    string `json:"title"`
	URL      string `json:"contentsUrl"`
	Required bool   `json:"required"`
}

// OpenAPIKeyTerms lists the terms agreed at issuance so they can be shown
// before the user consents.
func (c *Client) OpenAPIKeyTerms(ctx context.Context) ([]OpenAPITerm, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{"termsKeys": OpenAPIKeyTermsKeys})
	var env struct {
		Result []OpenAPITerm `json:"result"`
	}
	if err := c.postJSON(ctx, c.certBaseURL+"/api/v1/account/frontend/terms/list", body, &env); err != nil {
		return nil, err
	}
	return env.Result, nil
}

// StartOpenAPIKeyIssue sends the certificate approval push to the Toss app.
func (c *Client) StartOpenAPIKeyIssue(ctx context.Context, kind OpenAPIIssueKind) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	path, err := kind.path("push")
	if err != nil {
		return err
	}
	var ignored json.RawMessage
	return c.postJSON(ctx, c.apiBaseURL+path, json.RawMessage(`{}`), &ignored)
}

// OpenAPIKeyIssueStatus reports the approval state of a started issuance. The
// certificate flow is shared with session extension (REQUESTED → COMPLETED,
// EXPIRED when invalidated), so ExtensionStatus classifies it.
func (c *Client) OpenAPIKeyIssueStatus(ctx context.Context, kind OpenAPIIssueKind) (ExtensionStatus, error) {
	if err := c.requireSession(); err != nil {
		return ExtensionStatus{}, err
	}
	path, err := kind.path("status")
	if err != nil {
		return ExtensionStatus{}, err
	}
	var env struct {
		Result json.RawMessage `json:"result"`
	}
	if err := c.getJSON(ctx, c.apiBaseURL+path, &env); err != nil {
		return ExtensionStatus{}, err
	}
	return ExtensionStatus{Status: statusString(env.Result)}, nil
}

// statusString accepts both observed shapes of certificate status results:
// a bare string ("REQUESTED") or an object carrying a status field.
func statusString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		for _, key := range []string{"status", "state", "result"} {
			if v, ok := obj[key].(string); ok {
				return v
			}
		}
	}
	return ""
}

// IssuedOpenAPIKey is the key pair returned once by cert/complete.
type IssuedOpenAPIKey struct {
	APIKey    string
	SecretKey string
}

var (
	issuedKeyPattern    = regexp.MustCompile(`^tsck_live_[A-Za-z0-9]+$`)
	issuedSecretPattern = regexp.MustCompile(`^tssk_live_[A-Za-z0-9]+$`)
)

// ErrIssuedKeyMissing means cert/complete succeeded but no usable key pair
// was found in its response. The key may already be issued (and, on reissue,
// the old one revoked): the caller must say so instead of retrying blindly.
var ErrIssuedKeyMissing = errors.New("open api key issuance finished but the response carried no key and secret")

// CompleteOpenAPIKeyIssue agrees to the issuance terms and finalizes the key.
// The response is never logged: it holds the plaintext secret.
func (c *Client) CompleteOpenAPIKeyIssue(ctx context.Context, kind OpenAPIIssueKind) (IssuedOpenAPIKey, error) {
	if err := c.requireSession(); err != nil {
		return IssuedOpenAPIKey{}, err
	}
	path, err := kind.path("complete")
	if err != nil {
		return IssuedOpenAPIKey{}, err
	}
	body, _ := json.Marshal(map[string]any{"termsKeys": OpenAPIKeyTermsKeys})
	var env struct {
		Result json.RawMessage `json:"result"`
	}
	if err := c.postJSON(ctx, c.apiBaseURL+path, body, &env); err != nil {
		return IssuedOpenAPIKey{}, err
	}
	key, ok := findIssuedKey(env.Result)
	if !ok {
		return IssuedOpenAPIKey{}, ErrIssuedKeyMissing
	}
	return key, nil
}

// findIssuedKey locates the key pair by its value shape rather than a field
// name: the complete response was not observable without revoking a live key.
// A masked secret (tssk_live_****) never matches.
func findIssuedKey(raw json.RawMessage) (IssuedOpenAPIKey, bool) {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return IssuedOpenAPIKey{}, false
	}
	var out IssuedOpenAPIKey
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for _, item := range t {
				walk(item)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		case string:
			s := strings.TrimSpace(t)
			if out.APIKey == "" && issuedKeyPattern.MatchString(s) {
				out.APIKey = s
			}
			if out.SecretKey == "" && issuedSecretPattern.MatchString(s) {
				out.SecretKey = s
			}
		}
	}
	walk(root)
	return out, out.APIKey != "" && out.SecretKey != ""
}
