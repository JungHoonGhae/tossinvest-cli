# Android 5.275.0 정적 계약 감사 — 2026-09-02

## 결론

- APK 문자열만 보고 만들어진 기능은 없다. 이번에 구현한 것은 Android 인터페이스와
  serializer, repository 호출부, 현재 WTS 세션의 마스킹된 라이브 schema까지 모두 맞은
  `account overview` 하나다.
- 카드 결제 내역·이번 달 소비·은행 계좌 조회 계약은 앱에 존재하고, 웹 UI 유무와 무관하게
  API 자체는 호출 가능한 구조다. 다만 Toss Home/MyData client 소속이며 현재
  `.tossinvest.com` WTS 세션만으로 인증할 수 있다는 근거가 없으므로 구현하지 않았다.
- 일반 Toss Banking/MyData 쓰기 endpoint도 확인했지만, 구현할 만큼 모바일 인증·동의·복구
  계약이 완성된 후보는 없었다. 실제 모바일 mutation 호출은 한 건도 하지 않았다.
- 후속 WTS 감사(2026-09-03)에서 **증권 목표가 알림**과 **증권 포트폴리오 보유종목 숨김**은
  WTS 호출부·요청 serializer·읽기 상태 계약이 모두 확인되어 안전한 preview/confirm 쓰기로
  구현했다. 이는 일반 Banking/MyData 모바일 쓰기가 아니다.

## 분석본 신뢰성

| Item | Value |
| --- | --- |
| Package | `viva.republica.toss` |
| Version | `5.275.0` (`50527500`) |
| XAPK SHA-256 | `aead5b60995d3f7e16e4790d330d24ab32e28165393e3fae415431b4295d8d85` |
| Base APK SHA-256 | `06715f2d15dd530cb426474e87a4b39aaaf91d1013ef8db171f882d2bf29e2ca` |
| APK signature | v2 verified, v3 verified |
| Certificate subject | `L=Seoul, O=Viva Republica, OU=Developing unit, CN=Seung Gun Lee` |
| Certificate SHA-256 | `45:DB:EC:B9:90:A1:37:45:58:D8:0A:6C:65:A9:DE:46:5F:96:0F:09:7D:F9:D1:2E:F9:13:38:2C:EC:BD:E0:5C` |

서명 검증은 APK 내용이 배포 후 변조되지 않았고 예상한 배포 주체의 키와 일치한다는
근거다. 앱 동작이나 비공개 API의 장기 안정성을 보장하지는 않는다.

## 구현한 계약 — `account overview`

정적 근거:

- Retrofit interface: `POST /api/v1/dashboard/all-accounts`
- request serializer: `sections: List<String>`
- default request: `{"sections":["SUMMARY_WITH_MINOR"]}`
- outer response: `SecuritiesBaseApiResponse<List<JsonObject>>` (`result[]`)
- repository: `result` 첫 항목을 `AssetSummaryResponse`로 decode
- inner response: `data.accountOverviews[]`, `data.minorAccountOverviews[]`,
  `data.totalAssetAmount`
- item: `accountName`, `accountNo`, `pendingOrderCount`, `totalAssetAmount`

호스트는 APK에서 난독화된 base-client accessor로 남았지만, 기존 카탈로그가
`wts-info-api`에서 parameter validation을 관측했고 이번 read-only 라이브 요청이 정확한
본문으로 200 및 위 schema를 반환해 최종 `verified`로 올렸다. 값은 출력·커밋하지 않았다.

CLI는 `account overview [--full]`이다. 일반 계좌와 미성년 계좌를 구분하며 account number는
table/JSON/CSV 모두 기본 마스킹한다. `monitor api`는 같은 본문과 세 핵심 response path를
검사한다.

## 은행·카드·소비 계약 — `partial`, 구현하지 않음

다음 인터페이스는 method/path가 정적으로 확인됐다.

