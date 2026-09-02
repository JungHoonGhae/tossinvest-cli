package openapiip

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type fakeAllowlistClient struct {
	ips                 []string
	added               []string
	deleted             []string
	calls               []string
	addErrors           map[string]error
	addAppliedErrors    map[string]error
	deleteErrors        map[string]error
	deleteAppliedErrors map[string]error
	skipAdds            map[string]bool
	addHook             func(context.Context, string) error
}

func (f *fakeAllowlistClient) OpenAPIAllowedIPs(context.Context) ([]string, error) {
	return append([]string(nil), f.ips...), nil
}

func (f *fakeAllowlistClient) AddOpenAPIAllowedIP(ctx context.Context, ip string) error {
	f.calls = append(f.calls, "add:"+ip)
	f.added = append(f.added, ip)
	if f.addHook != nil {
		if err := f.addHook(ctx, ip); err != nil {
			return err
		}
	}
	if err := f.addErrors[ip]; err != nil {
		return err
	}
	if f.skipAdds[ip] {
		return nil
	}
	f.ips = append(f.ips, ip)
	if err := f.addAppliedErrors[ip]; err != nil {
		return err
	}
	return nil
}

func (f *fakeAllowlistClient) DeleteOpenAPIAllowedIP(_ context.Context, ip string) error {
	f.calls = append(f.calls, "delete:"+ip)
	f.deleted = append(f.deleted, ip)
	if err := f.deleteErrors[ip]; err != nil {
		return err
	}
	for i, existing := range f.ips {
		if existing == ip {
			f.ips = append(f.ips[:i], f.ips[i+1:]...)
			break
		}
	}
	if err := f.deleteAppliedErrors[ip]; err != nil {
		return err
	}
	return nil
}

func TestListNormalizesSortsAndDeduplicates(t *testing.T) {
	t.Parallel()
	client := &fakeAllowlistClient{ips: []string{" 203.0.113.7 ", "::ffff:198.51.100.9", "203.0.113.7"}}
	got, err := NewService(client, staticResolver("")).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"198.51.100.9", "203.0.113.7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
}

type staticResolver string

func (r staticResolver) CurrentPublicIP(context.Context) (string, error) {
	return string(r), nil
}

func TestReplaceCurrentPreviewDoesNotMutate(t *testing.T) {
	t.Parallel()
	client := &fakeAllowlistClient{ips: []string{"203.0.113.7"}}
	service := NewService(client, staticResolver("198.51.100.9"))

	plan, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if plan.Kind != "openapi_ip_replace_current" || plan.CurrentIP != "198.51.100.9" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if !reflect.DeepEqual(plan.ExistingIPs, []string{"203.0.113.7"}) ||
		!reflect.DeepEqual(plan.DeleteIPs, []string{"203.0.113.7"}) ||
		plan.AddIP != "198.51.100.9" || plan.ConfirmToken == "" || plan.Applied {
		t.Fatalf("unexpected preview: %#v", plan)
	}
	if len(client.added) != 0 || len(client.deleted) != 0 {
		t.Fatalf("preview mutated allowlist: added=%v deleted=%v", client.added, client.deleted)
	}
}

func TestReplaceCurrentRejectsWrongConfirmationBeforeMutation(t *testing.T) {
	t.Parallel()
	client := &fakeAllowlistClient{ips: []string{"203.0.113.7"}}
	service := NewService(client, staticResolver("198.51.100.9"))

	_, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{Execute: true, Confirm: "wrong"})
	if err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if len(client.added) != 0 || len(client.deleted) != 0 {
		t.Fatalf("wrong confirmation mutated allowlist: added=%v deleted=%v", client.added, client.deleted)
	}
}

