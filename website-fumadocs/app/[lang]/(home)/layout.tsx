import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { baseOptions, linkItems, githubItem } from '@/components/layouts/shared';

export default async function Layout({ params, children }: LayoutProps<'/[lang]'>) {
  const { lang } = await params;
  return (
    <HomeLayout
      {...baseOptions()}
      links={[...linkItems(lang), githubItem()]}
      // 랜딩은 디자인 소스(MZ8Ua)에 맞춰 항상 다크 — 토글은 동작하지 않으므로 숨긴다.
      // (테마 토글은 docs 레이아웃에서는 그대로 동작)
      themeSwitch={{ enabled: false }}
      className="dark bg-[#0a0a0a] [--color-fd-background:#0a0a0a] [--color-fd-primary:var(--color-brand)]"
    >
      {children}
    </HomeLayout>
  );
}