| Area | Contract examples | Level | Reason not implemented |
| --- | --- | --- | --- |
| 월간 소비 | `POST v3/home/mydata-home/consumption/overview/legacy-with-dst?yearMonth=...` | `partial` | Toss Home client; 현재 WTS auth와 호환 미확인 |
| 소비 정보 | `GET /api/v3/home/consumption/info` | `partial` | 같은 main API client; mobile session과 MyData 동의 필요 |
| 카드 거래 | `POST v3/home/consumption/card-code/{cardCode}/transactions?yearMonth=...` | `partial` | 카드 식별자·body·동의 수명 검증 필요 |
| 전체 계좌 | `POST v3/home/accounts` | `partial` | 증권 WTS의 all-accounts와 다른 금융자산 영역 |
| 잔액 | `POST v3/home/mydata-home/mydata-account/balances` | `partial` | main API request cipher·session header 계약 필요 |
| 계좌 거래 | `POST v3/home/s/accounts/{referenceId}/transactions?...` | `partial` | 모바일 reference/auth lifecycle 미확인 |

이 API들은 `TossApiServiceModule`이 만드는 main API Retrofit client에 묶여 있고, base host는
`StatusManager`의 fallback에서도 `api-gateway.toss.im`으로 확인된다. 해당 client는 공통
app/device header, pay-session에서 공급되는 조건부 header, `ApiCipherRequestInterceptor`와
`ApiCipherInterceptor`를 함께 사용한다. 반면 `tossctl auth login`은 Toss Securities web
state를 가져와 `.tossinvest.com`의 WTS API 세 호스트에 적용한다.

즉 API가 없어서가 아니라, 현재 보유한 자격증명과 request envelope의 호환성이 아직
증명되지 않은 것이다. 웹 UI는 endpoint를 발견하는 힌트일 뿐 호출 가능 여부의 기준이 아니다.
은행 기능을 추가하려면 독립 connector, 명시적 MyData 동의, 별도 secret storage, 기본
개인정보 마스킹부터 설계해야 한다.

정리하면 기능 영역과 접근 경로는 별개다. 이 문서의 Banking/MyData는
`domain=banking|mydata, source=mobile`이고, 증권 모바일 화면에서 발견한 뒤 WTS로도 검증한
계약은 `domain=securities, source=wts`다. `platform`이라는 중간 도메인은 사용하지 않는다.

### 증권 주식모으기용 오픈뱅킹 상태는 별도 `verified`

일반 Toss Home/MyData의 은행 계좌·소비 내역과 달리,
`GET /api/v1/autotrade/open-banking/info/find`는 현재 WTS 번들의 `wts-api` 클라이언트가
소유하고 같은 증권 웹 세션으로 읽기 전용 라이브 schema까지 확인됐다. 따라서 전체 은행
거래내역이 아니라 **주식모으기 출금계좌 연결 상태만** `banking status [--full]`로 구현했다.
CLI·ops/MCP는 예금주명과 계좌번호를 기본 마스킹한다. 같은 상태 조회에
`open-banking/creatable`과 `open-banking/need-registration`을 묶어 연결 생성 가능 여부와
등록 필요 여부도 보존한다.

## 후속 WTS 번들로 추가 검증한 읽기 계약

다음 계약들은 Android APK 근거가 아니라, 현재 WTS 77개 chunk의 정적 호출부와 읽기 전용
라이브 응답 schema를 교차 검증했다. 빈 응답의 item model을 추측하거나 쓰기 API를 호출하지
않았다.

