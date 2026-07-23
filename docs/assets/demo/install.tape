# tossctl 설치→로그인→조회 데모 (vhs)
# 안전: Hide 구간에서 stub/setup.sh 를 source → curl/sh/tossctl 이 전부 스텁으로
# 대체됨. 실제 네트워크·실계좌 접근 0. 화면엔 진짜 명령을 타이핑하는 것처럼 보이되
# 출력은 전부 합성 예시값.
# 생성: cd docs/assets/demo && vhs install.tape

Output install.gif

Set Shell "bash"
Set FontFamily "JetBrainsMonoHangul Nerd Font Mono"
Set FontSize 20
Set Width 1240
Set Height 480
Set Padding 30
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

# 데모 환경 로드 (안 보이게)
Hide
Type "source stub/setup.sh" Enter
Show
Sleep 700ms

# 1) 설치 — curl 한 줄 (실제 설치 URL)
Type "curl -fsSL https://raw.githubusercontent.com/JungHoonGhae/tossinvest-cli/main/install.sh | sh"
Sleep 500ms
Enter
Sleep 1100ms

# 2) 로그인 — QR
Type "tossctl auth login"
Sleep 500ms
Enter
Sleep 1500ms

# 3) 시세 조회 (합성값)
Type "tossctl quote get NVDA"
Sleep 500ms
Enter
Sleep 1300ms

# 4) WTS 전용 — 공식 API 엔 없는 기능
Type "tossctl market briefing"
Sleep 500ms
Enter
Sleep 2600ms
