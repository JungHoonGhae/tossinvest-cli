# ASC 기반 고도화 후보 — 2026-09-16

현재 기능과 Android 정적 근거를 대조하면 **실적 발표 내용 조회와 내 종목의 발표 일정**을
우선 검증할 가치가 크다. 이어서 관심종목 폴더별 브리핑과 폴더 간 이동을 검토한다.
아래는 구현 제안이며, 현재 tossctl이 제공한다고 안내할 기능 목록은 아니다.

## 조사 범위와 재현

- 기준 소스: tossinvest-cli `8acc9701a4ba96c02d37971de1be8fe2232cee57`.
- APK: `viva.republica.toss` 5.275.0, SHA-256
  `06715f2d15dd530cb426474e87a4b39aaaf91d1013ef8db171f882d2bf29e2ca`.
  [서명 검증한 기존 분석본](2026-09-02-android-static.md)을 사용했다.
- ASC로 18개 클래스의 service 선언·serializer·enum·응답 모델을 읽고, 같은 APK의 JADX
  추출 소스로 인자와 필드를 대조했다. 앱 실행이나 실서비스·계정 호출은 하지 않았다.
- Android 감사 상태는 여전히 `stale`이다. 이번 분석은 최신 APK 감사나 현재 WTS 세션과의
  호환성 검증을 대신하지 않는다. 호스트·인증·비어 있지 않은 응답은 후속 확인 대상이다.
- 당시 WTS 카탈로그는 구현 165 / 후보 324 / 제외 660, 총 1,149개 경로다.
  후보 경로 수는 신규 기능 수가 아니다. 중복·구버전·기존 기능으로 대체된 경로를 빼야 한다.

원본은 프로젝트의 `.artifacts/android/toss/5.275.0/`에서 관리한다. APK·`jadx/sources/`·
`asc/opportunities-20260916/`은 Git 제외 자산이고, 재현 profile과 이 요약은 소스로 관리한다.
최초 조사 로그에 있는 이전 절대 경로는 당시 실행 기록이며, 재실행은 아래 경로를 사용한다.

```bash
python3 tools/asc/verify.py .artifacts/android/toss/5.275.0/viva.republica.toss.apk \
  --profile tools/asc/toss-5.275.0-opportunities.json \
  --output .artifacts/android/toss/5.275.0/asc/opportunities-verify-20260916 --runs 1
```

같은 출력 디렉터리는 덮어쓰지 않는다. 재실행할 때 날짜·회차를 바꾼다.
이 profile의 성공 표식은 정적 문자열·필드 검사다. 기능의 라이브 지원 여부는 판정하지 않는다.
service 인자 누락과 한글 문자열 손실을 coverage로 검사하므로 현재 ASC 결과는 `partial`이다.
재현 profile의 18개 작업 모두 필수 근거를 확인했고, 네 service의 parameter annotation과
한글 라벨 검사 1개는 누락으로 남았다. 경로 허용·소스 경로 탈출 차단·기존 결과 보존을 포함한
Python 테스트 120개가 통과했다.

## 우선순위

| 순위 | 제안 | 사용자에게 생기는 변화 | 남은 확인 |
| --- | --- | --- | --- |
| 1 | 실적 발표 원문·번역·요약·관전 포인트 | 자료 링크를 연 뒤 직접 찾던 내용을 CLI·MCP에서 읽고, 발언 시점과 출처로 돌아갈 수 있다 | 실제 event ID의 응답, 번역·요약 미생성 상태, 호스트·인증 |
| 2 | 내 투자·관심 종목의 실적 발표 일정 | 전체 일정에서 찾지 않고 내 종목의 예정·지난 발표를 시간순이나 인기순으로 조회한다 | v2 경로의 현재 지원, 필터의 정확한 대상, 정렬·목록 제한 |
| 3 | 관심종목 폴더별 브리핑 | 반도체·배당주 등 선택한 폴더에 맞춰 뉴스와 관련 종목을 모아 본다 | `watchlistId` 적용 결과, 빈 폴더·권한·응답 항목 |
| 4 | 관심종목 여러 개를 폴더 간 이동 | 기존 폴더에서 제거하고 다른 폴더에 추가하던 작업을 하나의 검토 가능한 변경으로 다룬다 | 전체 요청 계약, 중복·다중 소속 의미, 변경 전후 검증 |
| 기반 개선 | APK 계약 변경 비교 | 앱 업데이트 때 어느 기능의 경로·필드·enum을 재검증해야 하는지 바로 파악한다 | annotation 추출 범위, 버전 간 난독화 클래스 대응 |

