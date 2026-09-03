# Asset snapshot 계약 검증 및 구현 — 2026-09-03

## 결론

웹 화면에 노출되지 않던 토스증권 자산 스냅샷 6개 읽기 계약을 현재 WTS 세션으로
검증하고 CLI·ops/MCP·monitor에 연결했다. 이 기능은 일반 Toss 앱에서 보이는 형태와
유사하지만, 호출 백엔드는 별도 모바일 토큰이 아니라 `wts-cert-api`다. 따라서 분류는
`domain=securities`, `backend=wts`이며 `mobile`이라는 별도 백엔드를 추측하지 않는다.

## 근거 수준

| 근거 | 결과 |
|---|---|
| 현재 production WTS 번들의 요청 레지스트리 | 6개 경로 모두 `host:"cert", method:"GET"`와 전체 동적 경로 확인 |
| 로그인된 세션의 읽기 전용 호출 | 전 계좌 합산·단일 계좌 변형 모두 HTTP 200과 응답 타입 확인 |
| 일반 Toss Android 5.275.0 정적 본체 | 경로 문자열 미발견; 증권 미니앱 코드가 원격 배포될 수 있으므로 부재를 API 부재로 해석하지 않음 |
| WTS 웹 UI 캡처 | 2026-08-24에 관련 화면에서 미호출 확인; 현재도 CLI 지원 여부를 UI 노출 여부로 결정하지 않음 |

세션 쿠키, 계좌 키, 금액, 종목명, 커서 값은 문서나 fixture에 저장하지 않았다. 테스트
fixture는 합성 값만 사용한다.

## 검증된 요청 계약

| 범위 | 요청 | 필수 입력 |
|---|---|---|
| 전 계좌 추이 | `GET /api/v1/asset-snapshot/all-accounts/chart/ONE_MONTH/DAY` | 없음 |
| 단일 계좌 추이 | `GET /api/v1/asset-snapshot/chart/ONE_MONTH/DAY` | `accountKey` 헤더 |
| 전 계좌 이력 | `GET /api/v1/asset-snapshot/all-accounts/page` | `pageSize` 양의 정수, `cursorKey` 선택 |
| 단일 계좌 이력 | `GET /api/v1/asset-snapshot/page` | 위 query + `accountKey` 헤더 |
| 전 계좌 일자 상세 | `GET /api/v1/asset-snapshot/all-accounts/detail-by-date` | `baseDate=YYYY-MM-DD` |
| 단일 계좌 일자 상세 | `GET /api/v1/asset-snapshot/detail-by-date` | 위 query + `accountKey` 헤더 |

`pageSize`는 누락되면 400이고 0·음수도 거절된다. 파라미터 이름은 `cursorKey`이며
`cursor` 또는 `nextCursorKey`를 보내도 페이지가 이동하지 않는다. 실시간 현재 지점은
요청한 과거 행 수 외에 한 건 더 붙을 수 있으므로 클라이언트가 결과를 `pageSize`로
잘라서는 안 된다. 실제 종료 페이지에서는 `nextCursorKey`가 JSON `null`이었다.

차트 서버는 `INVALID/INVALID` 같은 임의 값도 HTTP 200으로 받아 의미 없는 요청을
성공처럼 보이게 한다. 그래서 CLI는 번들 호출부와 라이브 응답을 함께 검증한
`ONE_MONTH/DAY` 조합만 제공한다. 다른 기간은 정확한 enum과 의미를 추가 관측한 뒤
확장한다.

## 검증된 응답 계약

- 추이: 보유 여부 플래그, 평가액 증감, 기간 최고·최저 평가액, 날짜별 원금·평가액·수익률·실시간 여부
- 이력: 날짜별 원금·평가액·손익·수익률·실시간 여부·평가 완료 여부와 다음 커서
- 상세: 전체 요약과 `kr`·`option`·`us`·`bond` 시장 요약, 각 시장의 보유종목 목록
- 보유종목: 상품 코드·ISIN·심볼·이름·수량·매입단가·매입액·평가액·손익·수익률·시장 구분·유형

빈 시장도 네 섹션을 모두 반환하도록 정규화하고 배열은 `null` 대신 빈 배열로
내보낸다. `--account` 결과에는 원래 계좌 키 대신 세션 비밀로 서명한 불투명 scope만
포함한다.

## 공개 표면과 회귀 감시

- CLI: `portfolio performance`, `portfolio snapshots`, `portfolio snapshot <date>`
- ops/MCP: `portfolio_performance`, `portfolio_snapshots`, `portfolio_snapshot`
- machine output: JSON은 전체 타입, CSV는 추이/이력의 안정된 행과 상세의 summary/holding 행을 보존
- monitor: 전 계좌·단일 계좌 변형 6개를 모두 probe하며 단일 계좌 요청은 검증된 기본 `accountKey`를 주입

모든 동작은 읽기 전용이다. 주문·이체·설정 변경 권한은 추가하지 않았다.
