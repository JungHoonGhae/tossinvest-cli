package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
)

// Dummy key material only.
const setupTestKey = "tsck_live_DUMMYKEY0000000000000"

func TestClassifyKeySetup(t *testing.T) {
	ok := &probeResult{OK: true}
	ipBlocked := &probeResult{ErrorKind: "ip_not_allowed"}
	badSecret := &probeResult{ErrorKind: "auth"}
	same := &official.Credentials{APIKey: setupTestKey, SecretKey: "s"}
	other := &official.Credentials{APIKey: "tsck_live_OTHER", SecretKey: "s"}
	cases := []struct {
		name   string
		server string
		local  *official.Credentials
		probe  *probeResult
		want   keySetupState
	}{
		{"no key at Toss", "", same, ok, keyMissing},
		{"saved key works", setupTestKey, same, ok, keyConnected},
		{"saved key, IP blocked", setupTestKey, same, ipBlocked, keyConnectedIP},
		{"saved key, wrong secret", setupTestKey, same, badSecret, keyServerOnly},
		{"nothing saved", setupTestKey, nil, nil, keyServerOnly},
		{"another key saved", setupTestKey, other, nil, keyServerOnly},
	}
	for _, tc := range cases {
		if got := classifyKeySetup(tc.server, tc.local, tc.probe); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

type fakeIssueClient struct {
	statuses []string
	calls    int
}

func (f *fakeIssueClient) OpenAPIClientInfo(context.Context) (tossclient.OpenAPIClientInfo, error) {
	return tossclient.OpenAPIClientInfo{}, nil
}
func (f *fakeIssueClient) OpenAPIKeyTerms(context.Context) ([]tossclient.OpenAPITerm, error) {
	return nil, nil
}
func (f *fakeIssueClient) StartOpenAPIKeyIssue(context.Context, tossclient.OpenAPIIssueKind) error {
	return nil
}
func (f *fakeIssueClient) OpenAPIKeyIssueStatus(context.Context, tossclient.OpenAPIIssueKind) (tossclient.ExtensionStatus, error) {
	i := f.calls
	f.calls++
	if i >= len(f.statuses) {
		i = len(f.statuses) - 1
	}
	if f.statuses[i] == "ERR" {
		return tossclient.ExtensionStatus{}, errors.New("transient")
	}
	return tossclient.ExtensionStatus{Status: f.statuses[i]}, nil
}
func (f *fakeIssueClient) CompleteOpenAPIKeyIssue(context.Context, tossclient.OpenAPIIssueKind) (tossclient.IssuedOpenAPIKey, error) {
	return tossclient.IssuedOpenAPIKey{}, nil
}

func TestWaitForKeyIssueApproval(t *testing.T) {
	ctx := context.Background()
	approved := &fakeIssueClient{statuses: []string{"REQUESTED", "ERR", "COMPLETED"}}
	if err := waitForKeyIssueApproval(ctx, approved, tossclient.OpenAPIReissue, time.Second, time.Millisecond); err != nil {
		t.Fatalf("approval after a transient error must succeed, got %v", err)
	}
	rejected := &fakeIssueClient{statuses: []string{"REQUESTED", "EXPIRED"}}
	if err := waitForKeyIssueApproval(ctx, rejected, tossclient.OpenAPIReissue, time.Second, time.Millisecond); !errors.Is(err, errKeyIssueRejected) {
		t.Fatalf("want rejected, got %v", err)
	}
	pending := &fakeIssueClient{statuses: []string{"REQUESTED"}}
	if err := waitForKeyIssueApproval(ctx, pending, tossclient.OpenAPIReissue, 20*time.Millisecond, time.Millisecond); !errors.Is(err, errKeyIssueTimeout) {
		t.Fatalf("want timeout, got %v", err)
	}
}

// An issued pair is saved even when the connection check fails: Toss never
// shows the secret again. The stale token cache is dropped.
func TestSaveIssuedCredentialsKeepsThePairAndDropsTheTokenCache(t *testing.T) {
	dir := t.TempDir()
	credFile := filepath.Join(dir, "openapi-credentials.json")
	tokenFile := filepath.Join(dir, "openapi-token.json")
	if err := os.WriteFile(tokenFile, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	creds := official.Credentials{APIKey: setupTestKey, SecretKey: "tssk_live_DUMMYSECRET000000000"}
	if err := saveIssuedCredentials(&out, creds, credFile, tokenFile, probeResult{ErrorKind: "server_error", Message: "down"}); err != nil {
		t.Fatal(err)
	}
	saved, err := official.LoadCredentials(func(string) string { return "" }, credFile)
	if err != nil || saved == nil || saved.APIKey != creds.APIKey || saved.SecretKey != creds.SecretKey {
		t.Fatalf("issued pair must be saved, got %+v, %v", saved, err)
	}
	if _, err := os.Stat(tokenFile); !os.IsNotExist(err) {
		t.Fatalf("token cache must be dropped, stat err=%v", err)
	}
	if bytes.Contains(out.Bytes(), []byte("DUMMYSECRET")) {
		t.Fatalf("the secret must never be printed: %q", out.String())
	}
}

func TestPhoneLoginValidationAndEnvPath(t *testing.T) {
	if _, err := newPhoneLogin("홍길동", "900230", "01000000000"); err == nil {
		t.Fatal("an impossible birth date (Feb 30) must be rejected")
	}
	if _, err := newPhoneLogin("홍길동", "900101", "02-123-4567"); err == nil {
		t.Fatal("a landline must be rejected")
	}
	login, err := newPhoneLogin(" 홍길동 ", "900101", "010-0000-0000")
	if err != nil || login.Name != "홍길동" || login.Number != "01000000000" {
		t.Fatalf("want trimmed name and digits-only phone, got %+v, %v", login, err)
	}
	// Tests run without a TTY: the environment path is the one exercised.
	env := map[string]string{envLoginName: "홍길동", envLoginBirth: "900101", envLoginPhone: "01000000000"}
	got, ok, err := resolvePhoneLogin(&bytes.Buffer{}, func(k string) string { return env[k] })
	if err != nil || !ok || got.Birth != "900101" {
		t.Fatalf("env path: %+v, %v, %v", got, ok, err)
	}
	if _, _, err := resolvePhoneLogin(&bytes.Buffer{}, func(string) string { return "" }); err == nil {
		t.Fatal("missing env must explain how to provide the details")
	}
}
