# ADR 0001 — CLI 커맨드는 ops 레지스트리를 경유하지 않는다

- 상태: 채택
- 날짜: 2026-07-25
- 관련: `internal/ops`, `cmd/tossctl`, `internal/output`

## 맥락

`internal/ops` 는 오퍼레이션 단일 레지스트리이고 `internal/mcp` 와 `internal/monitor`
가 여기서 파생된다. 반면 `cmd/tossctl` 의 cobra 커맨드는 레지스트리를 거치지 않고
typed client 를 직접 호출한 뒤 `internal/output` 의 포매터로 넘긴다.

겉보기에 이것은 명백한 이중 선언이다 — 같은 조회가 cobra 커맨드로 한 번, `ops.Operation`
으로 또 한 번 선언된다. 실제로 새 오퍼레이션 하나를 추가하면 domain · client · output ·
cmd · root.go · ops 여섯 파일을 함께 손대야 한다. 그래서 "cmd 를 ops 로 태워 call seam
을 하나로 합치자" 는 제안이 자연스럽게 나온다.

## 결정

**합치지 않는다.** cobra 커맨드는 앞으로도 typed client 를 직접 호출한다.

## 이유

`Catalog.Call` 의 시그니처가 막는다:

```go
func (c *Catalog) Call(ctx context.Context, deps *Deps, id string, args map[string]any) (any, error)
```

반환 타입이 `any` 다. MCP 에게는 이상적이다 — 받은 값을 그대로 JSON 직렬화하면 끝이고,
레지스트리가 타입을 몰라도 된다. 하지만 CLI 는 `output.WriteQuote(w, format, domain.Quote)`
처럼 **구체 타입**을 요구한다. cmd 가 `Call` 을 경유하면 오퍼레이션마다
`res.(domain.Quote)` 형태의 타입 단언이 필요해지고, 지금 컴파일러가 잡아 주는 실수가
런타임 패닉으로 내려간다.

즉 cmd 가 client 를 직접 부르는 것은 중복이 아니라 **정적 타입을 지키는 대가**다.
통합은 코드량을 줄이는 대신 타입 안전성을 잃으므로 순손실이다.

검토한 대안:

- **제네릭 `CallTyped[T]`** — Go 는 메서드에 타입 파라미터를 붙일 수 없어 패키지 레벨
  함수여야 하고, 레지스트리 핸들러가 여전히 `any` 를 반환하므로 내부 단언이 남는다.
  복잡도만 늘고 문제는 그대로다.
- **48개 타입 단언 수용** — 위 이유로 기각.

## 결과

- 두 표면(사람용 CLI, 에이전트용 ops)은 계속 공존한다. 공유되는 것은 typed client 다.
- 6파일 lockstep 은 남는다. 이를 줄이려면 레지스트리에서 **선언을 생성**하는 방향
  (`docs/migration` 의 discovery 기반 동적 커맨드)이지, 런타임에 `Call` 을 경유하는
  방향이 아니다.
- 두 표면이 **동작까지 갈라지는 것**은 별개 문제이며, 그쪽은 실제로 수렴시켰다 —
  `Backend: "auto"` 와 hybrid 라우팅으로 에이전트도 CLI 와 같은 official→WTS 폴백을
  받는다. `CONTEXT.md` 의 "hybrid routing" 항목 참고.

## 재검토 조건

`Call` 이 타입을 보존하는 형태로 바뀌면(예: 레지스트리가 제네릭으로 재작성되어
오퍼레이션별 반환 타입이 정적으로 드러나면) 이 결정을 다시 검토할 가치가 있다.
