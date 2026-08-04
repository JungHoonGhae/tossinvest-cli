package auth

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

type fakeExtensionRunner struct {
	requestErr     error
	statusSequence []client.ExtensionStatus
	statusErr      error
	finalizeErr    error
	expiredAt      time.Time
	expiredErr     error
	requestCalls   atomic.Int32
	statusCalls    atomic.Int32
	finalizeCalls  atomic.Int32
	expiredAtCalls atomic.Int32
}

func (r *fakeExtensionRunner) RequestExtension(context.Context) (string, error) {
	r.requestCalls.Add(1)
	return "uuid-1", r.requestErr
}

func (r *fakeExtensionRunner) GetExtensionStatus(context.Context, string) (client.ExtensionStatus, error) {
	idx := int(r.statusCalls.Add(1)) - 1
	if r.statusErr != nil {
		return client.ExtensionStatus{}, r.statusErr
	}
	if idx >= len(r.statusSequence) {
		return r.statusSequence[len(r.statusSequence)-1], nil
	}
	return r.statusSequence[idx], nil
}

func (r *fakeExtensionRunner) FinalizeExtension(context.Context, string) error {
	r.finalizeCalls.Add(1)
	return r.finalizeErr
}

func (r *fakeExtensionRunner) GetServerExpiredAt(context.Context) (time.Time, error) {
	r.expiredAtCalls.Add(1)
	return r.expiredAt, r.expiredErr
}

