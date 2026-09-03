# Notification and AI content status — 2026-09-03

## 결론

기존 `notifications list`는 통합 알림 설정 배열을 반환한다. 별도 알림 종류별 endpoint를
중복 호출하면 동일 상태의 진실 소스가 둘이 되므로, `notifications status`는 통합 설정을
재사용하고 여기에 받은함과 AI 분석 동의만 결합한다. 기존 `notifications list`의 JSON/CSV
계약은 바꾸지 않고 별도 `notification_status` operation으로 제공한다.

모두 `domain=securities`, `backend=wts`인 GET 요청이다. 알림 구독, 분석 동의, 계좌 또는 주문
상태를 변경하지 않는다.

## 검증 근거

분석 대상은 WTS build ID `Vn2JUgZup8HwoN8aQW3Nm`이다. 각 경로의
`host:"cert",method:"GET",path:...` 선언을 확인했고, 로그인 세션으로 값을 출력하거나
저장하지 않은 채 `.result`의 키와 타입만 다시 확인했다.

| Endpoint | 확인한 `.result` 계약 | 공개 필드 |
|---|---|---|
| `GET /api/v1/user-alimies` | 알림 종류별 `{type, enabled}` 배열 | 아래 세 알림 활성 상태의 정식 진실 소스 |
| `GET /api/v1/inbox-alimies/has-unread` | `{unread: boolean}` | `inbox_unread` |
| `GET /api/v1/reasoning/agreement` | `boolean` | `reasoning_agreement` |
| `GET /api/v1/reasoning-news/count` | `number` | deprecated JSON/CSV `reasoning_news_count` |

통합 설정의 `AI_ISSUE_SNS_RELEASE`, `FOMC_LIVE`, `REASONING_SUBSCRIPTION` 항목을 각각
기존 공개 boolean에 매핑한다. 별도 전용 GET 3개는 중복이므로 구현·probe에서 제외했고,
`reasoning-news/count`는 계정 알림 상태가 아니라 전역 UI 콘텐츠 수라 사용자용 table에서는
숨긴다. 숫자 JSON/CSV 계약을 유지하는 동안에는 값을 0으로 꾸미지 않고 실제 endpoint를 계속
조회하며 deprecated로 취급한다.
`reasoning/agreement`의 POST 계약은 이번 읽기 기능이 사용하지 않는다.

## 보류한 후보

같이 재검증한 계좌 잠금, 권리, 체결, 자동환전, 지연이체, 캐시백 후보는 현재 계정에서
`null` 또는 빈 목록만 반환했다. 목록 item 계약을 추측하지 않고 실제 데이터나 정적 decoder를
확보할 때까지 카탈로그 후보로 유지한다.

## 공개 표면과 회귀 감시

CLI table·JSON·CSV와 ops/MCP가 하나의 `NotificationStatus` 타입을 사용한다. 네 HTTP 의존성은
`notification-settings`, `notification-inbox-unread`, `notification-reasoning-agreement`,
`notification-reasoning-news-count` probe로 감시한다. `notification-settings` probe와 typed
client는 세 canonical type과 각 `enabled` boolean이 모두 존재하는지도 검사하므로 누락을
`false`로 오인하지 않는다. 응답 값, 쿠키, 토큰은 fixture나 문서에 저장하지 않았다.
