# tossinvest-cli Docs

Fumadocs-based documentation site for `tossinvest-cli`.

## Commands

```bash
pnpm install
pnpm dev
pnpm types:check
pnpm build
pnpm start --hostname localhost  # production Next.js server, not static export
# In another terminal, with the server running:
pnpm check:docs
# Or check a deployment built from the same revision:
pnpm check:docs https://tossinvest-cli.vercel.app
```

## Notes

- The site is deployed on Vercel as a Next.js application.
- Public documentation content lives in `content/docs`.
- `pnpm build` synchronizes the root changelog before running the production build.
- Korean pages use `/docs`; English `.en.mdx` pages use `/en/docs`. Update both languages together.
- Keep authentication, routing, safety, config defaults, and examples aligned with CLI help,
  `schemas/config.schema.json`, and the operation catalog. Implementation does not imply live availability.
- Markdown resources use `/llms.mdx/docs/[slug]/index.mdx` (Korean) or
  `/llms.mdx/docs/en/[slug]/index.mdx` (English). OG images follow the same locale convention.
- `pnpm check:docs` checks every page's language, source link, canonical URL, Markdown, OG image,
  sitemap entry, and inclusion in the bilingual LLM corpus. CI runs it after building the site.
- From the repository root, `node --test tools/tests/website_doc_resources.test.mjs` checks
  resource paths, translation pairs, document links, and configuration defaults without a server.
