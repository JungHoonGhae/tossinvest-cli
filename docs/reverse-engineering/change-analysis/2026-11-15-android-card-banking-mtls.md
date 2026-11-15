# Android 카드·은행 조회 계약 + mTLS 전송 심화 — 2026-11-15

[2026-09-02 Android 정적 계약 감사](2026-09-02-android-static.md)의 후속 심화편이다.
그 문서는 "카드 결제 내역·이번 달 소비·은행 계좌 조회 계약은 앱에 존재하지만 Toss
Home/main client 소속이라 현재 `.tossinvest.com` WTS 세션으로 인증할 근거가 없어 구현하지
않았다"까지 확정했다. 이 문서는 **(1) 왜 호출이 막히는가 = mTLS 전송 메커니즘**과 **(2)
카드/소비 조회의 응답 필드 전체**, **(3) 호출 가능하게 만들려면 무엇을 추출해야 하는가**를
추가한다.

**이 영역은 여전히 정적 인벤토리다. 호출 가능 커맨드로 구현하지 않았고, WTS 커넥터에 섞지
않는다.** 실계좌·카드 데이터는 한 건도 수집·커밋하지 않았으며 아래는 전부 API 구조(경로·
필드명)다.

## 분석본

[2026-09-02 문서](2026-09-02-android-static.md)와 **같은 서명 검증 완료 artifact**
(`viva.republica.toss` 5.275.0, base APK SHA-256 `06715f2d…e2ca`, v2/v3 verified)의 JADX
산출물(`.artifacts/android/toss/5.275.0/jadx/`)을 더 깊게 읽었다. 새 다운로드·실행·계정
접근은 없다. 공개 후보 5.284.0은 아직 서명 검증 전이라 `android-app.json`은 `stale`로 둔다.

## 1. 카드 조회 계약 (사용자 목표: 카드 사용내역·카드 종류·소비 패턴)

Retrofit 인터페이스 `o/BasicWebSocketContext.java`
(= `im.toss.global...features.home` 조회 인터페이스). 난독화된 주석의 의미:

| 난독 심볼 | Retrofit 의미 |
| --- | --- |
| `@dc1(onNavigationEvent="…")` | HTTP 메서드 주석 — 사용 맥락상 **GET**(쓰기 `changePlaCardType`는 `@dc4`) |
| `@aeuycx(onWarmupCompleted="…")` | `@Path("…")` |
| `@oby1(onNavigationEvent="…")` | `@Query("…")` |
| 반환 `BaseApiResponse<T>` | `im.toss.network.model.BaseApiResponse` envelope |

> 전송 메모: 일부 라우트는 `@DeviceUtilsAudioInfoReceiver(IAuthTabCallback={"X-Toss-Method:GET"})`
> 헤더를 추가한다(`asset-sync/{jobId}`에서 관측). 게이트웨이가 실제 메서드를 `X-Toss-Method`
> 헤더로 터널링할 수 있으니, 빌드 시 라우트별로 실제 wire 메서드를 확인한다.

### 카드 엔드포인트

| Method | Path | 응답 모델 | 용도 |
| --- | --- | --- | --- |
| GET | `/api/v3/home/card/{cardId}/transactions?paginationKey=` | `PagedCreditCardTransaction` | **카드 사용내역(페이지네이션)** |
| GET | `/api/v3/home/card/{cardId}/transactions/{transactionId}/detail` | `TransactionDetail` | 건별 상세 |
| GET | `/api/v3/home/card/{cardId}/card-bill/overview` | `CardBillOverview` | 카드 청구서 |
| GET | `/api/v3/home/card/{cardId}/overview-with-transactions` | `CreditCardOverviewWithTransactions` | 카드 개요 + 최근거래 |
| GET | `/api/v3/benefit-tab/v2/cards` · `/api/v3/benefit-tab/cards` | (별도) | 혜택 탭 카드 목록 |

### 응답 필드 (serializer `$$serializer.java`의 `addElement` wire 이름)

