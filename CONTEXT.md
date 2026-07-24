# CONTEXT — tossinvest-cli 도메인 용어

이 프로젝트에서 말이 갈리기 쉬운 용어를 한 곳에 모은다. 코드·PR·이슈에서 아래 낱말은
여기 정의된 뜻으로만 쓴다. 되돌리기 어려운 결정은 `docs/adr/` 에 기록한다.

## 백엔드

**공식 Open API (official)** — 토스증권이 공개한 정식 API. `internal/official`.
`tossctl openapi login` 으로 발급한 자격증명이 필요하다. 계약이 안정적이고 주문
집행의 유일한 경로다.

**WTS** — 토스증권 웹 트레이딩 시스템의 내부 API. `internal/client`.
`tossctl auth login` 으로 얻은 웹 세션 쿠키를 재사용한다. 공식 API 에 없는 조회(인기
순위·수급·AI 시그널·스크리너·업종·어닝·브리핑·배당 등)를 제공하지만 **비공식이라 예고
없이 바뀔 수 있다**.

**hybrid routing** — 하나의 조회를 두 백엔드 중 어디로 보낼지 런타임에 정하는 것.
`internal/hybrid` 가 담당한다. 정책은 `Policy{Prefer, Fallback}`:

- 읽기: official 을 먼저 시도하고, 폴백 대상 실패(전송·인증·IP·레이트리밋·서버 오류)
  면 WTS 로 재시도한다. 도메인 오류(404 등)는 폴백하지 않고 그대로 돌려준다.
- 쓰기(주문): **절대 교차 재시도하지 않는다.** 한 백엔드로만 보내고 실패하면 거기서
  멈춘다. 중복 주문을 막기 위한 규칙이다.

CLI 와 MCP 는 **같은 라우터를 공유한다** — 사람이 부르든 에이전트가 부르든 같은 조회는
같은 방식으로 해석된다.

## 오퍼레이션

**operation** — 카탈로그에 등록된 하나의 API 동작. `internal/ops` 가 단일 레지스트리이며
`ID`, `Params`, `Backend`, 그리고 typed client 를 호출하는 `handler` 로 이루어진다.
`internal/mcp`(에이전트용 3-tool 카탈로그)와 `internal/monitor`(헬스 probe)가 여기서
파생된다.

**backend (오퍼레이션 필드)** — 그 오퍼레이션이 요구하는 자격증명. `Catalog.Call` 이
디스패치 전에 검사한다.

| 값 | 뜻 | 필요한 것 |
|---|---|---|
| `""` (기본) | official 전용 | 공식 자격증명 |
| `"wts"` | WTS 전용 | 웹 세션 |
| `"auto"` | hybrid 라우터가 서빙 | **둘 중 하나면 충분** |
| `"none"` | 인증 불필요 | 없음 (예: `auth_status`) |

`"auto"` 는 official 과 WTS 양쪽에 **시그니처가 동일한** 대응 메서드가 있어 적응 코드
없이 라우터에 얹을 수 있는 오퍼레이션에만 붙인다.

**probe** — 오퍼레이션에 선택적으로 붙는 모니터링 명세. typed client 를 **일부러
우회**해서 raw method/URL/body 로 찌른다. 클라이언트 코드가 서버 변경과 함께 움직여도
계약 변화를 잡아내기 위함이다. 선별적으로만 단다(CLI 표면당 대표 엔드포인트 하나).

**write operation** — 주문 생성·취소·정정. config 옵트인 + execute·confirm 토큰으로
이중 게이팅되며 official 전용 브로커로만 라우팅된다.

## 인증 상태

**auth snapshot** (`AuthStatus`) — 백엔드별 연결 여부와 만료 시각. **비밀값을 담지
않는다** — 불리언과 타임스탬프뿐이라 에이전트에게 그대로 돌려줘도 안전하다.

hybrid 라우터는 세션이 없어도 구성되므로(임베드된 WTS 클라이언트가 non-nil 이어야 함),
**"웹 세션이 있는가" 의 판정 기준은 포인터의 nil 여부가 아니라 이 스냅샷이다.**
`Catalog.Call` 의 게이팅이 이 값을 본다.
