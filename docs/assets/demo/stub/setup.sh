# 데모 환경 세팅 (Hide 구간에서 source). 실제 명령 실행 방지.
export PS1='\[\e[38;5;114m\]$\[\e[0m\] '
export PATH="$PWD/stub:$PATH"
# curl 을 가로채 설치 연출만 (파이프 sh 안전)
curl() { :; }
sh()   { printf '  \033[38;5;114m✔\033[0m tossctl v0.26.0 → /usr/local/bin/tossctl\n'; }
# 파이프라인 'curl ... | sh' 는 curl(무동작) → sh(설치 메시지) 로 안전 동작
clear
