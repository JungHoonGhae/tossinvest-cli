import { NextResponse, type NextFetchEvent, type NextRequest } from 'next/server';
import { createI18nMiddleware } from 'fumadocs-core/i18n/middleware';
import { i18n } from '@/lib/i18n';

const i18nProxy = createI18nMiddleware(i18n);

// Some external tools that scan the Next.js app directory for routes don't
// know that a parenthesized folder (app/[lang]/(home)/page.tsx) is a route
// group excluded from the URL — they read the folder name literally and
// guess "/home" (or "/en/home") as the homepage path. That path doesn't
// exist in this app (the real homepage is "/" or "/en"), so redirect it
// here rather than relying on the external tool to be fixed.
const HOME_ALIAS = /^\/(?:(en)\/)?home\/?$/;

export default function proxy(request: NextRequest, event: NextFetchEvent) {
  const match = request.nextUrl.pathname.match(HOME_ALIAS);
  if (match) {
    const url = request.nextUrl.clone();
    url.pathname = match[1] ? `/${match[1]}` : '/';
    return NextResponse.redirect(url);
  }
  return i18nProxy(request, event);
}

export const config = {
  // 라우트 핸들러(api·llms·og)·정적파일·_next 는 제외하고 페이지만 로케일 처리
  matcher: ['/((?!api|og|_next|.*\\.).*)'],
};