func TestReplaceCurrentExecutesConfirmedPlan(t *testing.T) {
	t.Parallel()
	client := &fakeAllowlistClient{ips: []string{"203.0.113.7"}}
	service := NewService(client, staticResolver("198.51.100.9"))
	preview, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	result, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{
		Execute: true,
		Confirm: preview.ConfirmToken,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.Applied || !reflect.DeepEqual(client.deleted, []string{"203.0.113.7"}) ||
		!reflect.DeepEqual(client.added, []string{"198.51.100.9"}) ||
		!reflect.DeepEqual(client.ips, []string{"198.51.100.9"}) ||
		!reflect.DeepEqual(client.calls, []string{"add:198.51.100.9", "delete:203.0.113.7"}) {
		t.Fatalf("unexpected execution: result=%#v added=%v deleted=%v ips=%v", result, client.added, client.deleted, client.ips)
	}
}

func TestReplaceCurrentConfirmedNoopPerformsNoMutation(t *testing.T) {
	t.Parallel()
	client := &fakeAllowlistClient{ips: []string{"198.51.100.9"}}
	service := NewService(client, staticResolver("198.51.100.9"))
	preview, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Noop || preview.AddIP != "" || len(preview.DeleteIPs) != 0 {
		t.Fatalf("preview = %#v", preview)
	}
	result, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || len(client.added) != 0 || len(client.deleted) != 0 {
		t.Fatalf("noop mutated allowlist: result=%#v added=%v deleted=%v", result, client.added, client.deleted)
	}
}

func TestReplaceCurrentPreservesOldIPsWhenAddFails(t *testing.T) {
	t.Parallel()
	client := &fakeAllowlistClient{
		ips:       []string{"203.0.113.7"},
		addErrors: map[string]error{"198.51.100.9": errors.New("add failed")},
	}
	service := NewService(client, staticResolver("198.51.100.9"))
	preview, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	_, err = service.ReplaceCurrent(context.Background(), ExecuteOptions{
		Execute: true,
		Confirm: preview.ConfirmToken,
	})
	if err == nil || !strings.Contains(err.Error(), "restored previous allowlist") {
		t.Fatalf("expected rollback error, got %v", err)
	}
	if !reflect.DeepEqual(client.added, []string{"198.51.100.9"}) || len(client.deleted) != 0 ||
		!reflect.DeepEqual(client.ips, []string{"203.0.113.7"}) {
		t.Fatalf("rollback failed: added=%v ips=%v", client.added, client.ips)
	}
}

func TestReplaceCurrentRollbackSurvivesCancelledRequest(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeAllowlistClient{ips: []string{"203.0.113.7"}}
	client.addHook = func(callCtx context.Context, ip string) error {
		if ip == "198.51.100.9" {
			cancel()
			return errors.New("add failed after cancellation")
		}
		return callCtx.Err()
	}
	service := NewService(client, staticResolver("198.51.100.9"))
	preview, err := service.ReplaceCurrent(ctx, ExecuteOptions{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	_, err = service.ReplaceCurrent(ctx, ExecuteOptions{
		Execute: true,
		Confirm: preview.ConfirmToken,
	})
	if err == nil || !strings.Contains(err.Error(), "restored previous allowlist") {
		t.Fatalf("expected successful rollback after cancellation, got %v", err)
	}
	if !reflect.DeepEqual(client.ips, []string{"203.0.113.7"}) {
		t.Fatalf("cancelled request left allowlist changed: %v", client.ips)
	}
}

func TestReplaceCurrentReportsDeleteAndRollbackFailures(t *testing.T) {
	t.Parallel()
	t.Run("delete failure restores previous state", func(t *testing.T) {
		client := &fakeAllowlistClient{
			ips:          []string{"203.0.113.7"},
			deleteErrors: map[string]error{"203.0.113.7": errors.New("delete failed")},
		}
		service := NewService(client, staticResolver("198.51.100.9"))
		preview, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.ReplaceCurrent(context.Background(), ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
		if err == nil || !strings.Contains(err.Error(), "delete allowed IP") {
			t.Fatalf("delete error = %v", err)
		}
		if !reflect.DeepEqual(client.ips, []string{"203.0.113.7"}) {
			t.Fatalf("failed delete changed state: %#v", client)
		}
	})

	t.Run("rollback failure names affected address", func(t *testing.T) {
		client := &fakeAllowlistClient{
			ips:                 []string{"203.0.113.7"},
			addErrors:           map[string]error{"203.0.113.7": errors.New("restore failed")},
			deleteErrors:        map[string]error{"198.51.100.9": errors.New("remove failed")},
			deleteAppliedErrors: map[string]error{"203.0.113.7": errors.New("response lost")},
		}
		service := NewService(client, staticResolver("198.51.100.9"))
		preview, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.ReplaceCurrent(context.Background(), ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
		if err == nil || !strings.Contains(err.Error(), "rollback failed for missing 203.0.113.7") {
			t.Fatalf("rollback error = %v", err)
		}
		if !reflect.DeepEqual(client.ips, []string{"198.51.100.9"}) {
			t.Fatalf("rollback must retain the verified current IP when restoration fails: %v", client.ips)
		}
	})
}

func TestReplaceCurrentReconcilesAmbiguousMutationErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		client *fakeAllowlistClient
	}{
		{
			name: "delete applied before response error",
			client: &fakeAllowlistClient{
				ips:                 []string{"203.0.113.7"},
				deleteAppliedErrors: map[string]error{"203.0.113.7": errors.New("response lost")},
			},
		},
		{
			name: "add applied before response error",
			client: &fakeAllowlistClient{
				ips:              []string{"203.0.113.7"},
				addAppliedErrors: map[string]error{"198.51.100.9": errors.New("response lost")},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService(tc.client, staticResolver("198.51.100.9"))
			preview, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.ReplaceCurrent(context.Background(), ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
			if err == nil || !strings.Contains(err.Error(), "restored previous allowlist") {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(tc.client.ips, []string{"203.0.113.7"}) {
				t.Fatalf("ambiguous failure left allowlist changed: %v", tc.client.ips)
			}
		})
	}
}

func TestReplaceCurrentReconcilesPostMutationVerificationMismatch(t *testing.T) {
	t.Parallel()
	client := &fakeAllowlistClient{
		ips:      []string{"203.0.113.7"},
		skipAdds: map[string]bool{"198.51.100.9": true},
	}
	service := NewService(client, staticResolver("198.51.100.9"))
	preview, err := service.ReplaceCurrent(context.Background(), ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ReplaceCurrent(context.Background(), ExecuteOptions{Execute: true, Confirm: preview.ConfirmToken})
	if err == nil || !strings.Contains(err.Error(), "verify current public IP before deleting old entries") || !strings.Contains(err.Error(), "restored previous allowlist") {
		t.Fatalf("verification error = %v", err)
	}
	if !reflect.DeepEqual(client.ips, []string{"203.0.113.7"}) {
		t.Fatalf("verification mismatch was not reconciled: %v", client.ips)
	}
}

func TestHTTPResolverValidatesResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(" 198.51.100.9\n"))
	}))
	t.Cleanup(server.Close)

	resolver := NewHTTPResolver(server.Client(), server.URL)
	got, err := resolver.CurrentPublicIP(context.Background())
	if err != nil || got != "198.51.100.9" {
		t.Fatalf("CurrentPublicIP = %q, %v", got, err)
	}
}

func TestHTTPResolverRejectsBadResponses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "non-2xx", status: http.StatusBadGateway, body: "upstream failed", want: "HTTP 502"},
		{name: "invalid address", status: http.StatusOK, body: "not-an-ip", want: "invalid IP address"},
		{name: "oversized garbage", status: http.StatusOK, body: strings.Repeat("x", 300), want: "invalid IP address"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)
			_, err := NewHTTPResolver(server.Client(), server.URL).CurrentPublicIP(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	_, err := NewHTTPResolver(server.Client(), server.URL).CurrentPublicIP(context.Background())
	if err == nil {
		t.Fatal("transport failure must be returned")
	}
}
