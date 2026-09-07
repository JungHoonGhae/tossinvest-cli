import Link from 'fumadocs-core/link';
import { TossctlIcon } from '@/app/layout.client';
import {
  Bot,
  ChartCandlestick,
  Check,
  ChevronDown,
  Github,
  Radio,
  ShieldCheck,
  Sparkles,
  Star,
  TerminalSquare,
} from 'lucide-react';

const FEATURE_ICONS = [ChartCandlestick, ShieldCheck, Bot, Sparkles, Radio, TerminalSquare];

const AGENTS = [
  { name: 'Claude Code', logo: '/logos/claude.svg', sub: 'agent' },
  { name: 'Codex', logo: '/logos/codex.svg', sub: 'agent' },
  { name: 'Cursor', logo: '/logos/cursor.svg', sub: 'agent' },
];
const INTEGRATIONS = [
  { name: 'OpenClaw', logo: '/logos/openclaw.svg', sub: 'agent' },
  { name: 'opencode', logo: '/logos/opencode.svg', sub: 'agent' },
  { name: 'Hermes Agent', logo: '/logos/hermes.png', sub: 'agent' },
];

const content = {
  ko: {
    sub: 'connect your AI agents to Toss Securities',
    desc: (
      <>
        공식 API의 계좌·시세·주문에 WTS 전용 수급·AI 시그널·배당·관심종목을 더하세요.{' '}
        <code className="font-mono text-white/90">tossctl</code> 하나로 터미널과 AI 에이전트에서 사용합니다.
      </>
    ),
    cta: '5분 만에 시작',
    proof: {
      label: 'GitHub community',
      note: 'Star를 남긴 개발자의 공개 프로필 소속 기준 · 기업의 사용·추천을 의미하지 않습니다',
    },
    thesis: {
      label: '왜 지금',
      headline: '계좌와 주문을 넘어, 토스증권 데이터를 자동화로',
      body: 'WTS는 토스증권 웹 트레이딩 시스템입니다. 공식 Open API에 없는 수급·AI 시그널·배당·관심종목을 웹 세션으로 연결하고, 공식 키가 필요한 기능은 공식 API로 사용합니다. 일반 토스뱅킹·카드 소비 내역은 아직 지원하지 않습니다.',
      points: [
        { k: 'WTS 전용 기능', v: '투자자 수급, 시장 지수, AI 시그널, 조건검색, 배당과 관심종목을 조회·관리합니다.' },
        { k: '공식 API 연결', v: '공식 키를 등록하면 지원되는 조회는 기본적으로 공식 경로를 우선 사용합니다. 주문 경로는 명령별 정책을 따릅니다.' },
        { k: '에이전트 연동 — CLI + MCP 둘 다', v: 'CLI는 JSON 출력으로, MCP 서버(tossctl mcp)는 툴로. 두 방식 모두 지원해 Claude·Codex·Cursor 등 어떤 AI 에이전트에든 바로 붙습니다.' },
        { k: '명시적인 실행 승인', v: '실거래는 기본 비활성입니다. 설정 변경과 모의투자도 각자의 미리보기·승인 정책을 따릅니다.' },
      ],
    },
    why: {
      headline: '공식 API와 WTS, 필요한 기능에 맞는 경로',
      p1: '공식 API는 키와 허용 IP로 연결합니다. WTS 기능은 휴대폰 승인을 거친 웹 세션이 필요합니다. 두 인증을 함께 연결하면 지원되는 공식·WTS 기능을 하나의 도구에서 사용할 수 있습니다.',
      p2: '일부 조회는 공식 API 오류 시 설정에 따라 WTS로 전환합니다. 주문은 한 경로로만 제출하며 다른 경로로 재시도하지 않습니다. WTS는 비공식 API이므로 예고 없이 변경될 수 있습니다.',
      kicker: '기능 범위와 인증·실행 정책을 함께 확인하세요.',
    },
    sectionLabel: '왜 tossctl 인가',
    compareLabel: '공식 OPEN API + WTS',
    compareLead: (
      <>
        공식 API의 계좌·시세·주문에 <span className="text-brand-200">WTS 전용 기능</span>을 더합니다.
      </>
    ),
    stats: [
      { n: 'API + WTS', l: '서로 다른 두 인증 경로' },
      { n: 'CLI + MCP', l: '터미널·스크립트·AI 에이전트' },
      { n: 'JSON · CSV', l: '조회 데이터를 자동화에 연결' },
    ],
    coverage: {
      addedLabel: '토스 WTS 고유 기능',
      officialLabel: '공식 Open API',
      hubLabel: 'tossctl',
      hubNote: '두 소스를 하나의 명령 체계로',
      note: '기능별 지원 여부와 인증 요구사항은 지원 범위 문서에서 확인할 수 있습니다.',
    },
    official: {
      name: '공식 Open API',
      note: '공식 키·허용 IP 필요',
      items: ['계좌·잔고', '시세·호가·체결', '주문·취소·정정'],
    },
    toss: {
      name: 'tossctl',
      note: '공식 API와 WTS 전용 기능을 한 인터페이스로',
      items: [
        '공식 API 계좌·시세·주문 (공식 키 필요)',
        '수급·시장지수·지수 상세·업종 등락',
        'AI 시그널·뉴스 브리핑·조건검색',
        '배당·커뮤니티 랭킹·관심종목·실시간 푸시·dry-run preview',
      ],
    },
    llmTitle: 'LLM이 바로 읽는 문서',
    llmDesc: (
      <>
        모든 페이지에 Copy Markdown · ChatGPT/Claude/Cursor로 열기 +{' '}
        <code className="font-mono text-white/80">/llms.txt</code> 제공.
      </>
    ),
    llmCta: 'AI 에이전트 가이드 →',
    disclaimer: '비공식 CLI · 토스증권과 무관 · 투자 손익의 책임은 본인에게 있습니다',
    faqTitle: '자주 묻는 질문',
    faqSub: '여기에 없는 내용은 GitHub Issues 로 남겨 주세요.',
    faq: [
      {
        q: '어떻게 공식 Open API보다 많은 기능을 제공하나요?',
        a: '공식 API에 없는 기능은 토스증권 WTS의 내부 API로 보완합니다. 검증된 조회·설정 기능만 구현하며, WTS 전체나 일반 토스 앱 전체를 지원한다는 뜻은 아닙니다.',
      },
      {
        q: '토스뱅킹이나 카드 소비 내역도 되나요?',
        a: '아직 지원하지 않습니다. 현재 대상은 토스증권이며, 일반 Banking·MyData는 별도의 모바일 인증과 연결 구현이 필요합니다. 증권 주식모으기의 자금연결 상태 조회와는 다릅니다.',
      },
      {
        q: '공식 API가 넓어지면 tossctl 은 무의미해지나요?',
        a: '공식 API 변경을 추적하고 지원 경로를 갱신합니다. WTS 전용 기능, CLI·MCP 인터페이스, 구조화된 출력과 실행 확인은 함께 유지합니다. 현재 지원 여부는 문서의 기능 비교표가 기준입니다.',
      },
      {
        q: '합법인가요? 토스 공식인가요?',
        a: '토스증권 공식 제품이 아닙니다. 공식 Open API와 비공식 WTS 경로를 함께 제공하며, WTS 사용은 이용약관 위반에 해당할 수 있고 예고 없이 중단될 수 있습니다. 사용 조건을 확인하고 본인의 책임으로 사용하세요.',
      },
      {
        q: '실수로 주문이 나갈 수 있나요?',
        a: '기본 설정에서는 실거래가 차단됩니다. 사용자가 기능을 켜도 주문별 미리보기와 --execute·--confirm이 필요합니다. 다만 잘못된 주문 승인이나 투자 손실까지 방지하는 보장은 아닙니다.',
      },
      {
        q: '내 계정 정보와 키는 안전한가요?',
        a: '자격증명은 로컬 소유자 전용 파일에 저장되며 인증·API 호출에 사용됩니다. AI 호스트에 반환한 계좌 데이터는 그 호스트의 처리 정책을 따르므로 공개 로그·프롬프트 공유에 주의하세요.',
      },
      {
        q: 'AI 에이전트와 어떻게 연동하나요?',
        a: '일반 조회는 --output json으로 출력할 수 있고 /llms.txt 와 에이전트 가이드를 제공합니다. Claude Code·Codex·Cursor 같은 도구가 바로 호출할 수 있습니다.',
      },
    ],
    features: [
      { label: 'DATA', title: '넓은 조회', desc: '계좌·시세·호가·체결·수급·지수·업종·배당·거래내역을 명령 한 줄로 조회합니다.' },
      { label: 'SAFETY', title: '안전한 거래', desc: '실거래는 기본 비활성입니다. 기능별 설정과 주문별 미리보기·실행 확인으로 의도하지 않은 제출을 줄입니다.' },
      { label: 'AGENTS', title: 'CLI + MCP 둘 다', desc: 'CLI(JSON 출력)로도, MCP 서버(tossctl mcp)로도 붙습니다. 두 방식 모두 지원 — Claude·Codex·Cursor 등 어떤 에이전트에든 그대로 연동됩니다.' },
      { label: 'INTELLIGENCE', title: '토스 AI 기능', desc: '공식 API에는 없는 AI 시그널·뉴스 브리핑·조건검색·커뮤니티 랭킹을 제공합니다.' },
      { label: 'REALTIME', title: '실시간 푸시', desc: '주문·체결·보유 변동을 실시간으로 받아봅니다.' },
      { label: 'AUTOMATION', title: '자동화 우선', desc: '표·파일·실시간 등 원하는 형식으로 내보내 스크립트와 자동화에 바로 연결합니다.' },
    ],
  },
  en: {
    sub: 'connect your AI agents to Toss Securities',
    desc: (
      <>
        Read accounts, quotes, and transactions, and place orders — plus the Toss WTS features the
        official API doesn't expose. All from{' '}
        <code className="font-mono text-white/90">tossctl</code>, right now, official API coverage included.
      </>
    ),
    cta: 'Start in 5 minutes',
    proof: {
      label: 'GitHub community',
      note: 'Affiliations from stargazers’ public profiles, not company usage or endorsements',
    },
    thesis: {
      label: 'WHY NOW',
      headline: 'Beyond accounts and orders: Securities data for your automation',
      body: 'WTS is the Toss Securities Web Trading System. Connect a web session for investor flows, AI signals, dividends, and watchlists missing from the official API. Connect an official key for official-only features. General Toss banking and card spending are not supported.',
      points: [
        { k: 'WTS-only features', v: 'Explore investor flows, indices, AI signals, screeners, dividends, and watchlists.' },
        { k: 'Official API support', v: 'Supported reads prefer the official API by default when a key is connected. Order routing follows each command’s policy.' },
        { k: 'Agents — CLI + MCP', v: 'The CLI answers in JSON; the MCP server (tossctl mcp) exposes tools. Both are supported, so Claude, Codex, Cursor and any agent plug in right away.' },
        { k: 'Explicit execution approval', v: 'Live trading is disabled by default. Settings and paper trading follow their own preview and authorization policies.' },
      ],
    },
    why: {
      headline: 'Official API and WTS: the right path for each feature',
      p1: 'The official API needs a key and allowed IP. WTS uses a web session approved on your phone. Connect both credentials to use supported official and WTS features from one tool.',
      p2: 'Some reads can fall back to WTS after an eligible official API failure. Orders use one backend without cross-backend retry. WTS is unofficial and can change without notice.',
      kicker: 'Check feature coverage alongside authentication and execution policies.',
    },
    sectionLabel: 'WHY TOSSCTL',
    compareLabel: 'OFFICIAL OPEN API + WTS',
    compareLead: (
      <>
        Official accounts, quotes, and orders, plus <span className="text-brand-200">WTS-only features</span>.
      </>
    ),
    stats: [
      { n: 'API + WTS', l: 'Two separate authentication paths' },
      { n: 'CLI + MCP', l: 'Terminals, scripts, and AI agents' },
      { n: 'JSON · CSV', l: 'Read data for your automation' },
    ],
    coverage: {
      addedLabel: 'Toss WTS-only features',
      officialLabel: 'Official Open API',
      hubLabel: 'tossctl',
      hubNote: 'Two sources, one command interface',
      note: 'See Support Scope for feature availability and authentication requirements.',
    },
    official: {
      name: 'Official Open API',
      note: 'Official key and allowed IP required',
      items: ['Accounts · balances', 'Quotes · orderbook · ticks', 'Place · cancel · amend'],
    },
    toss: {
      name: 'tossctl',
      note: 'Official API and WTS-only capabilities in one interface',
      items: [
        'Official accounts, quotes, and orders (official key required)',
        'Flows · indices · index detail · sectors',
        'AI signals · news briefing · screener',
        'Dividends · community rankings · watchlist · real-time push · dry-run',
      ],
    },
    llmTitle: 'Docs LLMs can read directly',
    llmDesc: (
      <>
        Every page has Copy Markdown · Open in ChatGPT/Claude/Cursor +{' '}
        <code className="font-mono text-white/80">/llms.txt</code>.
      </>
    ),
    llmCta: 'AI Agent Guide →',
    disclaimer: 'Unofficial CLI · not affiliated with Toss Securities · use at your own risk',
    faqTitle: 'Frequently asked',
    faqSub: 'Anything missing? Open a GitHub issue.',
    faq: [
      {
        q: 'How do you offer more features than the official Open API?',
        a: 'Features missing from the official API are supplied through verified Toss Securities WTS endpoints. This does not mean every WTS or general Toss app feature is implemented.',
      },
      {
        q: 'Does this support Toss banking or card spending?',
        a: 'Not yet. The current scope is Toss Securities. General Banking and MyData need a separate mobile connector and authentication; Securities accumulation funding status is a different feature.',
      },
      {
        q: 'Does tossctl become pointless once the official API grows?',
        a: 'We track official API changes and update supported routes. WTS-only features, CLI and MCP access, structured output, and execution checks remain useful. Consult the support matrix for current coverage.',
      },
      {
        q: 'Is this legal? Is it official?',
        a: 'This is not an official Toss Securities product. It supports both official API and unofficial WTS routes. WTS use may violate the Terms of Service and stop working without notice. Review the terms and use at your own risk.',
      },
      {
        q: 'Can an order fire by accident?',
        a: 'Live trading is blocked by default. Even after enabling it, each order needs a preview, --execute, and --confirm. These checks cannot guarantee against approving a wrong order or investment losses.',
      },
      {
        q: 'Are my account data and keys safe?',
        a: 'Credentials are stored in owner-only local files and used for authentication and API calls. Account data returned to an AI host follows that host’s processing policies, so avoid public logs and shared prompts.',
      },
      {
        q: 'How does it work with AI agents?',
        a: 'General reads support --output json, and there is an agent guide plus /llms.txt. Tools like Claude Code, Codex, and Cursor can call it directly.',
      },
    ],
    features: [
      { label: 'DATA', title: 'Broad reads', desc: 'Accounts, quotes, orderbook, ticks, flows, indices, sectors, dividends, ledger, in one command.' },
      { label: 'SAFETY', title: 'Safe trading', desc: 'Live trading is disabled by default. Per-action config and order-specific preview and confirmation reduce unintended submissions.' },
      { label: 'AGENTS', title: 'CLI + MCP, both', desc: 'Connect via the CLI (JSON output) or the MCP server (tossctl mcp) — both supported, so agents like Claude, Codex, and Cursor plug in right away.' },
      { label: 'INTELLIGENCE', title: 'Toss AI features', desc: 'AI signals, news briefing, screener, community rankings, none of which the official API has.' },
      { label: 'REALTIME', title: 'Real-time push', desc: 'See order, fill, and holdings changes the moment they happen.' },
      { label: 'AUTOMATION', title: 'Automation-first', desc: 'Export as a table, file, or live feed, drop it straight into scripts and automation.' },
    ],
  },
} as const;

