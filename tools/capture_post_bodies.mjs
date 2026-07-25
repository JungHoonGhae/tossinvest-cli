#!/usr/bin/env node
// SPA 첫 로드의 non-GET 요청 바디를 CDP 로 캡처한다.
//
// 왜 이게 필요한가: 웹 UI 에만 있는 기능의 POST 바디는 라이브 요청을 봐야만 알 수 있는데,
// 탭 클릭·pushState 로는 React Query 캐시 때문에 재요청이 안 걸린다. 첫 로드를 잡아야 한다.
// 예전에는 addInitScript 로 fetch 를 몽키패치하려 했으나 브라우저를 남이 몰아서 실패했다
// (docs/reverse-engineering/capture-workflow.md "막힌 방법"). 브라우저 컨텍스트를 직접
// 소유하면 JS 주입이 아예 필요 없다 — Network 도메인이 postData 를 그대로 준다.
//
// 안전:
//   - 값은 기본적으로 마스킹된다. 구현에 필요한 건 키와 타입이지 실계좌 값이 아니다.
//     원본이 꼭 필요하면 --raw 를 쓰되 그 출력은 커밋·PR·이슈에 남기지 말 것.
//     --raw 에서도 token/secret/csrf 류 키는 계속 가려진다(재사용 가능한 자격증명).
//   - Playwright 캐시의 "Chrome for Testing" 을 임시 프로필로 띄운다. 사용자의 실제
//     Chrome 프로필/기본 브라우저를 건드리지 않는다.
//
// 사용법:
//   node tools/capture_post_bodies.mjs <path>            # 예: /account/profit
//   node tools/capture_post_bodies.mjs <path> --raw      # 값까지 (주의)
//   node tools/capture_post_bodies.mjs <path> --wait 8   # 대기 초
//   node tools/capture_post_bodies.mjs <path> --all      # 텔레메트리까지 포함
//
// 선행조건: `tossctl auth status` 의 Live Check 가 valid 여야 한다.

import { spawn } from "node:child_process";
import { mkdtempSync, rmSync, readFileSync, existsSync, readdirSync } from "node:fs";
import { tmpdir, homedir } from "node:os";
import { join } from "node:path";

const args = process.argv.slice(2);
const target = args.find((a) => !a.startsWith("--")) ?? "/";
const raw = args.includes("--raw");
const waitSec = Number(args[args.indexOf("--wait") + 1]) || 6;
const keepNoise = args.includes("--all");

// 텔레메트리·로깅 엔드포인트. 기능 발굴에 쓸모없고 출력의 절반을 차지한다.
const NOISE = /\/(log|perf-log)\/bulk|\/tuba\/|\/wts-login-device/;

const ORIGIN = "https://www.tossinvest.com";
const PORT = 9333;

// ── session.json 의 쿠키 → CDP 형식 ──────────────────────────────────────────
function loadCookies() {
  const p = join(homedir(), "Library/Application Support/tossctl/session.json");
  if (!existsSync(p)) throw new Error(`session.json 없음: ${p} — \`tossctl auth login\` 먼저`);
  const s = JSON.parse(readFileSync(p, "utf8"));
  const jar = s.cookies ?? {};
  return Object.entries(jar).map(([name, value]) => ({
    name,
    value: String(value),
    domain: ".tossinvest.com",
    path: "/",
    secure: true,
  }));
}

// ── Playwright 캐시에서 Chrome for Testing 찾기 ──────────────────────────────
function findChrome() {
  const base = join(homedir(), "Library/Caches/ms-playwright");
  if (!existsSync(base)) throw new Error("ms-playwright 캐시 없음 — Chrome for Testing 을 찾을 수 없다");
  const dirs = readdirSync(base).filter((d) => d.startsWith("chromium-")).sort().reverse();
  for (const d of dirs) {
    const exe = join(base, d, "chrome-mac-arm64",
      "Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing");
    if (existsSync(exe)) return exe;
  }
  throw new Error("Chrome for Testing 실행파일을 찾지 못했다");
}

// ── 값 마스킹: 구조는 남기고 내용만 지운다 ───────────────────────────────────
function redact(v) {
  if (v === null) return null;
  // 라벨은 "총 N개" 로 쓴다. 예전엔 "…(N개)" 였는데 앞에 표본이 하나 찍히니
  // "N개 더" 로 읽혀 실제로 개수를 잘못 세는 일이 있었다.
  if (Array.isArray(v)) {
    if (v.length === 0) return [];
    if (v.length === 1) return [redact(v[0])];
    return [redact(v[0]), `…총 ${v.length}개`];
  }
  if (typeof v === "object") return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, redact(x)]));
  if (typeof v === "number") return "<number>";
  if (typeof v === "boolean") return "<boolean>";
  const s = String(v);
  return s.length > 12 ? `<string:${s.length}>` : "<string>";
}

// SECRET_KEY 는 --raw 에서도 절대 원본을 내보내지 않는 필드다. 마스킹이 값을
// 지우는 건 계좌 데이터를 가리기 위함이고, 이쪽은 재사용 가능한 자격증명이라
// 성격이 다르다 — 조사 중 XSRF 토큰을 콘솔에 흘린 적이 있어 방어를 넣는다.
const SECRET_KEY = /token|secret|password|passwd|authorization|cookie|csrf|xsrf/i;

