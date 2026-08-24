#!/usr/bin/env python3
"""미확인 candidate 엔드포인트를 실제로 한 번씩 찔러보고 결과를 카탈로그에 기록한다.

왜 필요한가
-----------
`wts_endpoints.py` 는 번들에서 경로를 뽑아 `candidate` 로 쌓기만 한다. 주간 모니터는
**델타(신규·삭제)만** 보고하므로, 처음 쌓인 날 눈에 안 띈 경로는 영영 안 열어보게 된다.
2026-08-03 기준 candidate 358개 중 346개가 카탈로그 생성일(6-19)부터 45일째 미확인이었다.

이 도구는 그 백로그를 4등급으로 갈라 "다음에 뭘 볼지" 를 정할 수 있게 한다.

왜 CLI 가 아니라 tools/ 인가
---------------------------
카탈로그(`docs/reverse-engineering/wts-endpoints.json`)는 레포 파일이라 설치된 tossctl
바이너리에는 없다. 세션은 tossctl 이 저장한 것을 그대로 읽는다.

안전
----
- **GET 을 먼저 보내고, 405 일 때만 POST `{}` 로 재시도한다.** 토스는 조회에도 POST 를
  자주 쓴다(`profit/overview`, `dashboard/wts/news`, `calendar/monthly` 가 그렇다).
  GET 만 보내면 그 조회들이 통째로 `http-405` 로 사장된다 — 2026-08-03 첫 스윕에서
  34개가 그렇게 묻혔고 그중 하나가 월간 증시 캘린더였다.
  **POST 는 `{}` 빈 바디로만, 405 를 받은 경로에만 보낸다.** 쓰기를 유발하지 않기
  위해서다: 쓰기 엔드포인트는 GET 에 405 가 아니라 보통 401/403 이나 400 을 준다.
- **응답 본문을 저장하지 않는다.** 실계좌 데이터가 카탈로그에 새는 걸 막기 위해
  상태코드와 본문 크기 구간만 남긴다.
- 동시 요청을 제한하고 사이에 쉰다 — 남의 서비스다.

사용
----
    python3 tools/probe_candidates.py            # 미확인 candidate 전부
    python3 tools/probe_candidates.py --limit 40 # 40개만
    python3 tools/probe_candidates.py --recheck  # 이미 확인한 것도 다시
"""

from __future__ import annotations

import argparse
import concurrent.futures
import datetime
import json
import os
import re
import sys
import threading
import time
import urllib.error
import urllib.request

CATALOG = "docs/reverse-engineering/wts-endpoints.json"

# 토스는 호스트를 셋으로 나눠 쓴다. 경로만 보고 호스트를 짐작하면 403/404 로 헤매므로
# wts-api 를 먼저 보고, 404 일 때만 나머지를 시도한다 (요청 수를 3배로 늘리지 않기 위해).
HOSTS = [
    "https://wts-api.tossinvest.com",
    "https://wts-info-api.tossinvest.com",
    "https://wts-cert-api.tossinvest.com",
]

UA = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 "
    "(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

# 경로에 자리표시자가 있으면 그대로 부를 수 없다. 찔러봐야 404 라 요청을 아낀다.
TEMPLATED = re.compile(r"[\[{<]")

CONCURRENCY = 4
PAUSE_SEC = 0.15


def session_cookie_header() -> str:
    """tossctl 이 저장한 세션을 Cookie 헤더 한 줄로 만든다."""
    base = os.environ.get(
        "TOSSCTL_SESSION_FILE",
        os.path.expanduser("~/Library/Application Support/tossctl/session.json"),
    )
    if not os.path.exists(base):
        sys.exit(f"세션 파일이 없다: {base}\n먼저 `tossctl auth login` 을 실행할 것.")
    with open(base, encoding="utf-8") as fh:
        sess = json.load(fh)
    cookies = sess.get("cookies") or {}
    if not cookies:
        sys.exit("세션에 쿠키가 없다. `tossctl auth login` 을 다시 실행할 것.")
    return "; ".join(f"{k}={v}" for k, v in cookies.items())


def classify(status: int, size: int) -> str:
    """상태코드와 본문 크기를 사람이 다음 행동을 정할 수 있는 등급으로 바꾼다."""
    if status == 200:
        # 200 이어도 `{"result":null}` 이나 빈 배열이면 볼 게 없다. 오늘 rights/us
        # categories 가 정확히 그랬다 — 200 인데 body 가 늘 빈 배열이라 구현 불가.
        return "worth-review" if size > 120 else "thin"
    if status in (400, 422):
        return "needs-params"
    if status in (401, 403):
        return "forbidden"
    if status == 404:
        return "not-found"
    return f"http-{status}"


