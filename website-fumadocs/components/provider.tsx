'use client';
import dynamic from 'next/dynamic';
import { useEffect, type ReactNode } from 'react';
import { RootProvider } from 'fumadocs-ui/provider/base';
import { useSearchContext } from 'fumadocs-ui/contexts/search';
import { FrameworkProvider, type Framework } from 'fumadocs-core/framework';
import Link from 'next/link';
import Image from 'next/image';
import { useParams, usePathname, useRouter } from 'next/navigation';
import { i18n } from '@/lib/i18n';

type I18nProp = React.ComponentProps<typeof RootProvider>['i18n'];

// The proxy serves /docs/... from the prerendered /ko/docs/... page, so usePathname() is
// /ko/... on the server but /docs/... in the browser. That mismatch broke hydration on every
// Korean docs page (see Next's use-pathname docs, "Avoid hydration mismatch with rewrites").
// Page-tree URLs omit the default locale, so strip it once here for all fumadocs consumers.
const defaultLocalePrefix = new RegExp(`^/${i18n.defaultLanguage}(?=/|$)`);

function useBrowserPathname() {
  return usePathname().replace(defaultLocalePrefix, '') || '/';
}

// Same Link/Image as fumadocs' NextProvider; only usePathname differs. The casts match its internal typing.
export function FumadocsNextProvider({ children }: { children: ReactNode }) {
  return (
    <FrameworkProvider
      usePathname={useBrowserPathname}
      useRouter={useRouter}
      useParams={useParams}
      Link={Link as Framework['Link']}
      Image={Image as Framework['Image']}
    >
      {children}
    </FrameworkProvider>
  );
}

const SearchDialog = dynamic(() => import('@/components/search'), {
  ssr: false,
});

function SearchHotKey() {
  const { setOpenSearch, enabled } = useSearchContext();

  useEffect(() => {
    if (!enabled) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpenSearch(true);
      }
    };

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [enabled, setOpenSearch]);

  return null;
}

export function Provider({ children, i18n }: { children: ReactNode; i18n?: I18nProp }) {
  return (
    <RootProvider search={{ SearchDialog }} i18n={i18n}>
      <SearchHotKey />
      {children}
    </RootProvider>
  );
}
