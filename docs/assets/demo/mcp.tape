# tossctl MCP 등록 데모 (vhs)
# 안전: 실제 claude/tossctl 실행 안 함 — stub/claude 가 합성 출력만. 실계좌 0.
# 메시지: "한 줄로 등록하면 AI 에이전트에 3개 툴이 바로 붙는다"
# 생성: cd docs/assets/demo && vhs mcp.tape

Output mcp.gif

Set Shell "bash"
Set FontFamily "JetBrainsMonoHangul Nerd Font Mono"
Set FontSize 20
Set Width 1240
Set Height 400
Set Padding 30
Set Theme "Catppuccin Mocha"
Set TypingSpeed 42ms

Hide
Type "source stub/setup-mcp.sh" Enter
Show
Sleep 700ms

# 1) MCP 등록 — 한 줄
Type "claude mcp add tossctl tossctl mcp"
Sleep 500ms
Enter
Sleep 1300ms

# 2) 붙었는지 확인 — 3개 툴
Type "claude mcp list"
Sleep 500ms
Enter
Sleep 3000ms
