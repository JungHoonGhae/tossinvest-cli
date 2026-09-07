import { source } from '@/lib/source';
import { llms } from 'fumadocs-core/source';
import { siteDescription } from '@/lib/site';

export const revalidate = false;

// Curated index (llmstxt.org style). Prepend a one-line project summary and a
// pointer to the single-file corpus so an agent landing here has immediate
// context before the per-page links.
export function GET() {
  const header =
    `> ${siteDescription.ko}\n> ${siteDescription.en}\n\nKorean and English documentation: /llms-full.txt\n\n`;

  return new Response(header + llms(source).index(), {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
}
