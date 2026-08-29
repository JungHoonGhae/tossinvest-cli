package ops

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 기능이 한쪽 표면에만 얹히는 일이 반복됐다 — 이슈 #136(공식 API 가 MCP 에만),
// #111(조건주문 게이트가 CLI 에만). ops 엔트리 추가는 구조체 리터럴 하나로
// 끝나지만 hybrid 라우팅 + 타입 커맨드는 일이 많아서, 레지스트리가 CLI 를
// 앞지르는 쪽으로 계속 기운다.
//
// 이 테스트는 통합을 강제하지 않는다 — ADR-0001 이 cmd 를 ops 로 태우지 않기로
// 결정했고 그건 유효하다. 여기서 막는 것은 **말없이 벌어지는 것**이다. 새
// official.Client 메서드가 ops 에만 노출되면 실패하고, 의도한 것이라면 아래
// 목록에 이유를 적어야 통과한다.
//
// 커버리지를 "구현했는가" 로만 세면 이 격차가 안 보인다. "CLI 에서 쓸 수
// 있는가" 로도 세야 한다.
var opsOnlyByDesign = map[string]string{
	"Orders": "공식 status enum 매핑이 미확정이다. 라우팅하면 체결된 주문이 취소로 " +
		"보일 수 있다 — 백엔드를 사용자가 고르게 하는 것보다 나쁘다. 트리거: " +
		"같은 달 완료 주문이 생겨 WTS/공식 상태값을 나란히 관측할 때.",
	"OrderByID": "Orders 와 같은 이유 (status enum 미확정).",
	"Stocks":    "종목 메타 일괄 조회(최대 200). `market stocks`(ListStocks, 상장 유니버스)와 다른 엔드포인트이고, 단건은 `quote` 가 이미 덮는다.",
}

var officialMethodRE = regexp.MustCompile(`func \(c \*Client\) ([A-Z]\w+)\(`)

func readAllGo(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		b.Write(data)
	}
	return b.String()
}

func TestOfficialMethodsReachableFromCLI(t *testing.T) {
	root := filepath.Join("..", "..")
	official := readAllGo(t, filepath.Join(root, "internal", "official"))
	// CLI 도달 경로 셋: hybrid 라우팅 · cmd 직접 호출 · trading 브로커 경유.
	reach := readAllGo(t, filepath.Join(root, "internal", "hybrid")) +
		readAllGo(t, filepath.Join(root, "cmd", "tossctl")) +
		readAllGo(t, filepath.Join(root, "internal", "trading"))
	opsSrc := readAllGo(t, filepath.Join(root, "internal", "ops"))

	var drifted []string
	seen := map[string]bool{}
	for _, m := range officialMethodRE.FindAllStringSubmatch(official, -1) {
		name := m[1]
		if seen[name] || name == "BaseURL" {
			continue
		}
		seen[name] = true
		call := "." + name + "("
		if strings.Contains(reach, call) {
			continue // CLI 에서 도달한다
		}
		if !strings.Contains(opsSrc, call) {
			continue // 어디에도 노출 안 됨 — 이 테스트의 관심사가 아니다
		}
		if _, ok := opsOnlyByDesign[name]; !ok {
			drifted = append(drifted, name)
		}
	}
	sort.Strings(drifted)
	if len(drifted) > 0 {
		t.Errorf(`official.Client 메서드 %v 가 ops(MCP)에만 노출돼 있다.

CLI 커맨드를 붙이거나, 의도한 것이라면 opsOnlyByDesign 에 **이유와 함께** 등록할 것.
이유 없이 목록에 넣지 말 것 — 그러면 이 테스트가 통과만 하는 장식이 된다.`, drifted)
	}
}

// 예외 목록이 낡는 것도 드리프트다. CLI 커맨드가 붙었는데 목록에 남아 있으면
// 다음 사람이 "아직 못 쓴다" 고 잘못 읽는다.
func TestOpsOnlyExceptionsAreStillAccurate(t *testing.T) {
	root := filepath.Join("..", "..")
	reach := readAllGo(t, filepath.Join(root, "internal", "hybrid")) +
		readAllGo(t, filepath.Join(root, "cmd", "tossctl")) +
		readAllGo(t, filepath.Join(root, "internal", "trading"))

	for name, why := range opsOnlyByDesign {
		if strings.TrimSpace(why) == "" {
			t.Errorf("%s: 예외에는 이유가 있어야 한다", name)
		}
		if strings.Contains(reach, "."+name+"(") {
			t.Errorf("%s: 이제 CLI 에서 도달한다 — opsOnlyByDesign 에서 빼야 한다", name)
		}
	}
}