### 1. 실적 발표 내용까지 조회

현재 `market earnings <event-id>`와 `earning_call_detail`은 발표 메타데이터와 오디오·
스크립트·슬라이드 URL을 제공한다. 별도 문단 내용이나 관전 포인트 응답은 읽지 않는다.
근거: `internal/client/marketdata.go`의 `GetEarningCallDetail`,
`internal/domain/models.go`의 `EarningCallDetail`.

APK의 `o.RealConnectionCompanion`에서 다음 경로와 응답 타입을 확인했다.

| 경로 | 정적 응답 근거 |
| --- | --- |
| `/api/v1/company-events/{eventId}/transcripts/paragraph-inferences` | `ParagraphInferenceResponse`: `elapsedStartSeconds`, `elapsedEndSeconds`, `paragraphId`, `originalText`, 선택적 `translated`·`summarized` |
| `/api/v2/company-events/{eventId}/report` | `ReportsResponse`: `analysis`, `watchPoint`; 분석의 `overall`, `pros`, `cons`; 관전 포인트의 `points`, `sources`, `createdAt`, `updatedAt` |
| `/api/v1/company-events/{eventId}/preview/content` | `PreviewContentResponse`: `latestEvents`, `latestNews`, `researchCenterArticles`, `watchPoints` 등 |

번역·요약이 없으면 생성 대기·미제공을 구분해 표현할 계약이 필요하다. 내용이 있다는 가정으로
빈 값을 완성된 요약처럼 출력하면 안 된다. 기존 카탈로그에서 문단·report 경로의 과거 probe가
404였으므로, 당시 결과만으로 영구 미지원으로 보거나 이번 정적 분석만으로 지원으로 바꾸지 않는다.
실제 일정에서 얻은 event ID와 해당 클라이언트의 호스트를 먼저 확인한다.

실시간 SSE 번역 DTO도 APK에 있지만 이번 profile은 정적 내용 조회를 우선한다.
실시간 연결·재연결·메시지 순서까지 검증된 기능으로 확대 해석하지 않는다.

### 2. 개인화된 예정·지난 발표

현재 `market earnings`는 v1 `upcoming`, `--major`는 v1 `home`을 읽는다.
`portfolio briefing`은 보유 종목과 전체 예정 발표를 로컬에서 매칭한다.
서버의 투자·관심 필터, 지난 발표 목록, 정렬 선택은 노출하지 않는다.

- service: `o.setSettings`.
- 경로: `/api/v2/earning-call/home`, `/api/v2/earning-call/events/current-or-future`,
  `/api/v2/earning-call/events/past`. 기준 WTS 카탈로그에는 이 세 경로가 없었다.
- JADX parameter annotation: 목록 조회의 `maxSize`, `sort`, nullable `filter`.
- `o.ackSettings`: 정렬 wire value `POPULAR`, `TIME`.
- `o.Http2ConnectionsendDegradedPingLaterinlinedexecutedefault1`: `USER_BASED` 필터 매핑.
  `o.getReaderokhttp`의 UI 라벨은 `전체`, `내 투자·관심`이다.
- `EarningCallHomeV2Response`: `currentOrFuture`, `past`, `rankings`.
  `EarningCallEventSummaryResponse`: `userBaseContentText`, `watchPointKeywords` 등.

`USER_BASED`를 곧바로 보유 종목 전용 필터로 이름 붙이지 않는다. 관심 종목도 포함하는 UI
근거가 있으므로 기존 보유 종목 브리핑에 합칠 때 포함 범위를 검증해야 한다.