| Contract | CLI | Preserved fields |
| --- | --- | --- |
| `GET /api/v2/reasoning/personalized` | `market briefing` | 보유·관심 종목, 수익률, 시그널 방향, AI 사유, 뉴스, 관련 종목 |
| `GET /api/v1/calendar/ai-summary/key-events` | `market key-events` | 실적 예상·발표·서프라이즈, 경제지표 실제·예상·직전값 |
| `GET /api/v1/user-alimies` | `notifications list` | 알림 타입·활성화·갱신시각; 내부 `userId`는 폐기 |
| `GET /api/v1/autotrade/open-banking/creatable` | `banking status` | 새 자금연결 생성 가능 여부 |
| `GET /api/v1/autotrade/open-banking/need-registration` | `banking status` | 자금연결 등록 필요 여부 |
| `GET /api/v1/trading/settings/simple-trade` | `account trading-settings` | 계좌별 간편주문 활성 상태 |
| `GET /api/v2/trading/settings/investor-exchange-choice-type` | `account trading-settings` | KRX/NXT 체결시장 선택 값 |
| `GET /api/v1/users/settings/me/ats-notification` | `account trading-settings` | ATS 알림 활성 상태 |
| `GET /api/v1/member-subscriptions/get-option-real-time-tick` | `account trading-settings` | 옵션 실시간 시세의 `requested`·`serviced`·`shouldCharged` 원문 의미 플래그 |
| `GET /api/v1/securities-transfer/my-accounts` | `account transfer-accounts` | 내 이체 계좌의 은행 코드·계좌번호·계좌 ID; 번호 기본 마스킹 |
| `GET /api/v1/securities-transfer/recent-accounts` | `account transfer-accounts` | 최근 목적지 계좌의 은행 코드·계좌번호; 번호 기본 마스킹 |
| `GET /api/v1/dashboard/wts/overview/ai-signals/latest?nationCode=KOR\|USA` | `market briefing --scope kr\|us` | 개인화 브리핑과 같은 시그널·AI 사유·뉴스·관련 종목 구조 |
| `GET /api/v1/dashboard/wts/overview/ai-signals/detail?productCode=&productType=` | `market signal <symbol> [--type]` | 전체 AI 근거·이슈·키워드·출처 뉴스·연관 종목 흐름; `result:null`은 정상 빈 상태 |
| `GET /api/v2/dashboard/wts/overview/tics/{id}/simple` | `market sector <id>` | 업종명·현재 등락률·기간·이미지 |
| `GET /api/v2/dashboard/wts/overview/tics/{id}/overview` | `market sector <id>` | 업종명·요약·설명·깊이·종목/ETF 수·재귀 연관 업종 트리 |
| `POST /api/v2/dashboard/wts/overview/tics/{id}/stocks` body `{}` | `market sector <id>` | 구성 종목의 가격·등락·시총·거래대금·거래량·투자의견 |
| `POST /api/v2/dashboard/wts/overview/tics/{id}/etfs` body `{}` | `market sector <id>` | ETF 가격·보수·레버리지·최대 편입종목·거래대금 |
| `GET /api/v2/dashboard/wts/overview/tics/{id}/news` | `market sector <id>` | 뉴스 제목·요약·출처·시각·이미지 |
| `GET /api/v1/lending/revenue/account/top-revenue` | `lending top` | 익명 사용자명·누적 수익·KRW 환산 수익 |
| `GET /api/v1/earning-call/events/{eventId}/info` | `market earnings <event-id>` | 기업·대표종목·보고서 메타데이터, 공개 시점 이후 오디오·대본·발표자료 링크, 컨센서스 괴리·주가 변동 |

2026-09-03 추가 계약도 WTS 정적 호출부와 읽기 전용 라이브 schema를 함께 확인했다.
화면이 있는지 여부는 접근 가능성의 조건으로 쓰지 않았고, 현재 `.tossinvest.com` 세션으로
실제 호출이 확인된 계약만 `source=wts`로 구현했다. TICS 종목·ETF·뉴스는 `{}` 요청에서
서버 기본 첫 페이지를 반환하며 응답의 `totalCount`를 보존한다. 확인되지 않은 페이지 요청
필드를 추측해 전체 목록인 것처럼 표시하지 않는다. `account trading-settings`는 설정값을
읽기만 하고 변경 endpoint를 노출하지 않는다. `account transfer-accounts`도 주식이체 화면의
선택 후보만 조회하며 이체를 시작하거나 계좌 상태를 변경하지 않는다.