- `PagedCreditCardTransaction` = `txs: List<CreditCardTransactionItem>`, `nextPaginationKey?`,
  `searchStartYearMonth`, `searchEndYearMonth`
- `CreditCardTransactionItem` = `id`, `title: TextContent`, `description: TextContent`,
  `amount: TextContent`, `detailLink: String`
- `CreditCardOverviewWithTransactions` = `overview: CreditCardOverview`, `txsWithPaginationInfo`
- `CreditCardOverview` = `cardName`, `cardImage`, `maskedCardAddress`, `availableCredit`,
  `usedCredit`, `totalCredit`, `upcomingCardBillInfo`
- `CardBillOverview` = `closingBalance`, `cardImage`, `dueDate`, `minimumDue`, `totalCredit`
- `UpcomingCardBillInfo` = `amount`, `cardBillLink`, `logParams?`
- `TransactionDetail` = `id`, `amount`, `detailProperties: List<{label, …}>`

**핵심 설계 사실 — 거래 데이터는 "서버 렌더링 텍스트"다.**
`TextContent`(`im.toss.global.features.home.domain.model.TextContent`) = `{ text, textAlt?,
logText? }`. 즉 사용내역 항목의 `title`(가맹점명)·`amount`(표시 금액)·`description`(날짜/부가)은
**정규화된 수치/카테고리 코드가 아니라 앱 화면에 보이는 문자열**이다. 소비 패턴 분석을 하려면
`text` 값을 파싱해야 한다(가맹점 분류·MCC 같은 구조화 필드는 이 조회 API에 없음).

## 2. 카드 종류·식별 (cardId / cardName 출처)

위 엔드포인트는 모두 `{cardId}`를 입력으로 받는다. cardId와 카드 정체성은 홈 자산 목록에서 나온다:

- 홈 자산 목록 항목 `AssetHomeOverviewView.AssetListItem.CreditCard` = `id`(=cardId),
  `availableCredit`, `logParams` — `GET /api/v3/home/asset-home/overview` (반환 `JsonElement`,
  서버 주도 UI)
- 카드명·이미지·한도는 `CreditCardOverview`(위 §1)의 `cardName`/`cardImage`/`*Credit`
- 홈 메인 섹션 `GlobalHomeMainOverview.CreditCardSection.CreditCardItem` = `id`, `status`,
  `availableCreditPercent`, `cardBillDueDate`, `innerCells`

즉 "내 카드 종류"는 asset-home overview → 각 cardId → `overview-with-transactions`의
`CreditCardOverview.cardName`으로 조립한다.

## 3. 소비 패턴 (월별 현금흐름)

| Method | Path | 응답 모델 |
| --- | --- | --- |
| GET | `/api/v3/home/cash-flow/overview?yearMonth=` | `MonthlyCashFlowOverview` |

`MonthlyCashFlowOverview` = `dailySpendingAmounts`, `dailySpendingAmountsPreviousMonth`,
`dailyIncomeAmounts`, `dailyIncomeAmountsPreviousMonth`(각 `List<DailyAmount{date, amount}>`),
`spendingTransactions`, `incomeTransactions`(각 `List<DailyTransactions{date, txs}>`,
`txs: List<CashFlowTransactionItem{id, amount, detailLink}>`), `currentYearMonth`,
`minNavigableYearMonth`.

**소비 패턴엔 이 엔드포인트가 최적이다** — 일별 지출/수입 합계(전월 대비 포함)가 수치 시계열로
오고, 일자별 거래까지 묶여 있다. 카드 단건 내역보다 집계가 깔끔하다. (또한 mydata 계열
`POST v3/home/mydata-home/consumption/overview/...`, `GET /api/v3/home/consumption/info`는
2026-09-02 문서 참조 — MyData 동의 경계.)

## 4. 은행 계좌 조회 계약 (은행)

같은 `o/BasicWebSocketContext.java` 인터페이스, 호스트/인증 동일.

