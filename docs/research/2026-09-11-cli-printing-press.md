# CLI Printing Press에서 가져온 설계

참고: [CLI Printing Press v4.32.1](https://github.com/mvanhorn/cli-printing-press/tree/v4.32.1).
본체와 같은 버전의 Codex 스킬을 설치하고 바이너리·스킬 호환성을 확인했다.

적용한 범위는 세 가지다.

1. SQLite에 명시적인 관측을 저장하고 오프라인으로 검색·비교한다.
   `internal/history`가 저장과 확인 토큰을 소유하며, CLI와 ops/MCP가 같은 서비스를 호출한다.
   로컬 관측의 범위·수집 시각·불완전성을 결과에 남긴다. 모든 수집본은 독립적이다.
2. `portfolio briefing`이 보유 종목·예정 어닝콜·보유 뉴스·미체결 주문을 합친다.
   `internal/briefing`이 식별자 매칭과 부분 실패를 처리한다. API 의존성은 공용 probe로 감시한다.
3. 공용 JSON 직렬화에 `--fields`와 `--compact`를 추가한다. MCP는 `call_operation.fields`로
   같은 선택 규칙을 사용한다. 기본 table 출력과 기존 JSON 필드는 유지한다.

기존 ADR 0001에 따라 typed CLI를 `ops.Call`로 우회시키지 않는다. 저장용 SQLite는
`modernc.org/sqlite`의 pure-Go driver를 사용해 기존 CGO 없는 배포 경로를 유지한다.
명시적 로컬 수집은 서버의 portfolio snapshot 조회와 별개이며, 주문 검증·매수 가능 금액·
시세 라우팅에는 캐시를 연결하지 않는다.

Printing Press의 `scorecard --dir`도 실행했지만 `cmd/tossctl`과 자체 3-tool MCP 카탈로그를
생성기 표준 구조로 인식하지 못해, 이미 존재하는 JSON·doctor·MCP에도 0점을 부여했다.
그 점수는 이 프로젝트의 품질 지표로 사용하지 않는다. 검증은 실제 공용 서비스,
CLI/MCP 표면, DB 동시 쓰기·확인 토큰·오프라인 동작, 기존 회귀 테스트로 수행한다.

사용법: [로컬 이력 가이드](../../website-fumadocs/content/docs/guide/history.mdx).

검증 환경과 결과:

- 저장소의 최소 버전인 Go 1.25.13에서 전체 테스트, lint, 변경된 패키지의 race 검사,
  govulncheck를 통과했다.
- CGO 없이 macOS arm64/amd64, Linux arm64/amd64, Windows amd64를 빌드했다.
  Windows 경로는 [SQLite의 파일 URI 규칙](https://www.sqlite.org/uri.html)에 맞춰 변환한다.
  실행 검증은 macOS에서 수행했으며 다른 플랫폼은 크로스 빌드로 확인했다.
- 한·영 문서 빌드와 문서 경로 검사를 통과했다. 로컬 Node는 26.7.0으로,
  문서 프로젝트가 지정한 24.x와 달라 엔진 경고가 있었다.
- 실계정의 WTS 세션은 live check에서 401을 반환했다. 실제 계정으로 새 수집·브리핑을
  검증하지 못했으며, 서비스 fixture와 실제 SQLite를 연결한 CLI/MCP 테스트로 검증했다.
