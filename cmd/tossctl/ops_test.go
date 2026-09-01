package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// `tossctl ops` is the terminal view of the same operation registry the MCP
// server exposes. Its output is a machine surface: fixed JSON, no masking, no
// table. These tests pin that contract — a table renderer or a masked field
// creeping in here would silently break the agents that parse it.

func runOps(t *testing.T, args ...string) (string, error) {
	t.Helper()
	opts := &rootOptions{configDir: t.TempDir(), outputFormat: "table"}
	cmd := newOpsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

type opsListPayload struct {
	Count      int `json:"count"`
	Operations []struct {
		ID       string   `json:"id"`
		Method   string   `json:"method"`
		Path     string   `json:"path"`
		Category string   `json:"category"`
		Summary  string   `json:"summary"`
		Backend  string   `json:"backend"`
		Write    bool     `json:"write"`
		Required []string `json:"required"`
	} `json:"operations"`
}

// list 는 인증 없이 동작해야 한다 — 카탈로그는 로컬 선언이지 API 호출이 아니다.
func TestOpsListEmitsJSONCatalogWithoutAuth(t *testing.T) {
	out, err := runOps(t, "list")
	if err != nil {
		t.Fatalf("ops list 가 실패했다: %v (%s)", err, out)
	}
	var got opsListPayload
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON 이 아니다: %v\n%s", err, out)
	}
	if got.Count == 0 || got.Count != len(got.Operations) {
		t.Fatalf("count 가 목록과 맞지 않는다: count=%d len=%d", got.Count, len(got.Operations))
	}
	// 웹 세션 전용 오퍼레이션도 목록에는 보여야 한다. 안 보이면 에이전트가
	// 로그인하면 뭘 쓸 수 있는지 알 방법이 없다.
	var sawWTS bool
	for _, o := range got.Operations {
		if o.Backend == "wts" {
			sawWTS = true
			break
		}
	}
	if !sawWTS {
		t.Error("wts 백엔드 오퍼레이션이 목록에 없다")
	}
}

func TestOpsListQueryFilters(t *testing.T) {
	out, err := runOps(t, "list", "--query", "order")
	if err != nil {
		t.Fatalf("ops list --query 가 실패했다: %v (%s)", err, out)
	}
	var got opsListPayload
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON 이 아니다: %v\n%s", err, out)
	}
	if got.Count == 0 {
		t.Fatal("order 로 걸리는 오퍼레이션이 하나도 없다")
	}
	for _, o := range got.Operations {
		hay := strings.ToLower(o.ID + " " + o.Path + " " + o.Category + " " + o.Summary)
		if !strings.Contains(hay, "order") {
			t.Errorf("질의와 무관한 오퍼레이션이 섞였다: %s", o.ID)
		}
	}
	if all, _ := runOps(t, "list"); len(all) <= len(out) {
		t.Error("--query 가 목록을 좁히지 않았다")
	}
}

// describe 는 파라미터 스키마를 준다 — call 을 조립하려면 이게 필요하다.
func TestOpsDescribeReturnsParams(t *testing.T) {
	out, err := runOps(t, "describe", "place_order")
	if err != nil {
		t.Fatalf("ops describe 가 실패했다: %v (%s)", err, out)
	}
	var got struct {
		ID     string `json:"id"`
		Write  bool   `json:"write"`
		Params []struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Required bool   `json:"required"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON 이 아니다: %v\n%s", err, out)
	}
	if got.ID != "place_order" {
		t.Errorf("id: want place_order, got %q", got.ID)
	}
	if !got.Write {
		t.Error("place_order 는 write 로 표시돼야 한다")
	}
	if len(got.Params) == 0 {
		t.Error("params 가 비었다 — 이러면 call 을 조립할 수 없다")
	}
}

// 오타 난 id 는 조용히 빈 결과를 주면 안 된다. 어디서 고쳐야 하는지 알려줘야 한다.
func TestOpsDescribeUnknownOperationErrors(t *testing.T) {
	_, err := runOps(t, "describe", "no_such_operation")
	if err == nil {
		t.Fatal("모르는 오퍼레이션인데 에러가 없다")
	}
	if !strings.Contains(err.Error(), "ops list") {
		t.Errorf("에러가 다음 수단을 안내하지 않는다: %v", err)
	}
}

func TestOpsCallUnknownOperationErrors(t *testing.T) {
	_, err := runOps(t, "call", "no_such_operation")
	if err == nil {
		t.Fatal("모르는 오퍼레이션인데 에러가 없다")
	}
}

// --params 가 JSON 이 아니면 그 자리에서 말해줘야 한다. 인증까지 갔다가
// 엉뚱한 에러로 죽으면 무엇이 틀렸는지 알 수 없다.
func TestOpsCallRejectsMalformedParams(t *testing.T) {
	_, err := runOps(t, "call", "accounts", "--params", "{not json")
	if err == nil {
		t.Fatal("깨진 JSON 인데 에러가 없다")
	}
	if !strings.Contains(err.Error(), "--params") {
		t.Errorf("에러가 어느 플래그 문제인지 안 알려준다: %v", err)
	}
}

func TestOpsCallRejectsNullParams(t *testing.T) {
	_, err := runOps(t, "call", "auth_status", "--params", "null")
	if err == nil || !strings.Contains(err.Error(), "JSON object") {
		t.Fatalf("explicit null params must not satisfy the object contract, got %v", err)
	}
}

func TestOpsCallRejectsExplicitEmptyParams(t *testing.T) {
	_, err := runOps(t, "call", "auth_status", "--params", "")
	if err == nil || !strings.Contains(err.Error(), "JSON object") {
		t.Fatalf("explicit empty params must not be treated as an omitted flag, got %v", err)
	}
}

func TestOpsCallPreservesLargeJSONIntegerForValidation(t *testing.T) {
	_, err := runOps(t, "call", "place_order", "--params", `{"symbol":"AAPL","side":"buy","price":9007199254740993}`)
	if err == nil || !strings.Contains(err.Error(), "precision loss") {
		t.Fatalf("large JSON integer must be rejected without rounding, got %v", err)
	}
}

// 실패했을 때 stdout 에 반쪽짜리 JSON 을 남기면 안 된다. 에이전트는 stdout 을
// 통째로 파싱하므로, 성공했을 때만 JSON 이어야 파싱 성공 = 호출 성공이 된다.
func TestOpsFailureLeavesStdoutEmpty(t *testing.T) {
	opts := &rootOptions{configDir: t.TempDir(), outputFormat: "table"}
	cmd := newOpsCmd(opts)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"describe", "no_such_operation"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("모르는 오퍼레이션인데 에러가 없다")
	}
	if stdout.Len() != 0 {
		t.Errorf("실패했는데 stdout 에 뭔가 나갔다: %q", stdout.String())
	}
}

// 이 표면은 계좌번호·실명을 그대로 뱉는다. 도움말이 그걸 말하지 않으면
// 사람이 출력을 이슈에 붙여넣는다.
func TestOpsCallHelpWarnsOutputIsUnmasked(t *testing.T) {
	var cmd *cobra.Command
	for _, c := range newOpsCmd(&rootOptions{}).Commands() {
		if c.Name() == "call" {
			cmd = c
		}
	}
	if cmd == nil {
		t.Fatal("call 서브커맨드가 없다")
	}
	help := strings.ToLower(cmd.Long)
	for _, want := range []string{"raw", "mask"} {
		if !strings.Contains(help, want) {
			t.Errorf("call 의 Long 에 %q 경고가 없다: %q", want, cmd.Long)
		}
	}
}
