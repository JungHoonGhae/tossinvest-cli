// Package openapiip owns the safe workflow for replacing the Toss Open API
// allowlist with the machine's current public IP. It keeps endpoint details in
// the WTS client and exposes one preview/confirm/execute transaction shared by
// the CLI and MCP surfaces.
package openapiip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/confirmation"
)

const defaultPublicIPURL = "https://api.ipify.org"

// AllowlistClient is the narrow WTS capability needed by Service.
type AllowlistClient interface {
	OpenAPIAllowedIPs(context.Context) ([]string, error)
	AddOpenAPIAllowedIP(context.Context, string) error
	DeleteOpenAPIAllowedIP(context.Context, string) error
}

// PublicIPResolver discovers the caller's public egress IP.
type PublicIPResolver interface {
	CurrentPublicIP(context.Context) (string, error)
}

// ExecuteOptions controls the two-step mutation boundary.
type ExecuteOptions struct {
	Execute bool
	Confirm string
}

// Plan is both the dry-run preview and the successful execution result.
type Plan struct {
	Kind         string   `json:"kind"`
	CurrentIP    string   `json:"current_ip"`
	ExistingIPs  []string `json:"existing_ips"`
	DeleteIPs    []string `json:"delete_ips"`
	AddIP        string   `json:"add_ip,omitempty"`
	Noop         bool     `json:"noop"`
	Applied      bool     `json:"applied"`
	Canonical    string   `json:"canonical"`
	ConfirmToken string   `json:"confirm_token"`
}

// Service coordinates public-IP discovery, allowlist reads, confirmation, and
// rollback. It deliberately exposes one high-level replacement operation so
// callers cannot accidentally implement delete-then-add without recovery.
type Service struct {
	client   AllowlistClient
	resolver PublicIPResolver
}

func NewService(client AllowlistClient, resolver PublicIPResolver) *Service {
	return &Service{client: client, resolver: resolver}
}

// List returns a normalized, stable allowlist.
func (s *Service) List(ctx context.Context) ([]string, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("Open API IP manager is not configured")
	}
	ips, err := s.client.OpenAPIAllowedIPs(ctx)
	if err != nil {
		return nil, err
	}
	return normalizeIPs(ips)
}

// ReplaceCurrent previews by default. Execution requires the exact token from
// a fresh preview; the token binds both the discovered public IP and the full
// existing allowlist. Any mutation or final-verification failure triggers a
// state-based reconciliation, because a timed-out HTTP mutation may still have
// been applied by the server.
func (s *Service) ReplaceCurrent(ctx context.Context, opts ExecuteOptions) (Plan, error) {
	plan, err := s.buildPlan(ctx)
	if err != nil {
		return Plan{}, err
	}
	if !opts.Execute {
		return plan, nil
	}
	if !confirmation.Matches(opts.Confirm, plan.ConfirmToken) {
		return Plan{}, fmt.Errorf("confirmation token mismatch; preview again and pass its confirm_token")
	}
	if plan.Noop {
		plan.Applied = true
		return plan, nil
	}

	if plan.AddIP != "" {
		if err := s.client.AddOpenAPIAllowedIP(ctx, plan.AddIP); err != nil {
			return Plan{}, s.reconcilePrevious(ctx, plan, fmt.Errorf("add current public IP: %w", err))
		}
		if err := s.verifyCurrentPresent(ctx, plan.CurrentIP); err != nil {
			return Plan{}, s.reconcilePrevious(ctx, plan, err)
		}
	}
	for _, ip := range plan.DeleteIPs {
		if err := s.client.DeleteOpenAPIAllowedIP(ctx, ip); err != nil {
			return Plan{}, s.reconcilePrevious(ctx, plan, fmt.Errorf("delete allowed IP %s: %w", ip, err))
		}
	}
	if err := s.verifyApplied(ctx, plan); err != nil {
		return Plan{}, s.reconcilePrevious(ctx, plan, err)
	}
	plan.Applied = true
	return plan, nil
}

func (s *Service) verifyCurrentPresent(ctx context.Context, currentIP string) error {
	actual, err := s.List(ctx)
	if err != nil {
		return fmt.Errorf("verify current public IP before deleting old entries: %w", err)
	}
	if _, present := stringSet(actual)[currentIP]; !present {
		return fmt.Errorf("verify current public IP before deleting old entries: %s is missing", currentIP)
	}
	return nil
}

func (s *Service) buildPlan(ctx context.Context) (Plan, error) {
	if s == nil || s.client == nil || s.resolver == nil {
		return Plan{}, fmt.Errorf("Open API IP manager is not configured")
	}
	current, err := s.resolver.CurrentPublicIP(ctx)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve current public IP: %w", err)
	}
	current, err = normalizeIP(current)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve current public IP: %w", err)
	}
	existing, err := s.List(ctx)
	if err != nil {
		return Plan{}, fmt.Errorf("list allowed IPs: %w", err)
	}

	deleteIPs := make([]string, 0, len(existing))
	currentPresent := false
	for _, ip := range existing {
		if ip == current {
			currentPresent = true
			continue
		}
		deleteIPs = append(deleteIPs, ip)
	}
	addIP := ""
	if !currentPresent {
		addIP = current
	}
	plan := Plan{
		Kind:        "openapi_ip_replace_current",
		CurrentIP:   current,
		ExistingIPs: existing,
		DeleteIPs:   deleteIPs,
		AddIP:       addIP,
		Noop:        len(deleteIPs) == 0 && addIP == "",
	}
	canonical, err := canonicalPlan(plan)
	if err != nil {
		return Plan{}, err
	}
	plan.Canonical = canonical
	plan.ConfirmToken = confirmation.Token(canonical)
	return plan, nil
}

