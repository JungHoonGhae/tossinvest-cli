# Reverse Engineering Notes

토스증권 웹(WTS)의 비공개 API 를 조사한 결과물이다. 공식 Open API 로는 닿지
않는 조회를 `tossctl` 이 제공할 수 있는 근거가 여기에 있다.

## 어디부터 읽나

| 파일 | 내용 |
|---|---|
| **[capture-workflow.md](capture-workflow.md)** | **신규 기능 발굴 실전 플레이북** — 세션 주입 → 번들에서 경로 추출 → 라이브 프로브 → POST 바디 캡처 → 웹UI 유무 판정 → 구현. 새 기능을 붙인다면 여기서 시작한다. "막힌 방법" 절도 함께 읽을 것 |
| [wts-endpoints.json](wts-endpoints.json) | WTS API 전체 카탈로그(구현/후보/의도적 제외). `tools/wts_endpoints.py` 로 갱신, 주간 `wts-monitor.yml` 이 변화를 추적 |
| [android-app.json](android-app.json) | 일반 Toss Android 앱의 마지막 정적 감사 artifact와 최신 공개 후보. 후보가 새로우면 감사 stale로 표시하되 자동 신뢰하지 않음 |
| [rpc-catalog.md](rpc-catalog.md) | 엔드포인트별 요청/응답 형태 메모 |
| [auth-notes.md](auth-notes.md) | 세션·쿠키·인증 헤더 |
| [push-events.md](push-events.md) | 실시간 push 이벤트 |
| [change-analysis/](change-analysis/) | 서버 변경이 관측된 날의 분석 기록 |
| [change-analysis/2026-09-02-android-static.md](change-analysis/2026-09-02-android-static.md) | Android 5.275.0 정적 계약 감사: WTS 교차검증, 은행/MyData 인증 경계, 쓰기 후보 정책 |

## 도구

```bash
# 첫 로드 POST 바디 캡처 (값 마스킹이 기본, 조회 발굴엔 --get)
node tools/capture_post_bodies.mjs /account/profit
node tools/capture_post_bodies.mjs /feed/news --get

# 엔드포인트 카탈로그 갱신 (JS 번들 전수 파싱, stdlib only)
python3 tools/wts_endpoints.py

# 일반 Toss Android 공개 후보와 마지막 감사 버전 비교
ANDROID_DIFF_OUT=/tmp/android_diff.json python3 tools/android_app_monitor.py

# HAR 새니타이즈 · 공개 픽스처 갱신
python3 tools/sanitize_har.py <input.har> <output.har>
python3 tools/fetch_public_fixtures.py fixtures/responses/public
```

Android 결과와 diff의 `metadata_source`는 네트워크 후보 조회를 `live`,
`--metadata-file`을 쓴 fixture 재현을 `offline`으로 구분합니다.

**번들 파싱과 라이브 캡처는 상호보완이다.** `wts_endpoints.py` 는 번들에서 경로를
전수로 뽑지만 호출 형태(바디)는 모르고, 번들에 안 실린 엔드포인트는 못 본다 —
실제로 `profit/readable-tab`, `dashboard/wts/news`, `stock-infos/{code}/red-flags`
가 카탈로그에 없는 채 라이브 캡처로만 발견됐다. 반대로 캡처는 그 화면에서 실제로
발생한 요청만 보여준다. 둘 다 쓴다.

## 데이터 취급

**실제 계좌 데이터는 커밋하지 않는다** — 쿠키·토큰·계좌번호·개인정보가 든 raw
캡처는 어떤 형태로도 남기지 않는다. 테스트 픽스처는 합성 더미로 만들고, 라이브로
검증하되 그 값은 화면에서만 본다. 캡처 도구가 값을 기본 마스킹하는 이유도 같다
(`--raw` 출력은 이슈·PR 에 붙여넣지 말 것).

APK/XAPK와 jadx 결과처럼 용량이 큰 정적 분석 자산은 저장소 안의
`.artifacts/android/toss/<version>/`에 버전별로 둔다. 이 경로는 `.gitignore`와 로컬
`.git/info/exclude`에서 제외되므로 Git에는 올라가지 않지만, 분석 스크립트와 후속 버전 diff가
Downloads 같은 임시 위치에 의존하지 않는다. 공개 문서에는 package/version/hash/서명과
검증된 계약만 남기고, 로컬 분석 메모와 원본 바이너리는 `.artifacts` 안에 보관한다.

용어는 루트 [`CONTEXT.md`](../../CONTEXT.md), 되돌리기 어려운 결정은
[`docs/adr/`](../adr/).
