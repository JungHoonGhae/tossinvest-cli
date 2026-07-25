package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// 합성 더미. 실제 계좌 데이터는 쓰지 않는다.
func sampleDetail() domain.AccountDetail {
	restricted := false
	diff := true
	return domain.AccountDetail{
		Number:       "137-01-000930",
		Name:         "홍길동",
		Status:       "00",
		OpenedAt:     "2021-03-05",
		LastTradedAt: "2026-07-24",
		Withdrawable: &domain.WithdrawableByDay{Day0: 1000, Day1: 2000, Day2: 3000},
		WithdrawalLimits: &domain.WithdrawalLimits{
			PerTransaction: 10000000, PerDay: 100000000, UsedToday: 0,
		},
		FullWithdrawalOn:   "2026-07-28",
		TransferRestricted: &restricted,
		MarginKR:           &domain.MarginStatus{Receivable: false},
		MarginUS:           &domain.MarginStatus{Receivable: false},
		DifferentialMargin: &diff,
		FetchedAt:          time.Now(),
	}
}

// 이 출력은 이슈·채팅에 붙여넣어진다. 마스킹이 기본이라는 계약을 잠근다 —
// 리팩터가 이걸 떨어뜨리면 실명과 계좌번호가 그대로 샌다.
func TestAccountDetailMasksByDefault(t *testing.T) {
	d := sampleDetail()
	for _, format := range []Format{FormatTable, FormatJSON} {
		var buf bytes.Buffer
		if err := WriteAccountDetail(&buf, format, d, false); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if strings.Contains(out, d.Number) {
			t.Errorf("[%s] 계좌번호 전체가 노출됐다", format)
		}
		if strings.Contains(out, d.Name) {
			t.Errorf("[%s] 예금주 실명이 노출됐다 — accountName 은 실명이다", format)
		}
		if !strings.Contains(out, "*") {
			t.Errorf("[%s] 마스킹 흔적이 없다: %s", format, out)
		}
	}
}

// --full 은 명시적 해제여야 한다 (그리고 그때만).
func TestAccountDetailFullRevealsBoth(t *testing.T) {
	d := sampleDetail()
	var buf bytes.Buffer
	if err := WriteAccountDetail(&buf, FormatTable, d, true); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, d.Number) || !strings.Contains(out, d.Name) {
		t.Errorf("--full 인데 전체가 안 나온다: %s", out)
	}
}

// 포맷을 바꾸는 것이 마스킹 우회로가 되면 안 된다.
func TestAccountDetailJSONHonoursMasking(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAccountDetail(&buf, FormatJSON, sampleDetail(), false); err != nil {
		t.Fatal(err)
	}
	var got domain.AccountDetail
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Number, "*") || !strings.Contains(got.Name, "*") {
		t.Errorf("JSON 이 마스킹을 우회했다: number=%q name=%q", got.Number, got.Name)
	}
}

// 의미를 모르는 상태 코드는 사람이 읽는 출력에 넣지 않는다(JSON 에는 남긴다).
func TestAccountDetailHidesOpaqueStatusFromTable(t *testing.T) {
	d := sampleDetail() // Status: "00"
	var buf bytes.Buffer
	if err := WriteAccountDetail(&buf, FormatTable, d, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "00)") {
		t.Error("의미 모르는 status 코드를 사람 읽는 출력에 넣었다")
	}
	var j bytes.Buffer
	if err := WriteAccountDetail(&j, FormatJSON, d, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(j.String(), `"status"`) {
		t.Error("JSON 에는 status 가 남아야 한다")
	}
}

// 못 불러온 항목은 조용히 사라지지 않고 경고로 보여야 한다.
func TestAccountDetailShowsWarnings(t *testing.T) {
	d := sampleDetail()
	d.MarginKR, d.MarginUS, d.DifferentialMargin = nil, nil, nil
	d.Warnings = []string{"미수거래 상태: 서버 오류"}
	var buf bytes.Buffer
	if err := WriteAccountDetail(&buf, FormatTable, d, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "미수거래 상태") {
		t.Errorf("경고가 출력되지 않았다: %s", buf.String())
	}
}