// MZ8Ua-style connector trails: three rows of fading dots run from the node
// edge outward toward the three agent / integration cards on each side.
// Rows are aligned to the card centers (node box is HUB_H tall, cards ~70px apart).
const HUB_H = 198;
function DotTrails() {
  const cy = HUB_H / 2; // 99 — vertical center of the node box
  const rows = [-70, 0, 70]; // aligns with the 3 cards in each side column
  const n = 8;
  return (
    <div className="pointer-events-none absolute inset-0 hidden lg:block">
      {rows.map((dy) =>
        Array.from({ length: n }).map((_, i) => {
          const dist = 78 + i * 19; // 78 → ~211px from center (reaches into the gap)
          const op = 0.5 * (1 - i / (n + 1)); // brightest near node, fades outward — neutral
          return (
            <span key={`${dy}-${i}`}>
              <span
                className="absolute rounded-full bg-white"
                style={{ width: 3, height: 3, left: 120 - dist, top: cy + dy, opacity: op }}
              />
              <span
                className="absolute rounded-full bg-white"
                style={{ width: 3, height: 3, left: 120 + dist, top: cy + dy, opacity: op }}
              />
            </span>
          );
        }),
      )}
    </div>
  );
}

// Scrolling "works with" strip — AI coding agents that drive tossctl.
// Logos keep their brand colors; only naturally-black marks ship as white SVGs.
const MARQUEE = [
  { name: 'Claude Code', logo: '/logos/mq/claude.svg' },
  { name: 'Codex CLI', logo: '/logos/codex.svg' },
  { name: 'Gemini CLI', logo: '/logos/mq/googlegemini.svg' },
  { name: 'Cursor', logo: '/logos/mq/cursor.svg' },
  { name: 'GitHub Copilot', logo: '/logos/mq/githubcopilot.svg' },
  { name: 'OpenCode', logo: '/logos/opencode.svg' },
  { name: 'Qwen Code', logo: '/logos/mq/qwen.svg' },
  { name: 'DeepSeek', logo: '/logos/mq/deepseek.svg' },
  { name: 'Mistral', logo: '/logos/mq/mistralai.svg' },
  { name: 'Kimi CLI', logo: '/logos/mq/moonshotai.svg' },
  { name: 'OpenClaw', logo: '/logos/openclaw.svg' },
  { name: 'Hermes Agent', logo: '/logos/hermes.png' },
];
function Marquee() {
  // Two identical groups translated by exactly -50% → seamless infinite loop.
  // The trailing gap (gap-12) equals the inner gap so the seam is invisible.
  const group = (key: string) => (
    <div key={key} className="flex shrink-0 items-center gap-12 pe-12" aria-hidden={key === 'b'}>
      {MARQUEE.map((m) => (
        <span key={m.name} className="inline-flex shrink-0 items-center gap-2.5 text-white/60">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={m.logo} alt="" className="size-6 object-contain md:size-8" />
          <span className="font-mono text-xs md:text-sm">{m.name}</span>
        </span>
      ))}
    </div>
  );
  return (
    <div className="relative border-y border-white/10 py-12">
      {/* "Works with" sits ON the top border, legend-style */}
      <span className="absolute left-1/2 top-0 -translate-x-1/2 -translate-y-1/2 bg-[#0a0a0a] px-4 font-mono text-[11px] uppercase tracking-[0.3em] text-white/40">
        Works with
      </span>
      <div className="overflow-hidden [mask-image:linear-gradient(90deg,transparent,#000_6%,#000_94%,transparent)]">
        <div className="flex w-max animate-[tossMarquee_36s_linear_infinite]">
          {group('a')}
          {group('b')}
        </div>
      </div>
    </div>
  );
}

