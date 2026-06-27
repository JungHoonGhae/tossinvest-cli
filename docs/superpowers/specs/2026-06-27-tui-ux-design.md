# TUI/UX 고도화 — 설계

- 날짜: 2026-06-27
- 상태: 설계 승인됨 (브레인스토밍 산출물)
- 브랜치: `feat/hybrid-openapi` (하이브리드와 함께 배포)
- 참고: hermes-agent(NousResearch) — Ink(TS)+prompt_toolkit(Python). 코드 재사용 불가, UX 패턴만 차용 → Go는 charmbracelet `huh`(선택, 기보유) + `lipgloss`(스타일).

## 1. 목표 / 비목표

**목표** — CLI를 사람이 쓰기 편하게: (A) ID/심볼을 외워야 하던 지점에 **TTY 대화형 선택**, (B) 핵심 출력에 **정렬·색·상태표시**. 단 AI/스크립트 경로(플래그·JSON·비TTY)는 **불변**.

**비목표**
- 풀 인터랙티브 TUI 대시보드(bubbletea 단일 앱) — 별도 트랙(YAGNI).
- `order place` 심볼 선택(매수는 직접 입력이 자연스러움) — 후순위.
- 모든 출력 스타일링 — 핵심 출력만(YAGNI).
- 새 무거운 의존성 — `lipgloss`는 이미 huh 통해 모듈 그래프에 존재(직접 의존으로 승격).

## 2. 결정 사항 (브레인스토밍 합의)

| 축 | 결정 |
|----|------|
| 방향 | **타겟 대화형 선택 + 출력 폴리시** (풀 TUI 아님). |
| 선택 지점 | `order cancel`/`amend`(대기 주문), `order show`(최근 주문), `watchlist` 폴더(이름). 하나의 `pickOrder` 패턴으로 통일. |
| 출력 색 | **한국식 손익 색**: 빨강=상승/이익, 파랑=하락/손실. |
| 게이트 | 색 = `stdout이 TTY && NO_COLOR 미설정 && format==table`. 그 외(파이프·비TTY·NO_COLOR·json·csv)는 ANSI 없이 **기존 출력과 동일**. |
| AI 우선 | 플래그 주면 선택 스킵, 비TTY면 프롬프트 없이 명확한 에러. |

## 3. 대화형 선택 (TTY 전용)

### 동작 규약
- 대상 커맨드에서 식별자(orderID / 폴더 id)가 **플래그/인자로 주어지면** → 기존대로 그대로 사용(비대화형, AI 경로 불변).
- 식별자가 **생략 + stdout/stdin이 TTY**면 → 목록을 조회해 `huh` Select로 고르게 함.
- 식별자가 **생략 + 비TTY**면 → 프롬프트 없이 에러("orderID를 주거나 터미널에서 실행하세요"). 대기 금지.

### 지점별
- `order cancel` / `order amend` — `ListPendingOrders`로 대기 주문 목록 → Select(라벨: 종목·수량·가격·주문시각) → 선택 id로 진행.
- `order show`/inspect — 최근 주문(대기+체결) 목록 → Select.
- `watchlist` 폴더 작업 — 폴더 목록 → 이름으로 Select → 폴더 id로 진행.

### 컴포넌트
- `internal/tui` (기보유 `Select`/`IsInteractive` 재사용):
  - `func PickFromList(title string, items []Item) (selectedID string, err error)` — `Item{ID, Label string}`; 비TTY면 `ErrNotInteractive`. (huh Select 래핑)
- cmd 레이어: 식별자 미지정 + TTY일 때 목록 조회→`PickFromList`→id. 조회/매핑은 cmd에서, 렌더링 격리는 tui에서.
- 순수 매핑(`order → Item` 라벨 포맷)은 테스트 가능한 함수로 분리.

## 4. 출력 폴리시 (lipgloss, 게이트)

### 색 게이트 (안전 핵심)
- `internal/output/style.go`: `func colorEnabled(w io.Writer, format Format) bool` =
  `format==FormatTable && isTerminal(w) && os.Getenv("NO_COLOR")==""`.
- `isTerminal`: `internal/tui.IsInteractive` 재사용 또는 `os.File` char-device 체크(파일 핸들일 때만).
- **게이트 false면 스타일 0** → 출력이 현재와 바이트 동일 → 기존 fixture/계약 테스트 보존 + AI 파이프 불변.

### 스타일 대상 & 규칙
- 대상: `portfolio positions`, `account summary`, `quote`, `orders`, `watchlist`.
- 손익(수익률·평가손익·등락): 양수 빨강, 음수 파랑, 0 기본(한국식).
- 헤더: dim/bold. 상태/배지: 약한 강조. 컬럼 정렬은 기존 table.go 유지(스타일만 덧입힘).
- 과하지 않게(art-gallery 아님) — 가독성 우선.

### 의존성
- `lipgloss`를 go.mod 직접 의존으로(이미 그래프에 존재). `output`이 `lipgloss` import 허용(순수 스타일링, TTY 상호작용 아님 → huh 격리 원칙과 무관). huh는 여전히 `internal/tui`에만.

## 5. 아키텍처 요약
- `internal/tui/pick.go` — `PickFromList`(+`Item`), 비TTY 가드. huh 격리 유지.
- `internal/output/style.go` — `colorEnabled` 게이트 + 스타일 헬퍼(`profitColor`, `header`, `dim`, …).
- `internal/output/{portfolio,account,quote,orders,watchlist}.go` — 게이트 켜질 때만 스타일 적용(렌더 분기 최소, 값 포맷은 공유).
- `cmd/tossctl/{order,watchlist}.go` — 식별자 미지정+TTY 시 목록 조회→PickFromList. 플래그/비TTY 경로 불변.

## 6. 테스트
- `internal/tui`: `PickFromList` 비TTY → `ErrNotInteractive`(대기 없음). label 포맷 순수함수.
- `internal/output`: `colorEnabled` 진리표(table+tty+no NO_COLOR=true; json/csv/비tty/NO_COLOR=false). **게이트 false에서 출력 == 기존 plain**(기존 output 테스트가 비TTY로 도니 그대로 통과해야 함 = 회귀 가드). 게이트 true일 때만 ANSI 포함 단언.
- `cmd/tossctl`: 식별자 미지정+비TTY → 프롬프트 없이 에러; 식별자 지정 시 선택 스킵(기존 경로).
- 손익 색 매핑 순수함수(양수→빨강 코드, 음수→파랑) 단위 테스트.

## 7. 범위 밖 / 향후
- 풀 bubbletea 대시보드, `order place` 심볼 선택, 전 출력 스타일링, 스피너/프로그레스.