func (s *Service) verifyApplied(ctx context.Context, plan Plan) error {
	actual, err := s.List(ctx)
	if err != nil {
		return fmt.Errorf("verify replaced allowlist: %w", err)
	}
	expected := []string{plan.CurrentIP}
	if !slices.Equal(actual, expected) {
		return fmt.Errorf("verify replaced allowlist: got %v, want %v", actual, expected)
	}
	return nil
}

func (s *Service) reconcilePrevious(ctx context.Context, plan Plan, cause error) error {
	// A client timeout/cancellation may be the reason the replacement failed.
	// Preserve request values but give recovery its own short deadline so a
	// cancelled caller cannot prevent restoration of the previous allowlist.
	// Re-read state instead of trusting mutation responses: an HTTP request can
	// be applied server-side even when the client only observes an error.
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	current, err := s.List(rollbackCtx)
	if err != nil {
		return fmt.Errorf("%w; rollback failed: inspect current allowlist: %v", cause, err)
	}
	currentSet := stringSet(current)
	var attemptErrors []string
	for _, ip := range plan.ExistingIPs {
		if _, present := currentSet[ip]; present {
			continue
		}
		if err := s.client.AddOpenAPIAllowedIP(rollbackCtx, ip); err != nil {
			attemptErrors = append(attemptErrors, fmt.Sprintf("restore %s: %v", ip, err))
		}
	}

	restored, err := s.List(rollbackCtx)
	if err != nil {
		return fmt.Errorf("%w; rollback failed: verify previous allowlist: %v", cause, err)
	}
	restoredSet := stringSet(restored)
	var invalid []string
	for _, ip := range plan.ExistingIPs {
		if _, present := restoredSet[ip]; !present {
			invalid = append(invalid, "missing "+ip)
		}
	}
	if len(invalid) > 0 {
		details := append(invalid, attemptErrors...)
		return fmt.Errorf("%w; rollback failed for %s; retained current IP", cause, strings.Join(details, ", "))
	}

	if plan.AddIP != "" {
		if _, present := restoredSet[plan.AddIP]; present {
			if err := s.client.DeleteOpenAPIAllowedIP(rollbackCtx, plan.AddIP); err != nil {
				attemptErrors = append(attemptErrors, fmt.Sprintf("remove %s: %v", plan.AddIP, err))
			}
		}
	}

	final, err := s.List(rollbackCtx)
	if err != nil {
		return fmt.Errorf("%w; rollback failed: verify current allowlist: %v", cause, err)
	}
	finalSet := stringSet(final)
	invalid = invalid[:0]
	for _, ip := range plan.ExistingIPs {
		if _, present := finalSet[ip]; !present {
			invalid = append(invalid, "missing "+ip)
		}
	}
	if plan.AddIP != "" {
		if _, present := finalSet[plan.AddIP]; present {
			invalid = append(invalid, "unexpected "+plan.AddIP)
		}
	}
	if len(invalid) > 0 {
		details := append(invalid, attemptErrors...)
		return fmt.Errorf("%w; rollback failed for %s", cause, strings.Join(details, ", "))
	}
	return fmt.Errorf("%w; restored previous allowlist", cause)
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func canonicalPlan(plan Plan) (string, error) {
	payload := struct {
		Kind        string   `json:"kind"`
		CurrentIP   string   `json:"current_ip"`
		ExistingIPs []string `json:"existing_ips"`
		DeleteIPs   []string `json:"delete_ips"`
		AddIP       string   `json:"add_ip,omitempty"`
	}{plan.Kind, plan.CurrentIP, plan.ExistingIPs, plan.DeleteIPs, plan.AddIP}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode IP replacement plan: %w", err)
	}
	return string(data), nil
}

func normalizeIPs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		ip, err := normalizeIP(value)
		if err != nil {
			return nil, fmt.Errorf("invalid IP in current allowlist: %w", err)
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	sort.Strings(out)
	return out, nil
}

func normalizeIP(value string) (string, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("invalid IP address %q", value)
	}
	return addr.Unmap().String(), nil
}

// HTTPResolver resolves the current public IP through a bounded HTTPS request.
type HTTPResolver struct {
	client *http.Client
	url    string
}

func NewHTTPResolver(client *http.Client, endpoint string) *HTTPResolver {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultPublicIPURL
	}
	return &HTTPResolver{client: client, url: endpoint}
}

func (r *HTTPResolver) CurrentPublicIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("public IP service returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	return normalizeIP(string(data))
}
