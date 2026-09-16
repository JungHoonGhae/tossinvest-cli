# ASC: Android 정적 분석 보조 도구

큰 APK에서 이미 찾은 클래스와 참조를 빠르게 읽는 로컬 도구입니다. 실제 토스 APK 검증은
[2026-09-16 결과](../../docs/reverse-engineering/change-analysis/2026-09-16-asc.md)를 참고하세요.
API 전체 탐색·계약 확정에는 JADX와 원본 DEX 대조를 함께 사용합니다.

## 설치

macOS/Linux, Bash, [uv](https://docs.astral.sh/uv/)가 필요합니다.

```bash
tools/asc/run.sh setup
```

Python 3.12 전용 환경을 `${XDG_DATA_HOME:-$HOME/.local/share}/tossctl-tools/asc`에 만듭니다.
`TOSSCTL_ASC_ENV`로 위치를 지정할 수 있습니다. ASC는 upstream commit, Androguard와
나머지 의존성은 `requirements.txt`에 고정했습니다. 일반 실행은 설치·업데이트하지 않으며,
lock이 바뀌면 명시적으로 `setup`을 다시 실행해야 합니다. Go CLI·MCP 의존성에는 추가하지 않습니다.
ASC가 Python import를 패치하므로 항상 별도 CLI 프로세스로 실행합니다.

## 사용

APK는 [출처·서명 검증 절차](../../docs/reverse-engineering/capture-workflow.md#android-apk-정적-분석-2026-09-02-정립)를 먼저 통과해야 합니다.
다음 예시는 감사한 5.275.0의 클래스명입니다. 난독화 이름은 앱 버전마다 달라질 수 있습니다.

```bash
# 기본값과 Retrofit annotation이 들어 있는 API 클래스
tools/asc/run.sh getclass /path/to/toss.apk o.setRouteDatabaseokhttp
# 요청 serializer: $$를 셸에서 확장하지 않도록 작은따옴표 사용
tools/asc/run.sh getclass /path/to/toss.apk 'im.toss.tosssecurities.core.account.data.model.AllAccountsRequestBody$$serializer'
# 실제 코드의 메서드 참조
tools/asc/run.sh findrefs /path/to/toss.apk method IAuthTabCallback --class o.setRouteDatabaseokhttp
```

원본·추출 파일은 저장소 밖에 둡니다. `-o /path/to/output.java`로 저장할 수 있습니다.
`findrefs ... string /api/...`는 **Retrofit annotation에만 있는 선언을 놓칠 수 있습니다**.
결과가 없거나 일부만 나와도 endpoint가 없다는 뜻은 아닙니다. `getclass`가 endpoint
annotation을 복원해도 메서드 인자와 parameter annotation은 빠질 수 있습니다.

## 반복 검증

검증 스크립트 자체는 Python 3.11+ 표준 라이브러리만 사용합니다. 설치된 ASC를 subprocess로
호출하며 APK를 실행하거나 API에 접속하지 않습니다.

```bash
python3 tools/asc/verify.py /path/to/toss-5.275.0.apk \
  --output /tmp/asc-verification-20260916 --runs 3
# 선택: JADX 설치 시 같은 API 클래스를 새 프로세스로 1회 읽는 시간·출력 비교
python3 tools/asc/verify.py /path/to/toss-5.275.0.apk \
  --output /tmp/asc-verification-with-jadx --runs 3 --jadx
```

출력 경로는 **새 디렉터리이며 저장소 밖**이어야 합니다. 원본 stdout/stderr와 `report.json`이
남습니다. 빈 case/필수 검사, 중복·경로 형태의 case id, 잘못된 정규식은 실행 전에 거부합니다.
해시가 profile과 다르면 실행 전에 중단합니다. `--timeout`(기본 180초)은 각
호출에 적용하며 시간 초과 시 자식 프로세스까지 종료합니다.

- `passed`: 필수 검색 결과와 선언한 추가 coverage 검사 모두 확인.
- `partial`: 필수 탐색은 됐지만 인자·annotation 등 coverage에 누락이 있음. exit 0이어도
  계약 전체가 검증됐다는 뜻이 아니므로 자동화에서는 반드시 `report.json.status`를 확인합니다.
- `failed`: 실행 실패·시간 초과 또는 필수 결과 누락. exit 1.
- `--jadx` 결과는 별도 `jadx_reference`에 실제 exit code와 coverage를 기록하며 ASC 판정에
  합치지 않습니다. JADX도 오류가 있을 수 있어 출력 파일 생성만으로 성공 판정하지 않습니다.

새 앱 버전은 출처·서명을 확인한 뒤 `toss-5.275.0.json`을 복사하여 SHA-256, 클래스명,
필수 문자열·coverage를 갱신하고 `--profile`로 지정합니다. `service` 사례의 `getclass` 대상이
선택적 JADX 비교 대상입니다. 새 profile만 만든다고 `android-app.json`의 감사 버전을 올리지는 않습니다.

## 버전 변경

`requirements.in`의 ASC commit 또는 Androguard 버전을 수정한 뒤 다음 순서로 진행합니다.

```bash
uv pip compile tools/asc/requirements.in --python-version 3.12 \
  --output-file tools/asc/requirements.txt --no-emit-index-url
tools/asc/run.sh setup
# 위 APK 반복 검증으로 결과와 누락 범위가 변했는지 확인
```

CI에는 APK나 ASC 설치가 필요하지 않습니다. `tools/tests/test_android_asc.py`가 잘못된 APK,
빈 성공 응답, 부분 결과, timeout, stale 환경과 인자 전달을 합성 입력으로 검사합니다.
