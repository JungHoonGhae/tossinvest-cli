# `list_operations` 다이어트 — 측정과 결론

**날짜**: 2026-07-25 · **대상**: v0.30.0 (오퍼레이션 52개)
**결론**: **지금은 하지 않는다.** 최악의 경우를 최적화하는 일이고, 원래 동기가 측정과 어긋난다.

---

## 배경

무인자 `list_operations`(= `tossctl ops list`) 가 컨텍스트를 많이 먹는다는 문제 제기가
있었다. 제안은 "summary 첫 문장만 남기면 34% 절감" 이었고, 근거는 "`describe_operation`
과 거의 같은 것을 반환 중이라 3-tool 설계 의도가 미실현" 이었다.

두 주장 모두 측정으로 확인되지 않았다.

## 재현

```bash
tossctl ops list > /tmp/ops-full.json     # 무인자 = 최악의 경우
tossctl ops describe <id>                 # 비교 대상
```

MCP 와 CLI 는 같은 페이로드를 낸다 — `internal/mcp/server.go` 의 `toolResult` 도
`internal/output` 의 `WriteJSON` 도 둘 다 `MarshalIndent(v, "", "  ")` 다. 아래 수치는
양쪽에 그대로 적용된다.

## 측정

무인자 `ops list` = **15,306 바이트** (한글이 섞여 대략 4,600 토큰).

### 필드별 구성

| 필드 | 바이트 | 비중 |
|---|---:|---:|
| summary | 5,927 | 38.7% |
| **들여쓰기·구조** | **4,256** | **27.8%** |
| path | 1,521 | 9.9% |
| category | 965 | 6.3% |
| id | 944 | 6.2% |
| method | 683 | 4.5% |
| backend | 553 | 3.6% |
| required | 445 | 2.9% |
| write | 33 | 0.2% |

### 시나리오별

| | 결과 바이트 | 절감 | 정보 손실 |
|---|---:|---:|---|
| 현재 | 15,306 | — | — |
| A 첫 문장만 | 13,226 | 13.6% | 라우팅 신호 (아래) |
| B summary 삭제 | 8,859 | 42.1% | 라우팅 불가 |
| C id+category+backend+write 만 | 4,618 | 69.8% | list 가 순수 색인이 됨 |
| D compact JSON (들여쓰기 제거) | 11,842 | 22.6% | **없음** |

## 두 전제의 정정

### 1. "첫 문장만 남기면 34% 절감" → 실제 13.6%

summary 는 전체의 38.7% 이고, 첫 문장만 남겨도 그중 39% 만 줄어든다. 전체 기준
2,080 바이트다. 34% 는 여기에 `path` + `method` 까지 지워야 나오는 숫자(시나리오 D
계열, 34.8%)이지 summary 만으로는 도달하지 않는다.

### 2. "describe 와 거의 같은 것을 반환" → describe 가 1.9배

| | 합계 바이트 |
|---|---:|
| list 항목 52개 | 13,283 |
| describe 52회 | 25,051 |

list 는 `params` 를 통째로 내지 않는다 — 그게 describe 페이로드의 대부분이다.

```
place_order         list  269 → describe 1861  (+1592)
transactions        list  250 → describe 1155  (+905)
completed_orders    list  308 → describe 1074  (+766)
orders              list  141 → describe  861  (+720)
modify_order        list  260 → describe  959  (+699)
```

**3-tool 분리는 의도대로 작동하고 있다.** 개선안의 동기 자체가 성립하지 않는다.

## 시나리오 A 가 실제로 버리는 것

첫 문장은 *"이게 뭔지"* 를 말하고, 버려지는 뒷문장이 *"언제 이걸 고르는지"* 를 말한다.
라우팅에 쓰라고 있는 필드에서 라우팅 정보를 빼는 셈이다.

- **`auth_status`** — 버림: *"Call this to diagnose auth before other operations; a
  disconnected/expired backend means run `tossctl auth login` …"* ← 이 오퍼레이션을 고를 **이유**
- **`market_news`** — 버림: scope enum 전체(`all`/`watchlist`/`holdings`/`soaring`/
  `recommended`/`latest`) + 서버 상한 50건. scope 를 알려면 describe 를 불러야 한다
- **`profit_daily`** — 버림: *"currency selects the RATE BASIS … it is not a filter"*
  ← **함정 경고**. 빼면 에이전트가 `currency` 를 필터로 오해한다
- **`account_detail`** — 버림: 반환 필드 목록(D+0/1/2 출금 가능액, 한도, 미수거래 여부)과
  마스킹 동작

### 손익분기

A 는 2,080 바이트를 아낀다. describe 한 번은 평균 482 바이트(25,051 / 52)다.

> **추가 describe 호출 4~5번이면 절감이 사라진다.**

잘못 고른 오퍼레이션을 한 번 호출하면 그것만으로 적자다.

## 왜 지금 하지 않는가

1. **최악의 경우를 최적화하는 일이다.** 15,306 바이트는 *무인자* 호출 값이다.
   `--query dividend` 는 3개만 반환하며 1,000 바이트대로 떨어진다. 무인자 list 는
   세션당 사실상 한 번이다.
2. **A 의 손익분기가 4~5회로 너무 얕다.** 절감이 오라우팅 한 번에 지워진다.
3. **원래 동기가 사실이 아니었다** (위 정정 2).

## 굳이 한다면

순서는 **`path` + `method` 삭제 (14.4%, 정보 손실 거의 0)**:

- 에이전트는 `id` 로 호출하지 `path` 로 호출하지 않는다
- `path` 의 `wts:` / `local:` 접두사가 주는 정보는 `backend` 필드가 이미 준다
- 단, `path` 는 사람이 디버깅할 때와 `docs/reverse-engineering/wts-endpoints.json`
  카탈로그와 대조할 때 쓰인다. `describe` 에는 남겨야 한다

**summary 는 건드리지 않는다.**

### 시나리오 D(compact JSON)를 채택하지 않는 이유

22.6% 를 정보 손실 0 으로 얻지만 비용이 두 가지다.

- `ops list` 를 사람이 눈으로 훑기 어려워진다
- `output.WriteJSON` 공유를 깨야 한다. 두 번째 인코더 설정이 생기는데, 들여쓰기를
  한 곳에서 결정하려고 만든 함수다. MCP 의 `toolResult` 도 같이 바꿔야 한다

토큰 22.6% 가 그 두 가지보다 급해질 때 다시 본다.

## 트리거

**실제로 컨텍스트가 부족해 문제가 생겼을 때** 재검토한다. 구체적으로:

- 오퍼레이션 수가 100개를 넘어 무인자 list 가 ~9,000 토큰대에 접근할 때
- 에이전트가 무인자 list 를 세션당 여러 번 부르는 패턴이 관찰될 때
- 호스트가 툴 결과 크기 상한에 걸릴 때 (`maxResultBytes` 축약이 실제로 발동할 때)
