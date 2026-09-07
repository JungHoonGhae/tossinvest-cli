// Single source of truth for the canonical site URL (used by metadata,
// robots.txt, sitemap.xml, and the llms.txt routes).
export const siteUrl =
  process.env.NEXT_PUBLIC_SITE_URL ??
  (process.env.VERCEL_PROJECT_PRODUCTION_URL
    ? `https://${process.env.VERCEL_PROJECT_PRODUCTION_URL}`
    : 'https://tossinvest-cli.vercel.app');

export const absoluteUrl = (path: string) => new URL(path, siteUrl).toString();

export const siteDescription = {
  ko: '토스증권 CLI·MCP 서버. 공식 Open API의 계좌·시세·주문에 WTS 전용 수급·AI 시그널·배당·관심종목을 더합니다. 비공식 프로젝트.',
  en: 'Toss Securities CLI and MCP server. Official accounts, quotes, and orders plus WTS-only investor flows, AI signals, dividends, and watchlists. Unofficial project.',
} as const;