### 3. 폴더별 뉴스와 관련 종목

현재 `market news --scope watchlist`는 관심종목 뉴스 전체를 지원한다.
`watchlist`의 폴더 조회도 이미 있다. 추가 가치는 **선택한 폴더 기준의 뉴스·관련 종목 조회**다.

- service: `o.NioFileSystemWrappingFileSystemExternalSyntheticApiModelOutline4`.
- 상대 경로 `v1/new-watchlists/recommend/news`, `v1/new-watchlists/recommend/tics`.
- JADX에는 두 메서드 모두 nullable `watchlistId` query가 있다. 상수의 실제 값은
  `TossSecRoute.Main.PARAM_WATCHLIST_ID = "watchlistId"`로 대조했다.
- `RecommendedNewsSection`: `title`, `description`, `pageSize`, `newsList`.
  `RecommendedProductSection`: `title`, `products` 등.

APK에는 `/api/` 없는 상대 경로로 선언돼 있다. 기존 WTS 카탈로그의 `/api/v1/...` 경로와
대응하지만 base URL을 읽기 전에는 Android host까지 확정하지 않는다.

### 4. 여러 관심종목의 폴더 이동

현재 `internal/watchlist/service.go`에는 폴더 생성·이름 변경·삭제와 종목 추가·삭제가 있다.
여러 종목의 폴더 간 이동 작업은 없다.

- `o.concat`: `v1/new-watchlists/items/move-groups`와
  `NewMoveWatchListItemGroupRequest` body 타입.
- 해당 serializer: 필수 `items`, `fromWatchlistIds`, `toWatchlistIds`.
- JADX 모델: `items`는 `List<NewWatchListItemCodeRequest>`, 두 ID 필드는 `List<Long>`.
- 항목 serializer의 `code`, `itemType`은 확인했지만, 추가 선택 필드 하나는 문자열이
  난독화돼 있다. 전체 요청 계약이 확정된 것은 아니다.

구현하면 기존 preview·confirm 흐름을 확장하되 출발·도착 폴더 양쪽 상태와 항목 목록에
확인 토큰을 결합해야 한다. 이동 후 양쪽을 다시 읽어 결과를 검증한다.
이번 조사는 실제 관심종목을 변경할 승인이 아니므로 쓰기 호출은 하지 않았다.

### 기반 개선: annotation과 schema를 함께 비교

ASC가 빨라진 부분은 이미 찾은 클래스의 내용을 읽는 작업이다. 자동화의 다음 단계는
`경로 → service → 요청/응답 serializer → CLI·probe 의존 기능`의 대응표를 만드는 것이다.
새 APK에서 경로뿐 아니라 필드 추가·삭제, nullable, enum 변경을 비교하면 조사 우선순위를
자동으로 좁힐 수 있다. 난독화 클래스 이름 자체를 버전 간 고정 키로 삼으면 안 된다.

현재 `findrefs string`은 annotation에만 있는 경로를 놓치고 `getclass`는 메서드 인자를
생략한다. 이번에는 한글 라벨 `내 투자·관심`도 ASC에서 일부 글자만 남았고, JADX에는 전체
라벨이 보존됐다. 따라서 annotation 인덱스·원본 DEX/JADX 교차검증 없이 ASC 출력만으로
카탈로그 승격·HTTP 클라이언트 생성·한글 문구 추출을 자동화하지 않는다.

## 후속 작업 경계

우선 1·2번을 묶어 정확한 host·인증·현재 지원을 읽기 전용으로 검증한다. 그 뒤 응답 상태를
보존하는 domain 모델, CLI·MCP, 합성 계약 테스트, `ProbeSpec`, 사용자 문서를 함께 구현한다.
정적 근거를 발견했다는 이유로 WTS 카탈로그의 구현 상태나 Android 감사 버전을 올리지 않았다.

권리 행사·자동 환전·주문 내역 추가 필터도 조사했으나 이번 ASC 대조에서는 위 후보만큼
새로운 계약 근거를 확보하지 못했다. 이들을 즉시 개발 가능한 기능 수에 합산하지 않는다.
