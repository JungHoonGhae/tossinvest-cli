import type { BaseLayoutProps, LinkItemType } from 'fumadocs-ui/layouts/shared';
import { Bot, Github, History, Rocket, ShieldCheck, Star, TerminalSquare } from 'lucide-react';
import {
  NavbarMenu,
  NavbarMenuContent,
  NavbarMenuLink,
  NavbarMenuTrigger,
} from 'fumadocs-ui/layouts/home/navbar';
import Link from 'fumadocs-core/link';
import { TossctlIcon } from '@/app/layout.client';
import { defineI18nUI } from 'fumadocs-ui/i18n';
import { i18n } from '@/lib/i18n';

export const gitConfig = {
  user: 'JungHoonGhae',
  repo: 'tossinvest-cli',
  branch: 'main',
};

// A GitHub API failure (rate limit, network blip) must never take the whole
// site down — fumadocs' own GithubInfo throws on fetch failure, which 500s
// every page since this renders in the shared nav. Same star-count UI, but
// swallows failures and falls back to icon + repo name with no star count.
async function humanizeStars(stars: number): Promise<string> {
  if (stars < 1000) return stars.toString();
  if (stars < 100000) {
    const value = (stars / 1000).toFixed(1);
    return value.endsWith('.0') ? `${value.slice(0, -2)}K` : `${value}K`;
  }
  return `${Math.floor(stars / 1000)}K`;
}

async function SafeGithubInfo({
  owner,
  repo,
  token,
  className,
}: {
  owner: string;
  repo: string;
  token?: string;
  className?: string;
}) {
  let stars: string | null = null;
  try {
    const headers = new Headers({ 'Content-Type': 'application/json' });
    if (token) headers.set('Authorization', `Bearer ${token}`);
    const res = await fetch(`https://api.github.com/repos/${owner}/${repo}`, {
      headers,
      next: { revalidate: 60 },
    });
    if (res.ok) {
      const data = await res.json();
      stars = await humanizeStars(data.stargazers_count);
    }
  } catch {
    // network error, rate limit, etc. — render without the star count below
  }

  return (
    <a
      href={`https://github.com/${owner}/${repo}`}
      rel="noreferrer noopener"
      target="_blank"
      className={
        'flex flex-col gap-1.5 rounded-lg p-2 text-sm text-fd-foreground/80 transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground lg:flex-row lg:items-center ' +
        (className ?? '')
      }
    >
      <p className="flex items-center gap-2 truncate">
        <Github className="size-3.5" />
        {owner}/{repo}
      </p>
      {stars !== null && (
        <p className="flex items-center gap-1 text-xs text-fd-muted-foreground">
          <Star className="size-3" />
          {stars}
        </p>
      )}
    </a>
  );
}

// i18n UI translations (ko default + en). `provider(locale)` feeds RootProvider.
export const { provider } = defineI18nUI(i18n, {
  translations: {
    ko: {
      displayName: '한국어',
      search: '검색',
      searchNoResult: '검색 결과가 없습니다',
      toc: '목차',
      lastUpdate: '마지막 업데이트',
      previousPage: '이전',
      nextPage: '다음',
      chooseTheme: '테마',
      chooseLanguage: '언어',
    },
    en: { displayName: 'English' },
  },
});

const navText = {
  ko: {
    docs: '문서',
    docsBlurb: '설치·사용·명령 레퍼런스·안전 모델을 한곳에서.',
    quickstart: '시작하기',
    quickstartBlurb: '설치하고 첫 조회까지 빠르게.',
    agents: 'AI 에이전트',
    agentsBlurb: '에이전트 연동 규칙과 출력 계약.',
    commands: '명령 레퍼런스',
    commandsShort: '명령',
    commandsBlurb: '전체 명령과 출력 형식.',
    safety: '안전 모델',
    safetyBlurb: '거래 보호 장치와 게이트.',
    changelog: '변경 이력',
  },
  en: {
    docs: 'Docs',
    docsBlurb: 'Install, usage, command reference, and the safety model — all in one place.',
    quickstart: 'Quick Start',
    quickstartBlurb: 'Install and run your first query fast.',
    agents: 'AI Agent Guide',
    agentsBlurb: 'Agent integration rules and output contracts.',
    commands: 'Command Reference',
    commandsShort: 'Commands',
    commandsBlurb: 'Every command and output format.',
    safety: 'Safety Model',
    safetyBlurb: 'Trading safeguards and gates.',
    changelog: 'Changelog',
  },
} as const;

