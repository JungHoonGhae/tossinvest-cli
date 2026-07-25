# 한 번 선언 → CLI + MCP: 이미 있는가, 통증은 실재하는가

- 조사일: 2026-07-25
- 질문: "오퍼레이션을 한 번 선언하면 CLI 커맨드와 MCP 툴이 함께 나오는" 도구가 이미 존재하는가? 없다면 이중 선언의 통증은 실재하는가?
- 근거: 레포 소스·GitHub 이슈/PR·공식 문서만. 블로그 요약 제외.

## 결론 — **이미(대체로) 해결됨. 빈자리는 좁고, 게다가 좁아지는 중**

cobra 커맨드 트리를 걸어 MCP 툴로 자동 노출하는 Go 라이브러리는 **최소 4개** 존재하고
([ophis](https://github.com/njayp/ophis) 89★·2026-07-22 커밋,
[cobra-mcp](https://github.com/eat-pray-ai/cobra-mcp) 2★,
[mcp-cobra](https://github.com/PlusLemon/mcp-cobra) 7★,
[mcp-go-wrapper](https://github.com/aleksadvaisly/mcp-go-wrapper) 0★),
Python(`click-mcp`)·Node(`oclif-plugin-mcp-server`)에도 등가물이 있다. 통증 자체도 실재한다 —
spf13/cobra 이슈 [#2362](https://github.com/spf13/cobra/issues/2362)(2026-02-28)가
"cobra flag 와 MCP JSON schema 를 두 번 정의해야 한다"는 문제를 정면으로 서술했고,
flyctl·planetscale/cli·railwayapp/cli 세 개의 실제 CLI 레포에서 같은 기능이 CLI 와 MCP 에
각각 따로 선언된 것을 파일:줄로 확인했다.
**그런데도 아이디어는 죽는 쪽에 가깝다**: (1) 그 통증을 정확히 겨냥한 라이브러리가 이미 있고 스타가
89개에서 멈춰 있다 — 즉 통증은 실재하지만 **얕다**. (2) 자동 브리지의 알려진 한계(타입 소거·
subprocess 텍스트 왕복)는 우리 ADR 0001 이 이미 도달한 결론과 동일하며, ophis 메인테이너는
typed args 지원을 **명시적으로 포기**했다([ophis#22](https://github.com/njayp/ophis/issues/22),
2025-10-04). (3) 생태계는 반대로 가고 있다 — gh 와 playwright 팀은 MCP 를 붙이는 대신
**CLI 자체를 에이전트 표면으로** 미는 중이고, FastMCP 공식 문서는 기존 표면에서 자동 변환한 MCP 가
손으로 큐레이션한 MCP 보다 **성능이 나쁘다**고 경고한다.

즉 "한 번 선언 → 두 표면"은 새 제품이 들어설 빈자리가 아니라, **이미 두어 개가 점유하고 있고
수요가 커지지 않는 좁은 틈**이다.

---

## A. 이미 존재하는가 — 존재한다

### A-1. cobra → MCP 브리지 (Go)

| 레포 | ★ | 마지막 푸시 | 생성 | 실행 모델 |
|---|---|---|---|---|
| [njayp/ophis](https://github.com/njayp/ophis) | 89 | 2026-07-22 | 2025-06-09 | 커맨드 트리 재귀 순회 → flag 로 JSON schema 생성 → **자기 바이너리를 subprocess 로 실행**하고 stdout 캡처 |
| [eat-pray-ai/cobra-mcp](https://github.com/eat-pray-ai/cobra-mcp) | 2 | 2026-06-19 | 2026-02-28 | in-process. `*mcp.Server` + `mcp` 서브커맨드(stdio/HTTP) 를 한 번의 호출로 제공 |
| [PlusLemon/mcp-cobra](https://github.com/PlusLemon/mcp-cobra) | 7 | 2025-04-25 | 2025-04-23 | cobra 구조 분석 → 커맨드=툴, flag=파라미터 |
| [aleksadvaisly/mcp-go-wrapper](https://github.com/aleksadvaisly/mcp-go-wrapper) | 0 | 2026-06-07 | 2025-11-09 | annotation 기반 래핑 |

(수치: `gh api repos/<repo>`, 2026-07-25 조회)

ophis README 가 밝히는 동작: "Recursively walks your Cobra command tree" / "Creates JSON schemas
from command flags and arguments" / "Spawns your CLI as a subprocess and captures output".
문서화된 caveat 은 subprocess 환경변수/PATH 문제뿐이고, **출력 구조 보존에 대한 언급은 없다** —
구조화된 데이터가 텍스트로 평탄화된다.

### A-2. 이 접근의 알려진 한계 (1차 자료)

- [ophis#22 "Great idea but wondering how to overcome Cobra untyped-ness"](https://github.com/njayp/ophis/issues/22)
  (2025-09-24): 제기된 문제 — 글로벌 플래그가 MCP 스키마로 새어 나감, args 가 타입이 없음, args 문서화 불가.
  메인테이너 답변: "all args are typed as strings, and an unlimited amount are allowed…
  I put a modified `cobra.Command.Usage` in the args description, and let the binary itself report
  any error to AI." 이후 2025-10-04 에 typed args 를 **포기**: "After lots of thought, I won't be
  moving forward with typed args — messy to implement… significantly increases the config burden".
- [spf13/cobra#2362](https://github.com/spf13/cobra/issues/2362) (2026-02-28, OPEN, 댓글 11):
  "developers must define input parameters twice — once as Cobra flags and again as MCP JSON
  schemas". 빠진 조각으로 **enum 값**과 **numeric bounds** 를 지목. 제안: pflag `Annotations`
  관례 또는 `MarkFlagEnum` 같은 1급 API.
- 같은 스레드에서 두 구현자가 실행 모델 차이를 정리(OpenWaygate, 2026-02-28):
  ophis 는 `exec.Command(os.Executable(), args...)` — "Simple, universal, but lossy
  (structured data → text → text back to LLM)"; yutu/cobra-mcp 는 도메인 메서드를 직접 호출해
  구조를 보존하되 **커맨드마다 cobra flag 와 MCP 툴을 각각 등록**한다.
- [spf13/cobra#2363 "Mark Flag as Enum"](https://github.com/spf13/cobra/pull/2363): ophis
  메인테이너의 PoC PR. 2026-02-28 생성, **CLOSED / 미머지**. 즉 cobra 코어는 아직 MCP 를 위한
  메타데이터를 받아들이지 않았다.

### A-3. 다른 언어

- Python: [crowecawcaw/click-mcp](https://github.com/crowecawcaw/click-mcp) — 14★, 2026-07-20 푸시.
  "Turn click CLIs into MCP servers with one line of code". typer 전용 등가물은 **확인 못 함**
  (검색으로 유의미한 레포를 찾지 못함).
- Node: [npjonath/oclif-plugin-mcp-server](https://github.com/npjonath/oclif-plugin-mcp-server) —
  11★, 2025-10-25 푸시(정체). commander 전용 브리지는 **확인 못 함**.
- FastMCP 는 CLI→MCP 를 하지 **않는다**. 대신 API 표면에서 파생한다(A-4).

### A-4. 범용 어댑터 (OpenAPI → MCP)

- FastMCP([PrefectHQ/fastmcp](https://github.com/PrefectHQ/fastmcp), 26.8k★, 2026-07-24 푸시)는
  `FastMCP.from_openapi(...)` 를 제공한다. 단 [공식 문서](https://gofastmcp.com/integrations/openapi)가
  직접 경고한다: "LLMs achieve **significantly better performance** with well-designed and curated
  MCP servers than with auto-converted OpenAPI servers" — 자동 변환은 부트스트랩/프로토타입용이며
  프로덕션 권장이 아니라고 명시.
- [tadata-org/fastapi_mcp](https://github.com/tadata-org/fastapi_mcp) — 11.9k★ 이지만 마지막 푸시
  2025-11-24 (8개월 정체).
- OpenAPI→MCP 코드 생성기는 다수 존재(harsha-iiiv/openapi-mcp-generator, cnoe-io/openapi-mcp-codegen,
  vincent-pli/openapi-mcpserver-generator 등). 개별 성숙도는 **정밀 확인 못 함**.
- 상용: [speakeasy-api/speakeasy](https://github.com/speakeasy-api/speakeasy) (429★) 는 레포 설명에서
  하나의 OpenAPI 로부터 "SDKs, Terraform providers, **MCP servers and CLIs**" 를 생성한다고 표방한다 —
  "한 선언 → 여러 표면"의 상용 답안. 다만 speakeasy.com 의 MCP 문서 페이지는 404 라 **세부 확인 못 함**.

---

## B. 통증은 실재하는가 — 실재한다. 다만 각자 조용히 감수하고 있다

원래 CLI 였다가 MCP 를 나중에 붙인 레포를 열어 확인했다.

### B-1. 이중 선언이 확인된 레포

**superfly/flyctl (Go, cobra)** — MCP 툴을 CLI 와 완전히 별도 테이블로 재선언하고, 핸들러는
자기 CLI 를 다시 실행한다.
- `internal/command/mcp/server/apps.go:8` `var AppCommands = []FlyCommand{`,
  `:10` `ToolName: "fly-apps-create"`, `:80` `ToolName: "fly-apps-list"` — 툴 이름·설명·인자를
  손으로 다시 적고, `Builder` 가 `[]string{"apps", "list", ...}` 인자열을 만든다.
- 같은 기능의 CLI 선언은 `internal/command/apps/list.go:21` `func newList() *cobra.Command`,
  `:29` `short = "List applications."`.
- 규모: `internal/command/mcp/server/` 의 `ToolName` 수 — apps 6, machine 20, volumes 9, orgs 6,
  certs 5, ips 5, secrets 4, platform 3, logs 1, status 1 = **60개 툴**이 CLI 와 별도로 선언됨.

**planetscale/cli (Go, cobra)** — MCP 툴 8개를 별도 선언하고, 핸들러는 CLI 를 거치지 않고 API
클라이언트를 직접 호출한다(우리와 같은 in-process 방식).
- `internal/cmd/mcp/server.go:28` `mcp.NewTool("list_orgs", ...)`, `:34` `list_databases`,
  `:44` `list_branches`, `:76` `list_tables`, `:100` `get_schema`, `:125` `run_query`,
  `:150` `get_insights`.
- 핸들러: `internal/cmd/mcp/server_handlers.go:18` `HandleListOrgs`, `:52` `HandleListDatabases`,
  `:102` `HandleListBranches` …
- 대응 CLI 커맨드는 `internal/cmd/branch/`, `internal/cmd/database/` 에 별도로 존재.

**railwayapp/cli (Rust, clap)** — MCP 파라미터 구조체와 툴 핸들러를 별도 모듈로 두되,
**컨트롤러 계층은 공유**한다.
- `src/commands/mcp/tools/project.rs` — `impl RailwayMcp { pub(crate) async fn do_create_project(&self, params: CreateProjectParams) }`,
  `use crate::controllers::{github, project, regions}` 로 CLI 와 같은 도메인 계층 호출.
- 파라미터 선언은 `src/commands/mcp/params.rs` 에 clap 과 별도로 존재.
- CLI 커맨드는 `src/commands/*.rs`.
- → **우리 ADR 0001 과 사실상 동일한 구조**: 선언은 둘, 도메인은 하나.

**github (교차 레포 이중화)** — `cli/cli` 트리에 MCP 코드는 없고(`.github/mcp.json` 뿐),
MCP 는 완전히 별개 제품 [github/github-mcp-server](https://github.com/github/github-mcp-server)
(31.7k★)로 존재한다. 예: `pkg/github/repositories.go:345` `func ListBranches(...) inventory.ServerTool`,
`:349` `Name: "list_branches"`. 같은 조직이 같은 API 를 두 코드베이스에서 각각 선언 중.

### B-2. 이중 선언이 아닌 사례 (반례)

- **Azure/azure-dev (azd)**: `cli/azd/internal/mcp/tools/` 의 툴은 `azd_error_troubleshooting`,
  `azd_provision_common_error`, `azd_yaml_schema` — 프롬프트/지식 제공 툴이지 CLI 커맨드의 거울이
  아니다. MCP 를 CLI 미러링이 아니라 **다른 용도**로 쓴 예.
- **vercel/vercel**: `packages/cli/src/commands/mcp/` 는 원격 MCP 로의 프록시/인증 어댑터
  (`packages/connect/src/mcp/connect-auth-provider.ts`) 성격. CLI 커맨드를 툴로 재선언하지 않음.
- **mcp 경로 자체가 없는 CLI**(2026-07-25 트리 조회): docker/cli, digitalocean/doctl,
  tursodatabase/turso-cli, cloudflare/workers-sdk, temporalio/cli, ariga/atlas, argoproj/argo-cd,
  hashicorp/terraform, stripe/stripe-cli, supabase/cli, netlify/cli, grafana/k6 — **12개 중 0개**가
  자기 레포 안에 MCP 를 갖고 있지 않다(별도 레포로 내는 경우 포함).

### B-3. 이중 선언이 "문제로 인식"되고 있는가 — 부분적으로만

- **인식됨**: cobra#2362 는 정확히 "두 번 정의"를 문제로 명명했고, 댓글에서 ophis 메인테이너가
  "It's true. The gap seems hackable via flag annotations, but that approach introduces hidden
  complexity" 라고 동의했다. 실제 PoC PR(#2363)까지 나왔으나 미머지.
- **인식 안 됨**: flyctl 이슈에서 `mcp` 관련 이슈는 2건뿐이고 둘 다 설정/프록시 버그
  ([#4430](https://github.com/superfly/flyctl/issues/4430), [#4374](https://github.com/superfly/flyctl/issues/4374)),
  60개 툴 동기화에 대한 불평은 **없다**. planetscale/cli 도 동일.
- GitHub 전역 이슈 검색("MCP CLI in sync duplicate tools", "MCP tools CLI commands duplication",
  "single source of truth MCP CLI") 로는 유의미한 히트가 **거의 없었다**(1건, 무관 레포).
  → 통증은 코드에 남아 있지만 **불만으로 표출되지 않는다**. 60개 툴을 손으로 쓰는 비용을 팀들이
  그냥 감수하고 있다는 뜻이거나, 툴 목록이 CLI 전체가 아니라 **선별된 부분집합**이라 동기화 압력이
  낮다는 뜻이다(flyctl 60툴 vs CLI 전체 커맨드 수를 보면 후자 가능성이 크다 — 정밀 대조는 **확인 못 함**).

---

## C. 반대 신호 — 강하다

1. **gh 팀은 MCP 를 CLI 에 붙이지 않고, CLI 를 에이전트 표면으로 개선하는 쪽을 탐색 중.**
   [cli/cli#12522 "Explore: making `gh` better for agents"](https://github.com/cli/cli/issues/12522)
   (2026-01-22, OPEN, 댓글 19). 논의 축은 MCP 가 아니라 machine-readable 커맨드 스키마
   (babakks: "a machine readable data structure (e.g. in JSON) to expose available commands and
   flags… similar to OpenAPI/Swagger specs"), skills(`gh skill`), 출력 토큰 효율이다.
2. **토큰 비용 때문에 MCP 를 피하고 CLI 를 택하는 사용자 증언.**
   [cli/cli#12912](https://github.com/cli/cli/issues/12912) (2026-03-12):
   "github-mcp-server: It costs many tokens to keep this loaded due to the number of tools it
   provides, **CLI tools are currently more context efficient**."
3. **playwright 팀은 MCP 가 있는데도 별도로 CLI 를 만들었다.**
   [microsoft/playwright-cli README](https://github.com/microsoft/playwright-cli):
   "Modern coding agents increasingly favor CLI-based workflows exposed as SKILLs over MCP because
   CLI invocations are more token-efficient: they avoid loading large tool schemas… " —
   MCP 는 "persistent state 와 iterative reasoning 이 필요한 특화 워크플로"로 한정.
4. **자동 파생 MCP 의 품질 한계를 프레임워크가 직접 경고.** FastMCP 문서(A-4)의
   "significantly better performance with well-designed and curated MCP servers" 진술.
   툴 표면은 CLI 표면과 **다르게 설계되어야** 이롭다는 주장 — "한 선언 → 두 표면"의 전제를 흔든다.
5. **cobra 코어의 미온적 반응.** MCP 를 위한 메타데이터 PR 은 닫혔고, 스레드 참가자들은 MCP 대신
   `<cli> full-help --format json` 같은 **중립 스키마 덤프**를 선호했다(vincentbriglia, 2026-03-12:
   "i don't think that cobra should be exposing an MCP server per-se… so that anyone can create a
   thin-wrapper client in whatever language they prefer").

---

## 우리(internal/ops)가 이미 하는 것 vs 남들이 하는 것

| | 선언 수 | MCP 실행 모델 | 타입 | 추가 파생 |
|---|---|---|---|---|
| **tossinvest-cli** (`internal/ops/ops.go:90` `Operation`, `:114` `Catalog`) | 2 (cobra + ops) | in-process, typed client 직접 호출 | 핸들러 반환은 `any`, CLI 는 typed (ADR 0001) | **health probe** (`ProbeSpec`, `ops.go:81`) — 남들에게 없음 |
| ophis | 1 (cobra만) | **subprocess** + stdout 파싱 | args 전부 string, 포기 선언됨 | 없음 |
| cobra-mcp / yutu | 2 | in-process, 도메인 메서드 직접 | 제네릭 `GenToolHandler[T]` 로 구조 보존 | agent(in-memory transport) |
| flyctl | 2 | subprocess (`Builder` 가 CLI 인자열 생성) | 툴 인자 타입을 손으로 선언 | 없음 |
| planetscale/cli | 2 | in-process, API 클라이언트 직접 | 손으로 선언 | 없음 |
| railwayapp/cli | 2 | in-process, controllers 공유 | Rust 타입(params 구조체) | 없음 |
| Speakeasy | 1 (OpenAPI) | 생성기 | spec 기반 | SDK·Terraform provider·CLI |

읽을 점 세 가지:

1. **우리 구조(선언 2개 / 도메인 1개)는 이 분야의 사실상 표준이다.** railway·planetscale 이 독립적으로
   같은 곳에 도달했다. ADR 0001 의 결론이 소수 의견이 아니다.
2. **우리가 ADR 0001 에서 기각한 것(런타임 `Call` 경유의 타입 소거)을 ophis 는 그대로 채택했고,
   같은 벽에 부딪혀 typed args 를 포기했다.** 즉 우리가 이미 앞서 탐색을 끝낸 지점이다.
3. **우리만 갖고 있는 파생은 CLI/MCP 가 아니라 probe 다.** "한 선언 → CLI + MCP" 는 남들도 하지만
   "한 선언 → MCP + 헬스 모니터링" 은 조사 범위에서 다른 사례를 찾지 못했다. 차별점이 있다면 여기다
   (다만 이것이 남들이 원하는 것인지는 **확인 못 함** — 수요 증거 없음).

---

## 확인 못 한 것

- typer 전용, commander 전용 MCP 브리지의 존재 여부(검색으로 유의미한 레포 미발견 — 없다는 증명은 아님).
- Speakeasy 의 CLI+MCP 동시 생성 실제 동작/성숙도. 공식 문서 URL 2개가 404, 레포 description 만 확인.
- OpenAPI→MCP 생성기들(harsha-iiiv, cnoe-io, vincent-pli 등)의 개별 스타/유지보수 상태.
- flyctl 의 MCP 툴 60개가 CLI 커맨드 전체 대비 몇 %인지(부분집합 여부) 정밀 대조.
- ophis 의 실제 채택 규모. 코드 검색으로 import 하는 공개 레포 3개만 확인(Clyra-AI/safety,
  fillmore-labs/zerolint-test, linny006/mcp-servers-live) — GitHub 코드 검색 인덱스 한계로
  과소 집계일 가능성 큼.
- GitLab 호스팅 프로젝트(glab 등)는 GitHub API 로 조회 불가하여 미조사.
- gitlab-org/cli 는 GitHub 미러 404 로 조회 실패.
