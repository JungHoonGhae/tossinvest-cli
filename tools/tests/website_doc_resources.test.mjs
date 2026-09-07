import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync, readdirSync } from 'node:fs';
import { parseResourceSegments, resourceSegments } from '../../website-fumadocs/lib/doc-resources.ts';

test('localized Markdown and OG paths round-trip without changing Korean URLs', () => {
  for (const locale of ['ko', 'en']) {
    for (const slugs of [[], ['guide', 'mcp']]) {
      for (const file of ['index.mdx', 'image.png']) {
        const segments = resourceSegments(locale, slugs, file);
        assert.deepEqual(parseResourceSegments(segments, file), { locale, slugs });
        assert.deepEqual(segments, [...(locale === 'en' ? ['en'] : []), ...slugs, file]);
      }
    }
  }
});

const docsRoot = new URL('../../website-fumadocs/content/docs/', import.meta.url);
const pages = readdirSync(docsRoot, { recursive: true }).filter((file) => file.endsWith('.mdx'));

test('every documentation page has a matching translation and valid localized links', () => {
  for (const file of pages) {
    const english = file.endsWith('.en.mdx');
    const counterpart = english ? file.replace('.en.mdx', '.mdx') : file.replace('.mdx', '.en.mdx');
    assert.ok(pages.includes(counterpart), `${file}: missing ${counterpart}`);
    // Historical changelogs are generated from the original release notes.
    if (file.startsWith('changelog')) continue;
    const text = readFileSync(new URL(file, docsRoot), 'utf8');
    for (const [, locale, slug = ''] of text.matchAll(/\]\(\/(en\/)?docs(?:\/([^\s)#]*))?(?:#[^\s)]*)?\)/g)) {
      assert.equal(Boolean(locale), english, `${file}: cross-language /${locale ?? ''}docs/${slug}`);
      const target = `${slug || 'index'}${english ? '.en' : ''}.mdx`;
      assert.ok(pages.includes(target), `${file}: missing link target ${target}`);
    }
  }
});

test('both configuration examples track the schema and safe defaults', () => {
  const schema = JSON.parse(readFileSync(new URL('../../schemas/config.schema.json', import.meta.url), 'utf8'));
  function defaults(node) {
    if ('const' in node) return node.const;
    if ('default' in node) return node.default;
    return Object.fromEntries(Object.entries(node.properties).filter(([key]) => key !== '$schema').map(([key, value]) => [key, defaults(value)]));
  }
  for (const locale of ['', '.en']) {
    const text = readFileSync(new URL(`guide/configuration${locale}.mdx`, docsRoot), 'utf8');
    const example = JSON.parse(text.match(/```json\n([\s\S]*?)\n```/)[1]);
    assert.deepEqual(example, defaults(schema));
  }
});

test('missing or wrong resource filenames do not resolve to a documentation page', () => {
  for (const segments of [undefined, [], ['guide', 'mcp'], ['guide', 'mcp', 'bad.mdx']]) {
    assert.equal(parseResourceSegments(segments, 'index.mdx'), undefined);
  }
});
