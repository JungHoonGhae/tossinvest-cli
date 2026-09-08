// Run against a production build (pnpm start), or pass the deployed origin.
import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';

const origin = process.argv[2] ?? 'http://localhost:3000';
const docsRoot = new URL('../content/docs/', import.meta.url);
const pages = readdirSync(docsRoot, { recursive: true }).filter((file) => file.endsWith('.mdx'));
const fetchText = async (path) => {
  const response = await fetch(new URL(path, origin), { signal: AbortSignal.timeout(30_000) });
  assert.equal(response.status, 200, `${path}: HTTP ${response.status}`);
  return response.text();
};
const checkGithubIcon = (html, path) => {
  const links = html.match(/<a\b[^>]*href="https:\/\/github\.com\/JungHoonGhae\/tossinvest-cli"[^>]*>[\s\S]*?<\/a>/g) ?? [];
  assert.ok(links.length > 0, `${path}: GitHub repository link`);
  for (const link of links) {
    assert.match(link, /<svg\b[^>]*viewBox="0 0 24 24"[^>]*aria-hidden="true"/, `${path}: decorative GitHub icon`);
    assert.ok(link.includes('M9 18c-4.51 2-5-2-7-2'), `${path}: preserved GitHub icon path`);
  }
};
const fullText = await fetchText('/llms-full.txt');
const sitemap = await fetchText('/sitemap.xml');
await fetchText('/llms.txt');
for (const path of ['/', '/en']) checkGithubIcon(await fetchText(path), path);

for (const file of pages) {
  const english = file.endsWith('.en.mdx');
  const locale = english ? 'en' : 'ko';
  const slug = file.replace(/(?:\.en)?\.mdx$/, '').replace(/^index$/, '');
  const path = `${english ? '/en' : ''}/docs${slug ? `/${slug}` : ''}`;
  const resource = `${english ? 'en/' : ''}${slug ? `${slug}/` : ''}`;
  const markdownPath = `/llms.mdx/docs/${resource}index.mdx`;
  const imagePath = `/og/docs/${resource}image.png`;
  const title = readFileSync(new URL(file, docsRoot), 'utf8').match(/^title: (.+)$/m)[1];
  const html = await fetchText(path);
  checkGithubIcon(html, path);
  assert.ok(html.includes(`<html lang="${locale}"`), `${path}: document language`);
  assert.ok(html.includes(markdownPath), `${path}: localized Markdown link`);
  assert.ok(html.includes(`website-fumadocs/content/docs/${file}`), `${path}: GitHub source link`);
  assert.ok(html.includes(imagePath), `${path}: localized OG image`);
  const canonical = html.match(/<link rel="canonical" href="([^"]+)"/);
  assert.ok(canonical, `${path}: canonical metadata`);
  assert.equal(new URL(canonical[1]).pathname, path, `${path}: canonical URL`);
  assert.ok(html.includes('hrefLang="ko"') && html.includes('hrefLang="en"'), `${path}: language alternates`);
  assert.ok(sitemap.includes(`${path}</loc>`), `${path}: sitemap entry`);
  const markdown = await fetchText(markdownPath);
  assert.ok(markdown.startsWith(`# ${title}\n`), `${path}: Markdown title`);
  assert.ok(markdown.includes(`Source: ${canonical[1]}`), `${path}: Markdown origin`);
  assert.ok(fullText.includes(`Source: ${canonical[1]}`), `${path}: full LLM corpus`);
  const image = await fetch(new URL(imagePath, origin), { signal: AbortSignal.timeout(30_000) });
  assert.equal(image.status, 200, `${imagePath}: HTTP status`);
  assert.match(image.headers.get('content-type') ?? '', /^image\/png/, `${imagePath}: image response`);
  await image.arrayBuffer();
  console.log(`OK ${path} + Markdown + OG`);
}

for (const path of ['/llms.mdx/docs/guide/mcp/bad.mdx', '/og/docs/guide/mcp/bad.png']) {
  const response = await fetch(new URL(path, origin), { signal: AbortSignal.timeout(30_000) });
  assert.equal(response.status, 404, `${path}: invalid resource must not resolve`);
}
console.log(`Verified ${pages.length} localized docs, homepages, sitemap, and LLM resources at ${origin}`);