// "Trusted by builders from" grid. Companies are derived from stargazers'
// public GitHub `company` field — framed as *builders from*, never an
// endorsement. Toss is intentionally excluded (this is an unofficial tool).
// Each logo is bounded by a per-logo max height `h` AND a max width `w`
// (whichever binds first) so sizes are locked and visually balanced.
const COMPANIES: { name: string; logo?: string; h?: number; w?: number }[] = [
  { name: 'TwelveLabs', logo: '/logos/companies/twelvelabs.svg', h: 28 },
  { name: 'Sendbird', logo: '/logos/companies/sendbird.png', h: 18 },
  { name: 'Samsung', logo: '/logos/companies/samsung.png', h: 15, w: 120 },
  { name: 'Kakao', logo: '/logos/companies/kakao.png', h: 17 },
  { name: 'Naver', logo: '/logos/companies/naver.svg', h: 14 },
  { name: 'Dunamu', logo: '/logos/companies/dunamu.png', h: 15 },
  { name: 'TMAP', logo: '/logos/companies/tmap.svg', h: 22, w: 150 },
  { name: 'Daangn', logo: '/logos/companies/daangn.svg', h: 43 },
];

// Coverage dot-map: tossctl (official-matching + unique) reaches much further into
// the Toss web-app surface than the official API alone.
// Hub-and-spoke: tossctl sits in the middle, pulling from two real sources
// (the official Open API and the Toss WTS web app) — this is the actual
// hybrid-routing architecture, not an abstract proportion chart.
function CoverageHub({
  officialLabel,
  wtsLabel,
  hubLabel,
  hubNote,
  note,
}: {
  officialLabel: string;
  wtsLabel: string;
  hubLabel: string;
  hubNote: string;
  note: string;
}) {
  return (
    <div className="rounded-xl border border-white/10 bg-[#0f0f0f] p-6">
      <div className="flex flex-col items-center gap-3 py-4 sm:flex-row sm:justify-center sm:gap-0">
        <div className="flex w-full flex-col items-center gap-1 rounded-xl border border-white/10 bg-white/[0.03] px-5 py-4 text-center sm:w-40">
          <span className="text-2xl font-bold text-sky-400">API</span>
          <span className="font-mono text-[11px] text-white/45">{officialLabel}</span>
        </div>
        <div className="h-6 w-px bg-white/15 sm:h-px sm:w-10" />
        <div className="flex flex-col items-center gap-1 rounded-full border-2 border-orange-400 bg-orange-400/10 px-8 py-5 text-center">
          <span className="text-3xl font-bold text-orange-400">CLI · MCP</span>
          <span className="font-mono text-[11px] text-orange-300">{hubLabel}</span>
        </div>
        <div className="h-6 w-px bg-white/15 sm:h-px sm:w-10" />
        <div className="flex w-full flex-col items-center gap-1 rounded-xl border border-white/10 bg-white/[0.03] px-5 py-4 text-center sm:w-40">
          <span className="text-2xl font-bold text-brand-200">WTS</span>
          <span className="font-mono text-[11px] text-white/45">{wtsLabel}</span>
        </div>
      </div>
      <p className="mt-1 text-center text-[11px] text-white/35">{hubNote}</p>
      <p className="mt-3 text-center text-[12px] leading-relaxed text-white/45">{note}</p>
    </div>
  );
}

