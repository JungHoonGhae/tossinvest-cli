# Account access and Securities funding status — 2026-09-03

## 결론

웹 UI 노출 여부와 무관하게 현재 WTS 세션으로 읽을 수 있고 응답 계약까지 확인된 5개 API를
CLI와 ops/MCP에 연결했다. 모두 기능 영역은 `securities`, 접근 경로는 `wts`다.

- `account access-status [--account <key>]`는 최근 증권 접속 환경과 계좌별 제한 신호를 읽는다.
- `accumulate funding-status [--full]`의 추가 필드는 주식모으기·자동주문 자금연결과 거래목적 확인 절차에
  한정된다. 일반 Toss Banking이나 카드·소비 MyData 조회가 아니다.

어떤 호출도 계좌 잠금 해제, 설정 변경, 이체 또는 주문을 실행하지 않는다.

## 검증 근거

분석 대상 WTS build ID는 `Vn2JUgZup8HwoN8aQW3Nm`이며 77개 chunk를 확인했다. 정적
번들의 요청 호출부 또는 route descriptor에서 host와 method를 확인한 뒤, 로그인된 현재 세션으로
읽기 전용 호출을 수행해 아래의 결과 타입만 검증했다.

| Host | Endpoint | 확인한 `.result` 계약 | 공개 기능 |
|---|---|---|---|
| `wts-api` | `GET /api/v1/user/last-login-info` | `{channel, osName, agentName, timestamp}` | `account access-status` |
| `wts-cert-api` | `GET /api/v1/margin/cert/frozen-account` | `{isFrozen, startDate, endDate}` | `account access-status` |
| `wts-api` | `GET /api/v2/account/unlock/accident-account/count` | number | `account access-status` |
| `wts-cert-api` | `GET /api/v1/trading/open-banking/auto-trading` | `{connectedAccountBankCode, isRegistered}` | `accumulate funding-status` |
| `wts-api` | `GET /api/v1/trade-purpose-verification/my-data/account/exists` | boolean 200 또는 400 | 보류 — 같은 정상 세션에서도 응답이 불안정해 callable/probe에서 제외 |

마지막 MyData 경로는 정적 route descriptor와 라이브 스키마가 확인됐지만 일반 MyData 데이터를
반환하는 계약은 아니다. 나머지 네 경로는 현재 번들의 구체적인 호출 지점과 라이브 스키마를 함께
확인했다. 응답 값, 쿠키, 토큰, 원래 계좌 키는 분석 문서나 fixture에 저장하지 않았다.

## 보류한 후보

권리 정보, 체결 이력, 환전 자동주문, 지연 이체, 캐시백, 자동매매 규칙 등 함께 조사한 후보는
현재 계정에서 `null` 또는 빈 배열만 반환해 안정적인 item 계약을 만들 수 없었다. 경로 이름만으로
필드를 추측하지 않고 카탈로그 후보로 남겼다. 실제 비어 있지 않은 응답 또는 정적 decoder 계약을
확보하면 다시 검증한다.

## 공개 표면과 회귀 감시

`account access-status`는 CLI table·JSON·CSV와 `account_access_status` operation에서 같은 타입을
사용한다. 계좌 선택은 기본 계좌 또는 `--account`를 받지만, 출력에는 원래 키 대신 세션 비밀로
서명한 불투명한 account scope만 둔다.

추가한 probe는 `account-last-login`, `account-margin-frozen`, `account-accident-count`,
`auto-trading-open-banking` 4개다. 전체 수는 런타임
operation catalog에서 파생되며 `docs/operations.md`에 동기화한다.