function stripSecrets(v) {
  if (v === null || typeof v !== "object") return v;
  if (Array.isArray(v)) return v.map(stripSecrets);
  return Object.fromEntries(
    Object.entries(v).map(([k, x]) => [k, SECRET_KEY.test(k) ? "<secret 제거됨>" : stripSecrets(x)]),
  );
}

function show(body) {
  if (!body) return "(바디 없음)";
  if (raw) {
    // --raw 여도 자격증명은 거른다.
    try {
      return JSON.stringify(stripSecrets(JSON.parse(body)), null, 2);
    } catch {
      return body;
    }
  }
  try {
    return JSON.stringify(redact(JSON.parse(body)), null, 2);
  } catch {
    return `<non-JSON body: ${body.length} bytes>`;
  }
}

// ── CDP ──────────────────────────────────────────────────────────────────────
const profile = mkdtempSync(join(tmpdir(), "tossctl-capture-"));
const chrome = spawn(findChrome(), [
  "--headless=new",
  `--remote-debugging-port=${PORT}`,
  `--user-data-dir=${profile}`,
  "--no-first-run",
  "--no-default-browser-check",
  "about:blank",
], { stdio: "ignore" });

const cleanup = () => {
  try { chrome.kill(); } catch {}
  try { rmSync(profile, { recursive: true, force: true }); } catch {}
};
process.on("exit", cleanup);
process.on("SIGINT", () => { cleanup(); process.exit(130); });

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function waitForCDP() {
  for (let i = 0; i < 40; i++) {
    try { return await (await fetch(`http://127.0.0.1:${PORT}/json/version`)).json(); }
    catch { await sleep(250); }
  }
  throw new Error("CDP 연결 실패");
}

const ver = await waitForCDP();
const ws = new WebSocket(ver.webSocketDebuggerUrl);
let id = 0;
const pending = new Map();
const seen = [];

const send = (method, params = {}, sessionId) =>
  new Promise((resolve) => {
    const i = ++id;
    pending.set(i, resolve);
    ws.send(JSON.stringify({ id: i, method, params, ...(sessionId && { sessionId }) }));
  });

await new Promise((r) => (ws.onopen = r));
ws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.id && pending.has(m.id)) { pending.get(m.id)(m.result); pending.delete(m.id); return; }
  if (m.method === "Network.requestWillBeSent") {
    const r = m.params.request;
    // OPTIONS 는 CORS 프리플라이트라 바디가 없다 — 노이즈.
    if (r.method === "GET" || r.method === "OPTIONS") return;
    if (!r.url.includes("/api/")) return;
    if (!keepNoise && NOISE.test(r.url)) return;   // 텔레메트리 — --all 로 포함
    // postData 는 바디가 작을 때만 인라인으로 온다. 그 외에는 hasPostData 만 서고
    // Network.getRequestPostData 로 따로 받아야 한다 (라이브에서 대부분 이쪽).
    seen.push({
      method: r.method,
      url: r.url,
      postData: r.postData,
      requestId: m.params.requestId,
      hasPostData: r.hasPostData,
    });
  }
};

const { targetId } = await send("Target.createTarget", { url: "about:blank" });
const { sessionId } = await send("Target.attachToTarget", { targetId, flatten: true });

await send("Network.enable", {}, sessionId);              // 내비게이션 '전에'
await send("Network.setCookies", { cookies: loadCookies() }, sessionId);
await send("Page.enable", {}, sessionId);
await send("Page.navigate", { url: ORIGIN + target }, sessionId);
await sleep(waitSec * 1000);

// 인라인으로 안 온 바디를 requestId 로 회수한다.
for (const r of seen) {
  if (r.postData || !r.hasPostData) continue;
  const res = await send("Network.getRequestPostData", { requestId: r.requestId }, sessionId);
  r.postData = res?.postData;
}

console.log(`\n대상: ${ORIGIN}${target}`);
console.log(`캡처된 non-GET /api/ 요청: ${seen.length}개` + (raw ? "  [--raw: 값 노출됨]" : "  [값 마스킹됨]"));
// 같은 엔드포인트가 여러 번 불리면 한 번만 (로그 수집 등)
const byKey = new Map();
for (const r of seen) {
  // 호스트를 남긴다. 토스는 wts-api / wts-info-api / wts-cert-api 를 섞어 쓰고
  // client 도 셋을 따로 설정하므로, 경로만 보면 어느 BaseURL 에 붙일지 알 수 없다.
  const k = `${r.method} ${r.url.replace(/\?.*$/, "")}`;
  if (!byKey.has(k)) byKey.set(k, { ...r, count: 1 });
  else byKey.get(k).count++;
}
for (const [k, r] of byKey) {
  console.log(`\n── ${k}${r.count > 1 ? `  (×${r.count})` : ""}`);
  console.log(show(r.postData));
}
if (!seen.length) {
  console.log("\n힌트: 세션이 유효한지(`tossctl auth status` → Live Check: valid),");
  console.log("      해당 라우트에 웹 UI 가 있는지 확인. --wait 로 대기를 늘려볼 것.");
}
ws.close();
process.exit(0);
