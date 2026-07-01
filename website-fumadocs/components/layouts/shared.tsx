import type { BaseLayoutProps, LinkItemType } from 'fumadocs-ui/layouts/shared';
import { Bot, Github, History, Rocket, ShieldCheck, TerminalSquare } from 'lucide-react';
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
import { GithubInfo } from 'fumadocs-ui/components/github-info';

export const gitConfig = {
  user: 'JungHoonGhae',
  repo: 'tossinvest-cli',
  branch: 'main',
};

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

export const linkItems: LinkItemType[] = [
  {
    type: 'custom',
    on: 'nav',
    children: (
      <NavbarMenu>
        <NavbarMenuTrigger>
          <Link href="/docs">문서</Link>
        </NavbarMenuTrigger>
        <NavbarMenuContent>
          <NavbarMenuLink href="/docs" className="md:row-span-2">
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
            <p className="font-medium">문서</p>
            <p className="text-fd-muted-foreground text-sm">
              설치·사용·명령 레퍼런스·안전 모델을 한곳에서.
            </p>
          </NavbarMenuLink>

          <NavbarMenuLink href="/docs/getting-started/quickstart" className="lg:col-start-2">
            <Rocket className="bg-fd-primary text-fd-primary-foreground p-1 mb-2 rounded-md" />
            <p className="font-medium">시작하기</p>
            <p className="text-fd-muted-foreground text-sm">설치하고 첫 조회까지 빠르게.</p>
          </NavbarMenuLink>

          <NavbarMenuLink href="/docs/guide/agents" className="lg:col-start-2">
            <Bot className="bg-fd-primary text-fd-primary-foreground p-1 mb-2 rounded-md" />
            <p className="font-medium">AI 에이전트</p>
            <p className="text-fd-muted-foreground text-sm">에이전트 연동 규칙과 출력 계약.</p>
          </NavbarMenuLink>

          <NavbarMenuLink href="/docs/reference/commands" className="lg:col-start-3 lg:row-start-1">
            <TerminalSquare className="bg-fd-primary text-fd-primary-foreground p-1 mb-2 rounded-md" />
            <p className="font-medium">명령 레퍼런스</p>
            <p className="text-fd-muted-foreground text-sm">전체 명령과 출력 형식.</p>
          </NavbarMenuLink>

          <NavbarMenuLink href="/docs/guide/safety" className="lg:col-start-3 lg:row-start-2">
            <ShieldCheck className="bg-fd-primary text-fd-primary-foreground p-1 mb-2 rounded-md" />
            <p className="font-medium">안전 모델</p>
            <p className="text-fd-muted-foreground text-sm">거래 보호 장치와 게이트.</p>
          </NavbarMenuLink>
        </NavbarMenuContent>
      </NavbarMenu>
    ),
  },
  {
    text: '시작하기',
    url: '/docs/getting-started/quickstart',
    icon: <Rocket />,
    active: 'nested-url',
  },
  {
    text: 'AI 에이전트',
    url: '/docs/guide/agents',
    icon: <Bot />,
    active: 'nested-url',
  },
  {
    text: '명령',
    url: '/docs/reference/commands',
    icon: <TerminalSquare />,
    active: 'nested-url',
  },
  {
    text: '변경 이력',
    url: '/docs/changelog',
    icon: <History />,
    active: 'nested-url',
  },
  {
    type: 'icon',
    url: `https://github.com/${gitConfig.user}/${gitConfig.repo}`,
    label: 'github',
    text: 'GitHub',
    icon: <Github />,
    external: true,
  },
];

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
        <GithubInfo
          owner={gitConfig.user}
          repo={gitConfig.repo}
          className="max-lg:hidden"
        />
      ),
    },
  };
}
