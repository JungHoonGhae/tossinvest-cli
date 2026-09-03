# Notification and AI content status — 2026-09-03

## 결론

기존 `notifications list`는 일반 알림 설정 배열만 반환한다. 현재 WTS 번들에는 받은함과 AI
콘텐츠에 관한 독립된 읽기 계약도 있으며, 로그인 세션에서 6개 모두 확인된 boolean 또는 number
응답을 반환했다. 기존 자동화의 JSON/CSV 계약을 바꾸지 않기 위해 별도 `notifications status`와
`notification_status` operation으로 묶었다.

모두 `domain=securities`, `backend=wts`인 GET 요청이다. 알림 구독, 분석 동의, 계좌 또는 주문
상태를 변경하지 않는다.

## 검증 근거

분석 대상은 WTS build ID `Vn2JUgZup8HwoN8aQW3Nm`의 77개 chunk다. 각 경로에서
`host:"cert",method:"GET",path:...` 선언을 확인했고, 현재 로그인 세션으로 값을 출력하거나
저장하지 않은 채 `.result`의 키와 타입만 다시 확인했다.

| Endpoint | 확인한 `.result` 계약 | 공개 필드 |
|---|---|---|
| `GET /api/v1/inbox-alimies/has-unread` | `{unread: boolean}` | `inbox_unread` |
| `GET /api/v1/ai-issue/sns-release/alimy` | `{enabled: boolean}` | `ai_issue_sns_release_alert_enabled` |
| `GET /api/v1/fomc-live/alimy` | `{enabled: boolean}` | `fomc_live_alert_enabled` |
| `GET /api/v1/reasoning-contents/alimy/subscription` | `{enabled: boolean}` | `reasoning_contents_alert_enabled` |
| `GET /api/v1/reasoning/agreement` | `boolean` | `reasoning_agreement` |
| `GET /api/v1/reasoning-news/count` | `number` | `reasoning_news_count` |

`reasoning/agreement`에는 POST 계약도 있지만 이번 기능은 GET 상태 조회만 사용한다. 이름만으로
뉴스 수를 “미확인 수”라고 해석하지 않고 서버 계약 그대로 `reasoning_news_count`로 노출한다.

## 보류한 후보

같이 재검증한 계좌 잠금, 권리, 체결, 자동환전, 지연이체, 캐시백 후보는 현재 계정에서
`null` 또는 빈 목록만 반환했다. 목록 item 계약을 추측하지 않고 실제 데이터나 정적 decoder를
확보할 때까지 카탈로그 후보로 유지한다.

## 공개 표면과 회귀 감시

CLI table·JSON·CSV와 ops/MCP가 하나의 `NotificationStatus` 타입을 사용한다. 여섯 HTTP 의존성은
`notification-inbox-unread`, `notification-ai-issue-release`, `notification-fomc-live`,
`notification-reasoning-contents`, `notification-reasoning-agreement`,
`notification-reasoning-news-count` probe로 각각 감시한다. 응답 값, 쿠키, 토큰은 fixture나 문서에
저장하지 않았다.
