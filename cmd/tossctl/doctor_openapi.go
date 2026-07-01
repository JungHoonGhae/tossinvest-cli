package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
)

// doctorOpenAPIInputs holds all gathered pieces for the one-line Open API
// summary in `tossctl doctor`. All fields are optional — doctorOpenAPISummary
// degrades gracefully when pieces are missing.
type doctorOpenAPIInputs struct {
	credsPresent bool
	probeOK      bool
	probeErrKind string
	hasKeyInfo   bool       // true when WTS key-meta call succeeded
	keyActive    bool       // meaningful only when hasKeyInfo=true
	expiresAt    *time.Time // nil when unknown or no WTS data
	probeCallErr string     // non-empty when the probe call itself failed
}

// doctorOpenAPISummary is a pure function returning the one-line Open API
// summary shown in `tossctl doctor`. No I/O; safe to unit-test with fake inputs.
//
// Cases:
//   - credsPresent=false         → non-pushy hint toward `tossctl openapi login`
//   - probeCallErr != ""         → probe failed (timeout etc.), pointer to status
//   - probeOK=false              → actionable error with ❌ + remediation hint
//   - probeOK=true               → "연결 정상 · [활성 ·] [만료 D-NN ⚠ ·] 현재 IP 허용됨"
func doctorOpenAPISummary(in doctorOpenAPIInputs) string {
	const prefix = "Open API: "

	if !in.credsPresent {
		return prefix + "미설정 (키를 연결하면 일부 기능이 더 안정적으로 동작 — tossctl openapi login)"
	}

	// Probe call itself failed (context timeout, cancelled, etc.).
	if in.probeCallErr != "" {
		return prefix + "점검 실패 (" + in.probeCallErr + ") — 상세: tossctl openapi status"
	}

	// Probe returned an error classification — show actionable message.
	if !in.probeOK {
		switch in.probeErrKind {
		case "ip_not_allowed":
			return prefix + "❌ IP 미허용 — 토스 설정에 현재 IP 추가 (tossctl openapi status)"
		case "auth":
			return prefix + "❌ 인증 실패 — API 키/시크릿 확인 (tossctl openapi status)"
		case "rate_limited":
			return prefix + "❌ 호출 한도 초과 — 잠시 후 재시도 (tossctl openapi status)"
		case "server_error":
			return prefix + "❌ 서버 오류 (tossctl openapi status)"
		case "transport_error":
			return prefix + "❌ 네트워크 오류 — 인터넷 연결 확인 (tossctl openapi status)"
		default:
			return prefix + "❌ 연결 실패 — tossctl openapi status"
		}
	}

	// Probe succeeded — build a positive summary with available key metadata.
	var parts []string
	parts = append(parts, "연결 정상")

	if in.hasKeyInfo {
		if in.keyActive {
			parts = append(parts, "활성")
		} else {
			parts = append(parts, "비활성")
		}
		if in.expiresAt != nil {
			remaining := time.Until(*in.expiresAt)
			days := int(remaining.Hours() / 24)
			if days >= 0 && days <= 30 {
				parts = append(parts, fmt.Sprintf("만료 임박 D-%d ⚠", days))
			}
		}
	}

	parts = append(parts, "현재 IP 허용됨")

	return prefix + strings.Join(parts, " · ")
}

// computeDoctorOpenAPISummary gathers Open API status with short timeouts and
// returns the one-line summary for `tossctl doctor`. All errors are absorbed;
// this function must never cause doctor to fail or return a non-nil error.
func computeDoctorOpenAPISummary(ctx context.Context, opts *rootOptions, app *appContext) string {
	credFile, tokenFile, err := resolveOpenAPIPaths(opts)
	if err != nil {
		return "Open API: 경로 조회 실패 (tossctl openapi status)"
	}

	creds, err := official.LoadCredentials(os.Getenv, credFile)
	if err != nil || creds == nil {
		return doctorOpenAPISummary(doctorOpenAPIInputs{credsPresent: false})
	}

	in := doctorOpenAPIInputs{credsPresent: true}

	// Optional: try WTS key metadata (requires active session — graceful on error).
	if app != nil {
		wtsCtx, wtsCancel := context.WithTimeout(ctx, 3*time.Second)
		defer wtsCancel()
		if info, infoErr := app.client.OpenAPIClientInfo(wtsCtx); infoErr == nil {
			in.hasKeyInfo = true
			in.keyActive = info.Active
			if !info.ExpiresAt.IsZero() {
				t := info.ExpiresAt
				in.expiresAt = &t
			}
		}
		// WTS unavailable (no session, timeout) → skip key info, continue
	}

	// Live probe with short timeout — ground truth for connection + IP status.
	probeCtx, probeCancel := context.WithTimeout(ctx, 5*time.Second)
	defer probeCancel()

	probe, err := validateOpenAPICredentials(probeCtx, *creds, tokenFile)
	if err != nil {
		// validateOpenAPICredentials always returns nil error; defensive path.
		in.probeCallErr = err.Error()
	} else {
		in.probeOK = probe.OK
		in.probeErrKind = probe.ErrorKind
	}

	return doctorOpenAPISummary(in)
}
