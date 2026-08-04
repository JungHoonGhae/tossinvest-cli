package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

var (
	ErrExtensionTimeout       = errors.New("auth extend: phone approval timed out")
	ErrExtensionRejected      = errors.New("auth extend: phone approval was rejected or canceled")
	ErrExtensionNotConfigured = errors.New("auth extend: ExtensionRunner not configured")
)

// ExtensionTimeoutError wraps ErrExtensionTimeout with the elapsed wait time
// so callers can format user messages without parsing the error string.
// Replaces v0.4.5's `fmt.Errorf("%w (waited %s)", …)` + caller-side regex.
type ExtensionTimeoutError struct {
	Elapsed time.Duration
}

func (e *ExtensionTimeoutError) Error() string {
	return fmt.Sprintf("%s (waited %s)", ErrExtensionTimeout.Error(), e.Elapsed.Round(time.Second))
}

func (e *ExtensionTimeoutError) Unwrap() error { return ErrExtensionTimeout }

type ExtendResult struct {
	UUID            string
	ServerExpiresAt time.Time
	Elapsed         time.Duration
}

// ServerExpiryIn reports how long the server-side session has left.
//
// It asks the server instead of reading the stored copy: `ServerExpiresAt` on
// disk is only refreshed by `auth status` and by a successful Extend, so a
// scheduled run deciding "is it time to extend?" from the stored value would be
// deciding on a stale number — and getting that wrong means either a needless
// phone prompt or a session that quietly dies.
func (s *Service) ServerExpiryIn(ctx context.Context) (time.Duration, error) {
	if s.extensionRunner == nil {
		return 0, ErrExtensionNotConfigured
	}
	expiredAt, err := s.extensionRunner.GetServerExpiredAt(ctx)
	if err != nil {
		return 0, fmt.Errorf("read current expiry: %w", err)
	}
	return time.Until(expiredAt), nil
}

func (s *Service) Extend(ctx context.Context, timeout time.Duration) (*ExtendResult, error) {
	if s.extensionRunner == nil {
		return nil, ErrExtensionNotConfigured
	}

	sess, err := s.store.Load(ctx)
	if err != nil {
		return nil, err
	}
	if sess == nil || len(sess.Cookies) == 0 {
		return nil, session.ErrNoSession
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	uuid, err := s.extensionRunner.RequestExtension(ctx)
	if err != nil {
		return nil, fmt.Errorf("request extension: %w", err)
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		status, err := s.extensionRunner.GetExtensionStatus(ctx, uuid)
		if err == nil {
			if status.Approved() {
				break
			}
			if status.Rejected() {
				return nil, ErrExtensionRejected
			}
		} else if !isContextErr(err) {
			return nil, fmt.Errorf("poll extension status: %w", err)
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, &ExtensionTimeoutError{Elapsed: time.Since(start)}
			}
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}

	if err := s.extensionRunner.FinalizeExtension(ctx, uuid); err != nil {
		return nil, fmt.Errorf("finalize extension: %w", err)
	}

	expiredAt, err := s.extensionRunner.GetServerExpiredAt(ctx)
	if err != nil {
		return nil, fmt.Errorf("read new expiry: %w", err)
	}
	sess.ServerExpiresAt = &expiredAt
	if err := s.store.Save(ctx, sess); err != nil {
		return nil, fmt.Errorf("persist new expiry: %w", err)
	}

	return &ExtendResult{
		UUID:            uuid,
		ServerExpiresAt: expiredAt,
		Elapsed:         time.Since(start),
	}, nil
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