같은 감사에서 `r-chart` 함수 정의와 정확한 URL 조립식도 찾았지만 현재 77개 chunk에는
그 export를 소비하는 호출부가 없고, 실제 종목 화면의 일별 차트는 기존 `c-chart`, 실시간
체결은 `stock-prices/.../ticks`를 사용한다. 따라서 range/step enum을 추측해 중복 기능을
만들지 않고 미사용 계약으로 보류했다.

## 쓰기 기능 감사

| Class | Current decision | Evidence needed before expansion |
| --- | --- | --- |
| 주문 place/cancel/amend | 기존 안전 게이트 유지 | preview, config opt-in, execute, confirm token, 사람의 매회 승인 |
| 관심종목 add/remove/group | 기존 구현 유지 | 웹 캡처로 method/body/XSRF 확인됨; 비금융·복구 가능 |
| 목표가 알림 add/remove | 구현 (`quote alert`) | WTS call-site/serializer + list schema; preview/confirm/post-read verification |
| 보유종목 hide/show | 구현 (`portfolio hidden`) | WTS call-site/serializer + account-scoped list schema; 계좌 키 비노출 |
| 소비 숨김 저장 | 구현 안 함 | mobile auth, 사용자 의도, undo/복구 UX |
| 오픈뱅킹 활성화 | 구현 안 함 | consent flow이며 단순 API write가 아님 |
| Home/MyData 계좌 save/update/delete | 구현 안 함 | 별도 인증·동의·영향 범위와 복구 계약 |
| 송금·결제 | 호출·구현 안 함 | 고위험 금융 mutation; 현재 프로젝트/WTS auth 범위 밖 |

새 쓰기 기능은 “endpoint가 존재한다”가 아니라 **정확한 계약 + 인증 소유권 + 사용자에게
보이는 결과 + 실패/복구 + 명시적 승인**이 모두 확인될 때만 후보가 된다.

## 앱 감사 freshness 감시

`docs/reverse-engineering/android-app.json`은 마지막 감사 artifact와 공개 배포 후보를 분리한다.
주간 workflow가 APKPure metadata에서 더 높은 version을 발견하면 `audit_status=stale`로
알리지만, 제3자 metadata를 곧바로 신뢰하거나 artifact를 자동 실행하지 않는다. 새 XAPK/APK는
package id와 위 certificate SHA-256의 연속성을 검증한 뒤에만 이 문서의 감사 버전을 올린다.

### WTS Open API 허용 IP — `verified`, 구현

이 항목은 Android 계약이 아니라 같은 날 현재 WTS 웹 번들 및 실제 설정 화면으로 검증했다.

- 추가: `POST /api/v1/openapi/client/allowed-ips`, body `{"ip":"..."}`
- 삭제: `DELETE /api/v1/openapi/client/allowed-ips/{ip}`
- 목록: `GET /api/v1/openapi/client`의 `result.allowedIps[]`

브라우저에서 이사 전 IP를 삭제하고 현재 공인 IP를 등록한 뒤 공식 OAuth 호출이 `403`에서
정상으로 바뀌는 것까지 확인했다. CLI/MCP는 raw add/delete를 노출하지 않고
`replace-current`라는 상위 작업으로 감쌌다. 기본 호출은 preview이며, 기존 목록과 현재 IP를
묶은 `confirm_token`이 있어야 실행한다. 삭제 후 추가가 실패하면 삭제했던 IP를 다시 등록한다.

## 기존 추측을 줄이는 후속 감사 순서

1. 구현된 WTS endpoint를 카탈로그의 `observed`와 대조한다.
2. host/method/body/response 중 하나라도 근거가 없으면 `partial`로 내린다.
3. monitor probe가 없는 인증 endpoint부터 schema invariant를 추가한다.
4. 모바일 경로는 웹 UI 부재로 추론하지 않고 APK module·host·인증 소유권으로 판정한다.
5. 실제 값·계좌번호·토큰은 감사 산출물에 남기지 않는다.
