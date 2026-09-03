package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/monitor"
)

// 만료된 세션 하나가 계좌 스코프 probe 를 한꺼번에 401 로 떨어뜨린다. 그때
// "✗ ... status=401" 만 11줄 찍으면, 읽는 사람은 토스가 API 11개를 깬 줄 안다.
// 원인이 하나라는 걸 말해줘야 엉뚱한 곳을 파지 않는다.
//
// `internal/client` 의 401 처리(errors.go)는 이 경로를 안 탄다 — monitor 는
// 타입 있는 클라이언트가 아니라 raw probe 라서, 여기서 따로 짚어줘야 한다.

func failing(name string, status int) monitor.Result {
	return monitor.Result{
		Probe:  monitor.Probe{Name: name},
		Status: status,
		Detail: "status " + itoa(status) + " (want 200)",
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestMonitorHintsAtSessionWhenAuthFailuresDominate(t *testing.T) {
	results := []monitor.Result{
		{Probe: monitor.Probe{Name: "market-index"}, OK: true, Status: 200},
		failing("account-list", 401),
		failing("pending-orders", 401),
		failing("watchlist-groups", 403),
	}

	var stdout, stderr bytes.Buffer
	printResults(&stdout, &stderr, results, false)

	all := stdout.String() + stderr.String()
	if !strings.Contains(all, "auth login") {
		t.Errorf("401/403 이 여럿인데 세션을 짚어주지 않는다:\n%s", all)
	}
}

// 인증과 무관한 실패에까지 세션 얘기를 붙이면, 진짜 계약 변경을 세션 탓으로
// 오진하게 만든다. 그쪽이 더 비싸다.
func TestMonitorDoesNotBlameSessionForNonAuthFailures(t *testing.T) {
	results := []monitor.Result{
		{Probe: monitor.Probe{Name: "market-index"}, OK: true, Status: 200},
		{Probe: monitor.Probe{Name: "portfolio-positions"}, Status: 200,
			Detail: "section[0].data.products is not an array"},
		failing("quote-trades", 500),
	}

	var stdout, stderr bytes.Buffer
	printResults(&stdout, &stderr, results, false)

	all := stdout.String() + stderr.String()
	if strings.Contains(all, "auth login") {
		t.Errorf("인증 실패가 없는데 세션을 탓한다:\n%s", all)
	}
}

// --quiet 는 실패만 보겠다는 뜻이지, 원인 힌트를 버리겠다는 뜻이 아니다.
func TestMonitorSessionHintSurvivesQuiet(t *testing.T) {
	results := []monitor.Result{failing("account-list", 401)}

	var stdout, stderr bytes.Buffer
	printResults(&stdout, &stderr, results, true)

	if !strings.Contains(stdout.String()+stderr.String(), "auth login") {
		t.Error("--quiet 에서 세션 힌트가 사라졌다")
	}
}

func TestMonitorReportsInapplicableStateDependentProbeAsSkipped(t *testing.T) {
	results := []monitor.Result{
		{Probe: monitor.Probe{Name: "watchlist-groups"}, OK: true, Status: 200},
		{Probe: monitor.Probe{Name: "watchlist-group"}, Skipped: true, Detail: "not applicable: account has no watchlist folders"},
	}

	var stdout, stderr bytes.Buffer
	printResults(&stdout, &stderr, results, false)
	if stderr.Len() != 0 {
		t.Fatalf("skipped probe was reported as failure: %s", stderr.String())
	}
	for _, want := range []string{"watchlist-group", "1 passed, 0 failed, 1 skipped"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output missing %q: %s", want, stdout.String())
		}
	}
}
