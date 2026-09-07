import { getLLMText, source } from '@/lib/source';
import { notFound } from 'next/navigation';
import { parseResourceSegments, resourceSegments } from '@/lib/doc-resources';

export const revalidate = false;

export async function GET(_req: Request, { params }: RouteContext<'/llms.mdx/docs/[[...slug]]'>) {
  const { slug } = await params;
  const resource = parseResourceSegments(slug, 'index.mdx');
  if (!resource) notFound();
  const page = source.getPage(resource.slugs, resource.locale);
  if (!page) notFound();

  return new Response(await getLLMText(page), {
    headers: {
      'Content-Type': 'text/markdown',
    },
  });
}

export function generateStaticParams() {
  return source.getPages().map((page) => ({
    slug: resourceSegments(page.locale, page.slugs, 'index.mdx'),
  }));
}