function SpokeCard({
  logo,
  name,
  sub,
  mirror = false,
}: {
  logo: string;
  name: string;
  sub: string;
  mirror?: boolean;
}) {
  return (
    <div
      className={
        'flex items-center gap-3 rounded-lg border border-white/10 bg-[#111111] p-3 transition-colors hover:border-white/25 ' +
        (mirror ? 'flex-row-reverse text-right' : '')
      }
    >
      <span className="grid size-8 shrink-0 place-items-center rounded-md border border-white/10 bg-white/[0.06]">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        {/* 로고는 본연의 색 유지 (codex.svg 만 흰색으로 제작됨) */}
        <img src={logo} alt={name} className="size-4 object-contain" />
      </span>
      <div className="leading-tight">
        <div className="text-sm font-medium">{name}</div>
        <div className="font-mono text-[10px] uppercase tracking-widest text-white/30">{sub}</div>
      </div>
    </div>
  );
}

// Live GitHub star count (revalidated daily); never invent a count on failure.
async function getGitHubStars(): Promise<number | null> {
  try {
    const res = await fetch('https://api.github.com/repos/JungHoonGhae/tossinvest-cli', {
      next: { revalidate: 86400 },
      headers: { Accept: 'application/vnd.github+json' },
    });
    if (res.ok) {
      const data = (await res.json()) as { stargazers_count?: number };
      if (typeof data.stargazers_count === 'number') return data.stargazers_count;
    }
  } catch {
    // ignore — fall back below
  }
  return null;
}