| Method | Path | 응답 모델 |
| --- | --- | --- |
| GET | `/api/v3/home/account/{accountId}/transactions?paginationKey=` | `PagedAccountTransaction` |
| GET | `/api/v3/home/account/{accountId}/transactions/{transactionId}/detail` | `TransactionDetail` |
| GET | `/api/v3/home/account/{accountId}/trans-overview-with-transactions` | `TransactionOverviewWithTransactions` |
| GET | `/api/v3/home/account/{accountId}/savings-overview-with-transactions` | `SavingsOverviewWithTransactions` |
| GET | `/api/v3/home/account/{accountId}/savings-yearly-interest?year=` | `YearlyReportInfo` |
| GET | `/api/v3/home/account/{accountId}/loan-overview-with-transactions` | `LoanOverviewWithTransactions` |
| GET | `/api/v3/home/account/{accountId}/mortgage-overview-with-transactions` | `MortgageAccountOverviewWithTransactions` |
| GET | `/api/v3/home/account/{accountId}/offset-overview-with-transactions` | `OffsetAccountOverviewWithTransactions` |
| GET | `/api/v3/home/account/{referenceId}/nickname` | — |

- `PagedAccountTransaction` = `txs: List<AccountTransactionItem>`, `nextPaginationKey?`,
  `searchStartYearMonth`, `searchEndYearMonth`
- `AccountTransactionItem` = `id`, `amount: TextContent`, `balanceAfter`, `detailLink`
  (카드 항목과 달리 **거래 후 잔액**이 있다)
- `*OverviewWithTransactions` = `{ overview, txsWithPaginationInfo }`

MyData 자산 통합(`/api/v1/finance-mydata/users/asset`, `…/irc/*` 동의 흐름)과 계좌 변경
(`/api/v1/account/change-account`)은 별도 MyData/모바일 인증 경계로 정적 인벤토리에만 둔다.

## 5. 전송 보안 = 호출의 벽 (왜 WTS처럼 쉽지 않은가)

WTS(증권)는 **웹 세션(쿠키)** 재사용이라 `tools/capture_post_bodies.mjs`로 바로 잡혔다.
일반 토스 앱 카드/은행 API는 네이티브 전용이고 전송이 다층이다.

