#!/usr/bin/env node
// Generates content/docs/changelog.mdx (+ .en.mdx) from the repo-root
// CHANGELOG.md. Do not hand-edit the generated .mdx files — edit
// ../../CHANGELOG.md and re-run this script (or `pnpm build` / `pnpm dev`).
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..', '..');
const changelogPath = path.join(repoRoot, 'CHANGELOG.md');
const outDir = path.join(__dirname, '..', 'content', 'docs');

// Some deploy environments (e.g. Vercel with this project's subdirectory as
// the build root) only check out website-fumadocs/, not the monorepo root —
// so ../../CHANGELOG.md isn't reachable there. Skip regeneration rather than
// failing the whole build; the already-committed content/docs/changelog.mdx
// stays as the fallback.
if (!existsSync(changelogPath)) {
  console.warn(`sync-changelog: ${changelogPath} not found (outside build root?) — skipping, keeping committed changelog.mdx`);
  process.exit(0);
}

function loadBody() {
  const raw = readFileSync(changelogPath, 'utf8');
  const idx = raw.indexOf('## [');
  if (idx === -1) {
    throw new Error(`sync-changelog: no "## [" version heading found in ${changelogPath}`);
  }
  return raw.slice(idx).trimEnd() + '\n';
}

const GENERATED_NOTICE =
  '{/* AUTO-GENERATED from ../../../CHANGELOG.md by scripts/sync-changelog.mjs — do not edit directly. */}\n\n';

function writeKo(body) {
  const frontmatter = `---\ntitle: 변경 이력\ndescription: tossctl 버전별 변경 사항 (사용자 관점)\n---\n\n`;
  const intro =
    '버전별 변경 사항입니다. 각 릴리즈의 바이너리·릴리즈 노트는 ' +
    '[GitHub Releases](https://github.com/JungHoonGhae/tossinvest-cli/releases)에서도 볼 수 있습니다.\n\n';
  writeFileSync(
    path.join(outDir, 'changelog.mdx'),
    frontmatter + GENERATED_NOTICE + intro + body,
  );
}

function writeEn(body) {
  const frontmatter = `---\ntitle: Changelog\ndescription: Version history for tossctl, from the user's perspective\n---\n\n`;
  const intro =
    'Version history below (source: the Korean-language ' +
    '[CHANGELOG.md](https://github.com/JungHoonGhae/tossinvest-cli/blob/main/CHANGELOG.md) — entries are not yet translated). ' +
    'Binaries and release notes for each version are also on ' +
    '[GitHub Releases](https://github.com/JungHoonGhae/tossinvest-cli/releases).\n\n';
  writeFileSync(
    path.join(outDir, 'changelog.en.mdx'),
    frontmatter + GENERATED_NOTICE + intro + body,
  );
}

const body = loadBody();
writeKo(body);
writeEn(body);
console.log('sync-changelog: wrote content/docs/changelog.mdx + changelog.en.mdx');