export default async function HomePage(props: PageProps<'/[lang]'>) {
  const { lang } = await props.params;
  const t = content[lang === 'en' ? 'en' : 'ko'];
  const p = lang === 'en' ? '/en' : '';
  const stars = await getGitHubStars();
  const starsLabel = stars === null ? 'GitHub' : stars.toLocaleString('en-US');

  return (
    <main className="flex flex-1 flex-col bg-[#0a0a0a] text-white">
      {/* ── Hero (hub & spoke) ──────────────────────────────── */}
      <section className="relative overflow-hidden border-b border-white/10">
        <div
          className="pointer-events-none absolute inset-0"
          style={{
            backgroundImage:
              'linear-gradient(to right, rgba(255,255,255,0.045) 1px, transparent 1px), linear-gradient(to bottom, rgba(255,255,255,0.045) 1px, transparent 1px)',
            backgroundSize: '208px 148px',
            backgroundPosition: 'center',
            maskImage: 'radial-gradient(95% 75% at 50% 42%, black, transparent)',
            WebkitMaskImage: 'radial-gradient(95% 75% at 50% 42%, black, transparent)',
          }}
        />
        <div
          className="pointer-events-none absolute left-1/2 top-0 h-[460px] w-[760px] -translate-x-1/2"
          style={{ background: 'radial-gradient(50% 50% at 50% 0%, rgba(52,211,153,0.16), transparent)' }}
        />

        <div className="relative mx-auto w-full max-w-6xl px-4 pb-16 pt-14">
          <div className="mb-12 flex items-center justify-center gap-3 font-mono text-xs text-white/45">
            <span className="font-sans text-base font-bold tracking-tight text-white">tossctl</span>
            <span className="text-white/25">×</span>
            <span className="font-sans text-sm font-bold tracking-tight text-white/40">AI Agents</span>
          </div>

          {/* heading block (above the hub, MZ8Ua-style) */}
          <div className="mx-auto mb-16 max-w-2xl text-center">
            <h1 className="font-sans text-4xl font-bold tracking-tight md:text-5xl">tossinvest-cli</h1>
            <p className="mt-3 font-mono text-xs text-white/45">{t.sub}</p>
            <p className="mx-auto mt-5 max-w-md break-keep text-sm text-white/65">{t.desc}</p>
            <div className="mt-7 flex flex-wrap items-center justify-center gap-3">
              <Link
                href={`${p}/docs/getting-started/quickstart`}
                className="rounded-md bg-brand px-5 py-2.5 text-sm font-semibold text-brand-foreground transition-opacity hover:opacity-90"
              >
                {t.cta}
              </Link>
              <Link
                href="https://github.com/JungHoonGhae/tossinvest-cli"
                className="inline-flex items-center gap-2 rounded-md border border-white/15 px-5 py-2.5 text-sm font-medium text-white/80 transition-colors hover:bg-white/5"
              >
                <Github className="size-4" />
                GitHub
              </Link>
            </div>
          </div>

          {/* hub & spoke — node vertically aligned with the cards on each side */}
          <div className="relative grid items-center gap-8 lg:grid-cols-[1fr_auto_1fr]">
            <div className="relative z-2 hidden flex-col gap-3 lg:flex">
              <div className="absolute -top-7 left-0 font-mono text-[10px] uppercase tracking-[0.2em] text-white/30">AI AGENTS</div>
              {AGENTS.map((a) => (
                <SpokeCard key={a.name} {...a} />
              ))}
            </div>

            <div
              className="relative z-1 grid place-items-center justify-self-center"
              style={{ width: 240, height: HUB_H }}
            >
              {/* soft circle glow behind the node (replaces the concentric strokes) */}
              <span
                className="pointer-events-none absolute rounded-full"
                style={{
                  width: 360,
                  height: 360,
                  background:
                    'radial-gradient(circle, rgba(255,255,255,0.10), rgba(255,255,255,0.03) 42%, transparent 70%)',
                }}
              />
              <DotTrails />
              <TossctlIcon className="relative size-[80px] drop-shadow-[0_0_40px_rgba(255,255,255,0.08)]" />
              <span className="absolute left-1/2 top-[152px] -translate-x-1/2 font-mono text-[11px] tracking-wide text-white/40">
                tossctl
              </span>
            </div>

            <div className="relative z-2 hidden flex-col gap-3 lg:flex">
              <div className="absolute -top-7 right-0 font-mono text-[10px] uppercase tracking-[0.2em] text-white/30">AGENTS · SHELL</div>
              {INTEGRATIONS.map((a) => (
                <SpokeCard key={a.name} {...a} mirror />
              ))}
            </div>
          </div>

          <div className="mx-auto mt-14 w-full max-w-3xl overflow-hidden rounded-xl border border-white/10 bg-black/40 text-left shadow-2xl backdrop-blur">
            <div className="flex items-center gap-1.5 border-b border-white/10 px-4 py-3">
              <span className="size-3 rounded-full bg-[#ff5f56]" />
              <span className="size-3 rounded-full bg-[#ffbd2e]" />
              <span className="size-3 rounded-full bg-[#27c93f]" />
              <span className="ml-3 font-mono text-xs text-white/35">tossctl</span>
            </div>
            <pre className="overflow-auto p-4 font-mono text-[13px] leading-relaxed text-white/80">
              <code>{`$ tossctl auth login
$ tossctl account summary --output json
$ tossctl quote get 005930
$ tossctl market index nasdaq
$ tossctl order preview --symbol TSLA --side buy --qty 1 --price 250`}</code>
            </pre>
          </div>
        </div>
      </section>

      {/* ── Works-with marquee ─────────────────────────────── */}
      <Marquee />

      {/* ── Social proof: "Trusted by builders from" (stargazers' employers) ── */}
      <section className="border-b border-white/10">
        <div className="mx-auto max-w-5xl px-4 py-14 text-center">
          <div className="mb-8 font-sans text-sm font-semibold tracking-wide text-white/80">
            {t.proof.label}
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {COMPANIES.map((c) => (
              <div
                key={c.name}
                className="flex h-20 items-center justify-center rounded-xl border border-white/10 bg-[#0f0f0f] px-4"
              >
                {c.logo ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  // 로고별 max높이+max폭 중 먼저 걸리는 쪽으로 고정. 좁은 셀에서는
                  // min(px, 100%)로 셀 폭을 넘지 않게(가장자리 넘침 방지).
                  <img
                    src={c.logo}
                    alt={c.name}
                    style={{ maxHeight: c.h ?? 24, maxWidth: `min(${c.w ?? 130}px, 100%)` }}
                    className="object-contain"
                  />
                ) : (
                  <span className="font-sans text-[17px] font-semibold tracking-tight text-white/70">
                    {c.name}
                  </span>
                )}
              </div>
            ))}
          </div>
          <a
            href="https://github.com/JungHoonGhae/tossinvest-cli/stargazers"
            className="mt-6 inline-flex items-center gap-1.5 font-mono text-[11px] text-white/35 transition-colors hover:text-white/55"
          >
            <Star className="size-3" />
            {starsLabel} · {t.proof.note}
          </a>
        </div>
      </section>

      {/* ── Thesis / problem ───────────────────────────────── */}
      <section className="border-b border-white/10">
        <div className="mx-auto w-full max-w-5xl px-4 py-20">
          <div className="mb-3 font-mono text-[11px] uppercase tracking-[0.2em] text-brand-200">
            {t.thesis.label}
          </div>
          <h2 className="max-w-3xl font-sans text-2xl font-bold leading-snug md:text-[2rem]">
            {t.thesis.headline}
          </h2>
          <p className="mt-4 max-w-2xl break-keep text-white/60">{t.thesis.body}</p>
          <div className="mt-10 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            {t.thesis.points.map((pt) => (
              <div key={pt.k} className="rounded-xl border border-white/10 bg-[#0f0f0f] p-5">
                <div className="font-mono text-xs font-medium text-brand-200">{pt.k}</div>
                <p className="mt-2 text-sm leading-relaxed text-white/55">{pt.v}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Why the official API lags (editorial) ──────────── */}
      <section className="border-b border-white/10">
        <div className="mx-auto w-full max-w-5xl px-4 py-20 md:py-28">
          <h2 className="max-w-2xl break-keep text-2xl font-semibold leading-tight tracking-tight md:text-4xl">
            {t.why.headline}
          </h2>
          <div className="mt-7 max-w-3xl space-y-5 break-keep text-[15px] leading-relaxed text-white/55 md:text-base">
            <p>{t.why.p1}</p>
            <p>{t.why.p2}</p>
          </div>
          <p className="mt-9 max-w-3xl border-l-2 border-brand-200/60 pl-4 text-lg font-medium leading-snug text-white/90 md:text-xl">
            {t.why.kicker}
          </p>
        </div>
      </section>

      {/* ── Superset comparison ────────────────────────────── */}
      <section className="border-b border-white/10 bg-[#0c0c0c]">
        <div className="mx-auto w-full max-w-5xl px-4 py-16">
          <div className="mb-2 font-mono text-[11px] uppercase tracking-[0.2em] text-white/35">{t.compareLabel}</div>
          <p className="mb-8 max-w-2xl text-lg text-white/80">{t.compareLead}</p>

          <div className="mb-8 grid gap-6 lg:grid-cols-[1.5fr_1fr] lg:items-stretch">
            <CoverageHub
              officialLabel={t.coverage.officialLabel}
              wtsLabel={t.coverage.addedLabel}
              hubLabel={t.coverage.hubLabel}
              hubNote={t.coverage.hubNote}
              note={t.coverage.note}
            />
            <div className="grid grid-cols-1 divide-y divide-white/10 overflow-hidden rounded-xl border border-white/10 bg-[#0f0f0f] sm:grid-cols-3 sm:divide-x sm:divide-y-0 lg:grid-cols-1 lg:divide-x-0 lg:divide-y">
              {t.stats.map((s) => (
                <div key={s.l} className="px-4 py-6 text-center lg:py-5">
                  <div className="font-sans text-3xl font-bold tracking-tight text-brand-200">{s.n}</div>
                  <div className="mx-auto mt-1.5 max-w-[18ch] text-[12px] leading-snug text-white/45">{s.l}</div>
                </div>
              ))}
            </div>
          </div>

          <div className="grid items-stretch gap-4 md:grid-cols-2">
            <div className="rounded-xl border border-white/10 bg-[#0f0f0f] p-6 opacity-80">
              <div className="mb-1 text-sm font-medium text-white/70">{t.official.name}</div>
              <div className="mb-4 font-mono text-[11px] text-white/35">{t.official.note}</div>
              <ul className="space-y-2 text-sm text-white/55">
                {t.official.items.map((it) => (
                  <li key={it} className="flex items-start gap-2">
                    <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-white/25" />
                    {it}
                  </li>
                ))}
              </ul>
            </div>

            <div className="relative rounded-xl border border-brand/40 bg-[#0f1512] p-6 shadow-[0_0_40px_-12px_rgba(52,211,153,0.35)]">
              <div className="mb-1 flex items-center gap-2 text-sm font-semibold">
                <TossctlIcon className="size-4 rounded-[4px]" />
                {t.toss.name}
              </div>
              <div className="mb-4 font-mono text-[11px] text-brand-200">{t.toss.note}</div>
              <ul className="space-y-2 text-sm text-white/80">
                {t.toss.items.map((it) => (
                  <li key={it} className="flex items-start gap-2">
                    <Check className="mt-0.5 size-4 shrink-0 text-brand-200" />
                    {it}
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>
      </section>

      {/* ── Features ───────────────────────────────────────── */}
      <section className="mx-auto w-full max-w-5xl px-4 py-20">
        <div className="mb-8 font-mono text-[11px] uppercase tracking-[0.2em] text-white/35">{t.sectionLabel}</div>
        <div className="grid gap-px overflow-hidden rounded-xl border border-white/10 bg-white/10 sm:grid-cols-2 lg:grid-cols-3">
          {t.features.map((f, i) => {
            const Icon = FEATURE_ICONS[i];
            return (
              <div key={f.title} className="bg-[#0f0f0f] p-6 transition-colors hover:bg-[#141414]">
                <div className="mb-4 grid size-9 place-items-center rounded-lg border border-white/10 bg-white/5">
                  <Icon className="size-4.5 text-brand-200" />
                </div>
                <div className="mb-1 font-mono text-[10px] uppercase tracking-widest text-white/30">{f.label}</div>
                <h3 className="mb-1.5 font-medium">{f.title}</h3>
                <p className="text-sm text-white/55">{f.desc}</p>
              </div>
            );
          })}
        </div>

        <div className="mt-12 flex flex-col items-start gap-3 rounded-xl border border-white/10 bg-[#0f0f0f] p-6 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 className="font-medium">{t.llmTitle}</h2>
            <p className="mt-1 text-sm text-white/55">{t.llmDesc}</p>
          </div>
          <Link
            href={`${p}/docs/guide/agents`}
            className="shrink-0 rounded-md border border-white/15 px-4 py-2 text-sm text-white/80 transition-colors hover:bg-white/5"
          >
            {t.llmCta}
          </Link>
        </div>
      </section>

      {/* ── FAQ ────────────────────────────────────────────── */}
      <section className="border-t border-white/10">
        <div className="mx-auto w-full max-w-5xl px-4 py-20 md:py-24">
          <h2 className="text-2xl font-semibold tracking-tight md:text-3xl">{t.faqTitle}</h2>
          <p className="mt-2 mb-8 text-sm text-white/45">{t.faqSub}</p>
          <div>
            {t.faq.map((item) => (
              <details
                key={item.q}
                className="group border-b border-white/10 py-5 [&_summary::-webkit-details-marker]:hidden"
              >
                <summary className="flex cursor-pointer list-none items-center justify-between gap-4">
                  <span className="font-medium text-white/90">{item.q}</span>
                  <ChevronDown className="size-4 shrink-0 text-white/35 transition-transform duration-200 group-open:rotate-180" />
                </summary>
                <p className="mt-3 max-w-[66ch] text-sm leading-relaxed text-white/55">{item.a}</p>
              </details>
            ))}
          </div>
        </div>
      </section>

      {/* ── Footer ─────────────────────────────────────────── */}
      <footer className="border-t border-white/10">
        <div className="mx-auto flex max-w-5xl flex-col items-center justify-between gap-3 px-4 py-7 font-mono text-xs sm:flex-row">
          <span className="inline-flex items-center gap-2 text-white/45">
            <Github className="size-3.5" />
            JungHoonGhae/tossinvest-cli
          </span>
          <span className="inline-flex items-center gap-2">
            <span className="font-sans font-bold text-[#00ADD8]">Go</span>
            <span className="text-white/20">/</span>
            <span className="font-bold text-white/40">MIT</span>
            <span className="text-white/20">/</span>
            <span className="font-bold text-[#FF8800]">UNOFFICIAL</span>
          </span>
        </div>
        <p className="px-4 pb-8 text-center font-mono text-[10px] leading-relaxed tracking-wider text-white/25">
          {t.disclaimer}
        </p>
      </footer>
    </main>
  );
}