// i18n config hides the "ko" prefix (hideLocale: 'default-locale') — only
// "en" gets prefixed. Every nav href must go through this, or switching to
// English and then clicking any nav link silently drops back to Korean.
function withLocale(path: string, lang?: string): string {
  return lang === 'en' ? `/en${path}` : path;
}

export function linkItems(lang?: string): LinkItemType[] {
  const t = navText[lang === 'en' ? 'en' : 'ko'];
  const href = (path: string) => withLocale(path, lang);
  return [
    {
      type: 'custom',
      on: 'nav',
      children: (
        <NavbarMenu>
          <NavbarMenuTrigger>
            <Link href={href('/docs')}>{t.docs}</Link>
          </NavbarMenuTrigger>
          <NavbarMenuContent>
            <NavbarMenuLink href={href('/docs')} className="md:row-span-2">
              <div className="-mx-3 -mt-3 overflow-hidden rounded-t-lg border-b bg-[#0b1220] p-3 text-white">
                <div className="mb-3 flex items-center gap-2 text-sm font-medium">
                  <TossctlIcon className="size-4 shrink-0" />
                  tossctl
                </div>
                <div className="rounded-lg border border-white/10 bg-black/30 p-3 font-mono text-[11px] leading-relaxed text-white/70">
                  <div><span className="text-white/40">$</span> tossctl account summary</div>
                  <div><span className="text-white/40">$</span> tossctl quote get 005930</div>
                  <div><span className="text-white/40">$</span> tossctl market index nasdaq</div>
                </div>
              </div>
              <p className="font-medium">{t.docs}</p>
              <p className="text-fd-muted-foreground text-sm">{t.docsBlurb}</p>
            </NavbarMenuLink>

            <NavbarMenuLink href={href('/docs/getting-started/quickstart')} className="lg:col-start-2">
              <Rocket className="bg-fd-primary text-fd-primary-foreground p-1 mb-2 rounded-md" />
              <p className="font-medium">{t.quickstart}</p>
              <p className="text-fd-muted-foreground text-sm">{t.quickstartBlurb}</p>
            </NavbarMenuLink>

            <NavbarMenuLink href={href('/docs/guide/agents')} className="lg:col-start-2">
              <Bot className="bg-fd-primary text-fd-primary-foreground p-1 mb-2 rounded-md" />
              <p className="font-medium">{t.agents}</p>
              <p className="text-fd-muted-foreground text-sm">{t.agentsBlurb}</p>
            </NavbarMenuLink>

            <NavbarMenuLink href={href('/docs/reference/commands')} className="lg:col-start-3 lg:row-start-1">
              <TerminalSquare className="bg-fd-primary text-fd-primary-foreground p-1 mb-2 rounded-md" />
              <p className="font-medium">{t.commands}</p>
              <p className="text-fd-muted-foreground text-sm">{t.commandsBlurb}</p>
            </NavbarMenuLink>

            <NavbarMenuLink href={href('/docs/guide/safety')} className="lg:col-start-3 lg:row-start-2">
              <ShieldCheck className="bg-fd-primary text-fd-primary-foreground p-1 mb-2 rounded-md" />
              <p className="font-medium">{t.safety}</p>
              <p className="text-fd-muted-foreground text-sm">{t.safetyBlurb}</p>
            </NavbarMenuLink>
          </NavbarMenuContent>
        </NavbarMenu>
      ),
    },
    {
      text: t.quickstart,
      url: href('/docs/getting-started/quickstart'),
      icon: <Rocket />,
      active: 'nested-url',
    },
    {
      text: t.agents,
      url: href('/docs/guide/agents'),
      icon: <Bot />,
      active: 'nested-url',
    },
    {
      text: t.commandsShort,
      url: href('/docs/reference/commands'),
      icon: <TerminalSquare />,
      active: 'nested-url',
    },
    {
      text: t.changelog,
      url: href('/docs/changelog'),
      icon: <History />,
      active: 'nested-url',
    },
  ];
}

export const logoIcon = <TossctlIcon className="size-5 shrink-0" />;

export const logo = (
  <span className="inline-flex items-center gap-2">
    {logoIcon}
    <span className="font-medium">tossctl</span>
  </span>
);

export function baseOptions(): BaseLayoutProps {
  return {
    i18n: true,
    nav: {
      title: logo,
      children: (
        <SafeGithubInfo
          owner={gitConfig.user}
          repo={gitConfig.repo}
          token={process.env.GITHUB_TOKEN}
          className="max-lg:hidden"
        />
      ),
    },
  };
}