func newExtendService(t *testing.T, runner ExtensionRunner) (*Service, string) {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "session.json")
	if err := session.NewFileStore(path).Save(context.Background(), &session.Session{
		Provider:    "playwright-storage-state",
		Cookies:     map[string]string{"SESSION": "s"},
		Headers:     map[string]string{"X-XSRF-TOKEN": "x"},
		RetrievedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	svc := NewService(
		session.NewFileStore(path),
		path,
		Options{ExtensionRunner: runner, PollInterval: time.Millisecond},
	)
	return svc, path
}

func TestExtendApprovesAfterPolling(t *testing.T) {
	t.Parallel()

	want := time.Date(2026, 5, 13, 7, 3, 20, 0, time.FixedZone("KST", 9*3600))
	runner := &fakeExtensionRunner{
		statusSequence: []client.ExtensionStatus{
			{Status: "REQUESTED"},
			{Status: "REQUESTED"},
			{Status: "COMPLETED"},
		},
		expiredAt: want,
	}
	svc, path := newExtendService(t, runner)

	result, err := svc.Extend(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if result.UUID != "uuid-1" {
		t.Fatalf("UUID = %q", result.UUID)
	}
	if !result.ServerExpiresAt.Equal(want) {
		t.Fatalf("ServerExpiresAt = %s, want %s", result.ServerExpiresAt, want)
	}
	if runner.statusCalls.Load() != 3 {
		t.Fatalf("expected 3 status polls, got %d", runner.statusCalls.Load())
	}
	if runner.finalizeCalls.Load() != 1 {
		t.Fatalf("expected 1 finalize call, got %d", runner.finalizeCalls.Load())
	}

	stored, err := session.NewFileStore(path).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if stored.ServerExpiresAt == nil || !stored.ServerExpiresAt.Equal(want) {
		t.Fatalf("ServerExpiresAt not persisted")
	}
}

func TestExtendReturnsRejectionEarly(t *testing.T) {
	t.Parallel()

	runner := &fakeExtensionRunner{
		statusSequence: []client.ExtensionStatus{
			{Status: "REQUESTED"},
			{Status: "EXPIRED"},
		},
	}
	svc, _ := newExtendService(t, runner)

	_, err := svc.Extend(context.Background(), 5*time.Second)
	if !errors.Is(err, ErrExtensionRejected) {
		t.Fatalf("expected ErrExtensionRejected, got %v", err)
	}
	if runner.expiredAtCalls.Load() != 0 {
		t.Fatal("should not call GetServerExpiredAt on rejection")
	}
	if runner.finalizeCalls.Load() != 0 {
		t.Fatal("should not call FinalizeExtension on rejection")
	}
}

func TestExtendWrapsFinalizeError(t *testing.T) {
	t.Parallel()

	runner := &fakeExtensionRunner{
		statusSequence: []client.ExtensionStatus{{Status: "COMPLETED"}},
		finalizeErr:    errors.New("400 invalid-access"),
	}
	svc, _ := newExtendService(t, runner)

	_, err := svc.Extend(context.Background(), 5*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "finalize extension") {
		t.Fatalf("expected wrap prefix 'finalize extension', got %q", err.Error())
	}
	if runner.expiredAtCalls.Load() != 0 {
		t.Fatal("should not call GetServerExpiredAt when finalize fails")
	}
}

func TestExtendTimesOut(t *testing.T) {
	t.Parallel()

	runner := &fakeExtensionRunner{
		statusSequence: []client.ExtensionStatus{{Status: "REQUESTED"}},
	}
	svc, _ := newExtendService(t, runner)

	_, err := svc.Extend(context.Background(), 10*time.Millisecond)
	if !errors.Is(err, ErrExtensionTimeout) {
		t.Fatalf("expected ErrExtensionTimeout, got %v", err)
	}
	if !strings.Contains(err.Error(), "waited") {
		t.Fatalf("expected wrapped 'waited' detail, got %q", err.Error())
	}
}

func TestExtendRespectsContextCancel(t *testing.T) {
	t.Parallel()

	runner := &fakeExtensionRunner{
		statusSequence: []client.ExtensionStatus{{Status: "REQUESTED"}},
	}
	svc, _ := newExtendService(t, runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Extend(ctx, 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestExtendRequiresStoredSession(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "session.json")
	svc := NewService(
		session.NewFileStore(path),
		path,
		Options{ExtensionRunner: &fakeExtensionRunner{}, PollInterval: time.Millisecond},
	)

	_, err := svc.Extend(context.Background(), time.Second)
	if !errors.Is(err, session.ErrNoSession) {
		t.Fatalf("expected ErrNoSession, got %v", err)
	}
}

func TestExtendWrapsRequestError(t *testing.T) {
	t.Parallel()

	runner := &fakeExtensionRunner{requestErr: errors.New("dial tcp: connection refused")}
	svc, _ := newExtendService(t, runner)

	_, err := svc.Extend(context.Background(), 5*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "request extension") {
		t.Fatalf("expected wrap prefix 'request extension', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("expected underlying error to be retained, got %q", err.Error())
	}
}

func TestExtendWrapsPollingError(t *testing.T) {
	t.Parallel()

	runner := &fakeExtensionRunner{statusErr: errors.New("503 service unavailable")}
	svc, _ := newExtendService(t, runner)

	_, err := svc.Extend(context.Background(), 5*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "poll extension status") {
		t.Fatalf("expected wrap prefix 'poll extension status', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "503 service unavailable") {
		t.Fatalf("expected underlying error to be retained, got %q", err.Error())
	}
}

func TestExtendWrapsExpiryRefreshError(t *testing.T) {
	t.Parallel()

	runner := &fakeExtensionRunner{
		statusSequence: []client.ExtensionStatus{{Status: "COMPLETED"}},
		expiredErr:     errors.New("502 bad gateway"),
	}
	svc, _ := newExtendService(t, runner)

	_, err := svc.Extend(context.Background(), 5*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "read new expiry") {
		t.Fatalf("expected wrap prefix 'read new expiry', got %q", err.Error())
	}
}

func TestExtendRequiresExtensionRunner(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "session.json")
	if err := session.NewFileStore(path).Save(context.Background(), &session.Session{
		Provider:    "playwright-storage-state",
		Cookies:     map[string]string{"SESSION": "s"},
		RetrievedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	svc := NewService(
		session.NewFileStore(path),
		path,
		Options{PollInterval: time.Millisecond},
	)

	_, err := svc.Extend(context.Background(), time.Second)
	if !errors.Is(err, ErrExtensionNotConfigured) {
		t.Fatalf("expected ErrExtensionNotConfigured, got %v", err)
	}
}

func TestServerExpiryInReadsServerNotDisk(t *testing.T) {
	t.Parallel()

	runner := &fakeExtensionRunner{expiredAt: time.Now().Add(48 * time.Hour)}
	svc, _ := newExtendService(t, runner)

	got, err := svc.ServerExpiryIn(context.Background())
	if err != nil {
		t.Fatalf("ServerExpiryIn: %v", err)
	}
	// The stored session has no ServerExpiresAt at all, so a disk-based answer
	// would be zero — this asserts we asked the server.
	if got < 47*time.Hour || got > 48*time.Hour {
		t.Fatalf("remaining = %s, want ~48h", got)
	}
	if runner.expiredAtCalls.Load() != 1 {
		t.Fatalf("GetServerExpiredAt calls = %d, want 1", runner.expiredAtCalls.Load())
	}
	// Must not have poked the phone.
	if runner.requestCalls.Load() != 0 {
		t.Fatalf("RequestExtension calls = %d, want 0", runner.requestCalls.Load())
	}
}

func TestServerExpiryInPropagatesError(t *testing.T) {
	t.Parallel()

	runner := &fakeExtensionRunner{expiredErr: errors.New("boom")}
	svc, _ := newExtendService(t, runner)

	if _, err := svc.ServerExpiryIn(context.Background()); err == nil {
		t.Fatal("ServerExpiryIn: want error, got nil")
	}
}

func TestServerExpiryInWithoutRunner(t *testing.T) {
	t.Parallel()

	svc, _ := newExtendService(t, nil)
	svc.extensionRunner = nil

	if _, err := svc.ServerExpiryIn(context.Background()); !errors.Is(err, ErrExtensionNotConfigured) {
		t.Fatalf("err = %v, want ErrExtensionNotConfigured", err)
	}
}