def _request(url: str, cookie: str, method: str) -> tuple[int, int]:
    """(status, body_size) 를 돌려준다. 예외는 호출부가 처리한다."""
    headers = {"User-Agent": UA, "Cookie": cookie, "Accept": "application/json"}
    data = None
    if method == "POST":
        data = b"{}"
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return resp.status, len(resp.read(4096))
    except urllib.error.HTTPError as err:
        try:
            return err.code, len(err.read(4096))
        except Exception:
            return err.code, 0


def probe_one(path: str, cookie: str, known: dict | None = None) -> dict:
    if TEMPLATED.search(path):
        return {"verdict": "templated", "note": "경로에 자리표시자가 있어 그대로 호출 불가"}

    # 카탈로그가 번들 삼중에서 얻은 호스트를 알고 있으면 **그것만** 쓴다. 호스트를
    # 순회하며 추측하면 틀린 호스트의 404 를 정답으로 기록하게 된다 — 2026-08-03
    # 스윕이 그렇게 33건을 위양성 not-found 로 사장시켰다.
    hosts = HOSTS
    if known and (h := known.get("host")):
        hosts = [f"https://{h}.tossinvest.com"]

    last = None
    for host in hosts:
        try:
            status, size = _request(host + path, cookie, "GET")
            method = "GET"
        except Exception as exc:  # 네트워크 실패는 판정이 아니다
            last = {"verdict": "error", "note": type(exc).__name__}
            continue

        # 405 = "이 경로는 있는데 GET 이 아니다". 토스는 조회에도 POST 를 쓰므로
        # 여기서 멈추면 멀쩡한 조회를 놓친다. 빈 바디로만 재시도한다.
        if status == 405:
            try:
                status, size = _request(host + path, cookie, "POST")
                method = "POST"
            except Exception:
                pass

        last = {
            "verdict": classify(status, size),
            "status": status,
            "method": method,
            "host": host.split("//")[1].split(".")[0],
        }
        # 404 면 호스트를 잘못 골랐을 수 있으니 다음 호스트로. 그 외는 확정.
        if status != 404:
            return last
        time.sleep(PAUSE_SEC)
    return last or {"verdict": "error", "note": "no host answered"}


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--limit", type=int, default=0, help="찔러볼 개수 상한 (0=전부)")
    ap.add_argument("--recheck", action="store_true", help="이미 기록된 것도 다시 확인")
    ap.add_argument("--dry-run", action="store_true", help="대상만 세고 요청하지 않음")
    args = ap.parse_args()

    with open(CATALOG, encoding="utf-8") as fh:
        catalog = json.load(fh)
    endpoints = catalog["endpoints"]

    targets = [
        p
        for p, meta in sorted(endpoints.items())
        if meta.get("status") == "candidate" and (args.recheck or "probe" not in meta)
    ]
    if args.limit:
        targets = targets[: args.limit]

    print(f"candidate 중 대상 {len(targets)}개")
    if args.dry_run or not targets:
        return

    cookie = session_cookie_header()
    today = datetime.date.today().isoformat()
    lock = threading.Lock()
    done = [0]

    def work(path: str) -> None:
        result = probe_one(path, cookie, endpoints.get(path))
        result["at"] = today
        with lock:
            endpoints[path]["probe"] = result
            done[0] += 1
            if done[0] % 25 == 0:
                print(f"  … {done[0]}/{len(targets)}")
        time.sleep(PAUSE_SEC)

    with concurrent.futures.ThreadPoolExecutor(CONCURRENCY) as pool:
        list(pool.map(work, targets))

    with open(CATALOG, "w", encoding="utf-8") as fh:
        # sort_keys 를 쓰지 않는다: wts_endpoints.py 가 삽입 순서대로 쓰므로, 여기서
        # 정렬하면 두 도구가 번갈아 돌 때마다 파일 전체가 뒤집혀 diff 가 5000줄이 된다.
        json.dump(catalog, fh, ensure_ascii=False, indent=2)
        fh.write("\n")

    tally: dict[str, int] = {}
    for path in targets:
        tally[endpoints[path]["probe"]["verdict"]] = (
            tally.get(endpoints[path]["probe"]["verdict"], 0) + 1
        )
    print("\n등급별:")
    for verdict, count in sorted(tally.items(), key=lambda kv: -kv[1]):
        print(f"  {verdict:14} {count:4}개")
    print("\nworth-review 가 구현 후보다. 응답 본문은 저장하지 않았다(실계좌 데이터).")


if __name__ == "__main__":
    main()