| 레이어 | 실체 | 정적 근거 |
| --- | --- | --- |
| 호스트 승격 | `service.toss.im` → **`mtls-service.toss.im`** (pay는 `service-pay`→`mtls-service-pay`) | `o/zzn.java` = `im.toss.mtls.impl.network.MtlsHostMapping` |
| 클라이언트 인증 | **mTLS 클라이언트 인증서** (per-device, CSR→cert 발급) | `im.toss.mtls.impl.*`, `IssueCertificateRequestDto{appType, csr}` → `CertificateDto{pemCertificate, startValidTs, endValidTs}` |
| 키 생성 | **소프트웨어 EC 키페어**(1-arg `KeyPairGenerator.getInstance`, provider 없음) + BouncyCastle `JcaPKCS10CertificationRequestBuilder` CSR | `o/zzeb.java` |
| 키 저장 | prefs 키 `toss_mtls_private_key_pkcs8_base64`(PKCS#8 Base64), `toss_mtls_pem_certificate_v2` | `o/zzek.java`, `o/zzen.java`(`generatePrivate(PKCS8EncodedKeySpec(...))`) |
| at-rest 암호화 | AndroidKeyStore AES alias `toss_mtls_alias_private_pki`로 래핑 추정 | `o/zzee.java`(`KeyStore.getInstance("AndroidKeyStore")`, alias 관리/삭제) |
| 응답 무결성 | `X-Toss-Signature` = **SHA512withRSA**, 앱이 pinned 공개키로 **검증**(수신측) | `o/MaxAdViewImplExternalSyntheticLambda1.java:1157` |
| 요청 헤더 | `X-Toss-RequestId`, `X-Toss-RequestTs`, `X-Toss-App-Version`, `X-Toss-OS`, `X-Toss-Signature`(소문자 `x-toss-signature`도 존재), `X-Toss-Content-Encoding` | 헤더 리터럴 전수 |

**판정:** 개인키가 하드웨어 백업 비추출형(AndroidKeyStore/StrongBox 서명 전용)이 아니라
**소프트웨어 바이트(PKCS#8)로 저장**되므로 **이론상 추출 가능**하다. 단, 추출은 **루팅된 +
토스 로그인된 기기에서만** 된다(prefs가 하드웨어 AES로 래핑됐다면 오프디바이스 복호는 불가 →
기기에서 Frida로 로드된 PKCS#8 바이트를 훅). 토스는 루팅/에뮬 로그인을 Play Integrity로
차단하므로 Magisk DenyList 등 우회 후 로그인·덤프가 필요하다. (MITM 재패키징 경로는 핀닝+
Integrity로 막힘 — `capture-workflow.md` "막힌 방법" 참조.)

## 6. 호출 가능하게 만드는 경로 (기기 자료가 생기면)

루팅·로그인된 기기에서 아래를 덤프하면 오프디바이스 클라이언트 구성이 가능하다:

1. mTLS 클라이언트 인증서 PEM — prefs `toss_mtls_pem_certificate_v2`
2. mTLS 개인키 — prefs `toss_mtls_private_key_pkcs8_base64`(래핑됐으면 Frida로 복원된
   `PrivateKey` 바이트 훅: `KeyFactory.generatePrivate` 반환값)
3. 세션/인증 토큰 + 필요한 `X-Toss-*` 헤더(App-Version/OS/RequestId/RequestTs 생성 규칙)
4. 응답 검증용 pinned RSA 공개키(선택 — 수신 검증만)

빌드 계획(구현 시):

- `internal/client`에 **toss-app 클라이언트** 신설 — `mtls-service.toss.im` 대상, 덤프한 클라이언트
  인증서로 mTLS(`tls.Certificate`), `X-Toss-*` 헤더 세팅, 라우트별 메서드(필요 시 `X-Toss-Method`
  터널링) 처리. 기존 WTS `client.Config`(wts-api/info/cert 3호스트)와 **분리**한다.
- `internal/domain`에 `PagedCreditCardTransaction` 등 모델, `internal/output`에 표시(TextContent
  `text` 파싱).
- `internal/ops`에 **읽기 전용** 오퍼레이션 등록 → tossctl MCP `call_operation`으로 LLM 호출 가능:
  `card_list`(asset-home→cardName), `card_transactions`, `card_bill`, `cashflow_overview`,
  `account_transactions`.
- 계약 테스트는 httptest + **더미 데이터**. 실카드/계좌 값은 화면에서만 본다.

## 7. 경계·정책

- **읽기 전용만 대상.** 카드/은행 **쓰기**(송금·결제·`changePlaCardType`·계좌 저장/삭제·
  MyData 동의)는 이 계획에서 제외. 금융 mutation은 정적 분석만으로 구현하지 않으며, 구현하더라도
  preview→config opt-in→`--execute`→confirm token + 매 호출 사람 승인 설계가 선행한다.
- toss-app 세션을 WTS 호스트에, WTS 세션을 toss-app 호스트에 교차 사용하지 않는다.
- 공개 문서·테스트·로그·PR에 실계좌/카드 값 금지. 이 문서의 모든 내용은 API 구조다.

## 재현 (JADX 산출물 경로)

- 조회 인터페이스: `.artifacts/android/toss/5.275.0/jadx/sources/o/BasicWebSocketContext.java`
- 주석 정의: `o/dc1.java`(메서드), `o/aeuycx.java`(@Path), `o/oby1.java`(@Query)
- mTLS: `o/zzn.java`(호스트 매핑), `o/zzeb.java`(keygen+CSR), `o/zzen.java`(키 로드),
  `o/zzek.java`·`o/zzee.java`(저장/keystore), `im/toss/mtls/impl/network/model/*`
- 모델 wire 필드: 각 `…/model/<Name>$$serializer.java`의 `addElement` 호출
- 응답 서명 검증: `o/MaxAdViewImplExternalSyntheticLambda1.java`
