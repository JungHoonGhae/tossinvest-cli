import type { Metadata } from 'next';
import type { Viewport } from 'next';
import { Space_Grotesk, JetBrains_Mono } from 'next/font/google';
import { Provider } from '@/components/provider';
import { Body } from '@/app/layout.client';
import { source } from '@/lib/source';
import { NextProvider } from 'fumadocs-core/framework/next';
import { TreeContextProvider } from 'fumadocs-ui/contexts/tree';
import { provider } from '@/components/layouts/shared';
import { siteUrl, siteDescription } from '@/lib/site';
import '../global.css';

const geist = Space_Grotesk({
  variable: '--font-sans',
  subsets: ['latin'],
});

const mono = JetBrains_Mono({
  variable: '--font-mono',
  subsets: ['latin'],
});

export async function generateMetadata({ params }: LayoutProps<'/[lang]'>): Promise<Metadata> {
  const { lang } = await params;
  return {
    title: {
      default: 'tossinvest-cli',
      template: '%s | tossinvest-cli',
    },
    description: siteDescription[lang === 'en' ? 'en' : 'ko'],
    metadataBase: new URL(siteUrl),
    icons: {
      icon: '/favicon.svg',
    },
    // Machine-discoverable pointer to the LLM-friendly markdown index.
    alternates: {
      canonical: lang === 'en' ? '/en' : '/',
      languages: { ko: '/', en: '/en' },
      types: {
        'text/markdown': '/llms.txt',
      },
    },
  };
}

export const viewport: Viewport = {
  themeColor: [
    { media: '(prefers-color-scheme: dark)', color: '#0A0A0A' },
    { media: '(prefers-color-scheme: light)', color: '#fff' },
  ],
};

export default async function Layout({ params, children }: LayoutProps<'/[lang]'>) {
  const { lang } = await params;
  return (
    <html lang={lang} className={`${geist.variable} ${mono.variable}`} suppressHydrationWarning>
      <Body>
        <NextProvider>
          <TreeContextProvider tree={source.getPageTree(lang)}>
            <Provider i18n={provider(lang)}>{children}</Provider>
          </TreeContextProvider>
        </NextProvider>
      </Body>
    </html>
  );
}
