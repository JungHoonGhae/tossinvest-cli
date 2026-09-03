# ADR 0003 — 기능 영역과 접근 경로를 분리한다

- 상태: 채택
- 날짜: 2026-09-03
- 관련: `internal/ops`, `internal/client`, `internal/official`, 향후 mobile connector

## 맥락

일반 Toss 앱에는 토스증권 화면과 Banking/MyData 화면이 함께 있다. 반대로 일부 증권 기능은
웹 UI가 없어도 WTS API 계약과 웹 세션으로 호출된다. 따라서 화면이 어디에 보이는지, 어떤
제품의 기능인지, 어떤 자격증명으로 호출하는지는 서로 같은 축이 아니다.

이를 하나의 `platform` 또는 `WTS/앱` 구분으로 표현하면 다음 오류가 생긴다.

- 웹 UI가 없는 증권 API를 곧바로 mobile 전용으로 오분류한다.
- 일반 Toss 앱에 있다는 이유로 Banking/MyData API에 증권 WTS 쿠키를 재사용한다.
- 에이전트가 기능 이름만 보고 필요한 로그인과 개인정보·동의 경계를 잘못 판단한다.

## 결정

모든 operation은 두 개의 독립 축으로 설명한다.

- `Domain`: 제품 기능의 소유권. `securities`, `banking`, `mydata`, `system`.
- `Backend` 또는 CLI `source`: 실제 호출 자격증명과 전송 경로. `official`, `wts`,
  `auto`(official/WTS 라우팅), 향후 `mobile`.

웹 UI 유무는 발견과 문서화를 위한 속성일 뿐 backend 판정 기준으로 사용하지 않는다.
WTS는 토스증권 Web Trading System을 뜻하며 일반 Banking의 범용 웹 백엔드가 아니다.

mobile 계약은 APK의 Retrofit binding, base host, app/device 및 session header,
request/response cipher interceptor 소유권을 추적한 뒤에만 mobile 후보로 분류한다. 현재
확인한 일반 Banking/MyData 계약은 `api-gateway.toss.im`의 main-app client에 속하며,
기존 `.tossinvest.com` 세션 connector에 섞지 않는다.

## 결과

- 기존 operation은 기본적으로 `domain=securities`이며 인증·IP·감시는 `system`으로 명시한다.
- `banking_status`라는 기존 이름은 유지하지만 의미는 주식모으기 자금연결 상태이므로
  `domain=securities`, `backend=wts`다.
- 일반 은행·카드·소비 API는 계약이 존재해도 mobile 인증/cipher connector가 구현되기 전까지
  operation으로 노출하지 않는다.
- 향후 mobile connector는 WTS 세션과 별도 secret storage·동의·마스킹 경계를 가져야 한다.

## 재검토 조건

토스가 Banking/MyData용 공식 API를 공개하거나, 동일 자격증명이 여러 host에 사용된다는
명시적 계약을 제공하면 backend 구성은 확장할 수 있다. 제품 domain과 접근 경로를 독립적으로
표현한다는 결정 자체는 유지한다.
