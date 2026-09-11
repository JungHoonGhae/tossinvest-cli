# README diagrams

README의 세 그림은 Diagram Design의 **minimal light** 스타일로 작성했습니다.
한국어와 영어는 같은 구조를 사용합니다. HTML이 편집 원본이고, README에는
글꼴이 고정된 PNG를 넣습니다.

The three README figures use Diagram Design's **minimal light** style. Korean
and English share the same layout. Edit the standalone HTML sources; the
READMEs embed PNGs to preserve typography across viewers.

| Figure | 한국어 원본 | English source | Code reference |
|---|---|---|---|
| CLI·MCP와 API 연결 / API routing | [HTML](readme-overview.html) · [PNG](readme-overview.png) | [HTML](readme-overview.en.html) · [PNG](readme-overview.en.png) | [Hybrid routing](../internal/hybrid/client.go), [architecture](../docs/architecture.md) |
| MCP 카탈로그 호출 / Catalog discovery | [HTML](mcp-discovery.html) · [PNG](mcp-discovery.png) | [HTML](mcp-discovery.en.html) · [PNG](mcp-discovery.en.png) | [MCP server](../internal/mcp/server.go), [catalog](../internal/mcp/catalog.go) |
| 실주문 승인 / Live-order approval | [HTML](order-safety.html) · [PNG](order-safety.png) | [HTML](order-safety.en.html) · [PNG](order-safety.en.png) | [Trading checks](../internal/trading/service.go), [backend selection](../internal/hybrid/policy.go), [agent rules](../AGENTS.md) |

## Design and scope

- Types: **architecture**, **sequence**, **flowchart**. None of the specialized
  semantic patterns adds useful meaning to these three figures.
- Frame: `doc-inline`, `960 × 600`; PNG export at 2× (`1920 × 1200`). Each figure
  has at most five nodes and six message/flow arrows, plus its legend.
- Palette: paper `#f5f5f5`, ink `#2d3142`, muted `#4f5d75`, accent `#eb6c36`,
  external API links `#2e5aa8`. The user selected the shipped default palette.
- Korean fonts: **Pretendard Variable** for titles, names, and prose, as
  requested. Unmodified [Pretendard v1.3.9](https://github.com/orioncactus/pretendard/tree/v1.3.9)
  subsets and their SIL Open Font License are embedded in each Korean HTML.
- English fonts: Instrument Serif for titles and Geist for names and prose.
  Technical labels use Geist Mono in both languages. These Latin fonts use
  Google Fonts and may substitute local fonts offline. PNG typography is fixed.
- Figures are static and contain inline SVG with descriptive `title` / `desc`
  elements. README images have equivalent alt text and adjacent explanations.

The overview summarizes default read routing and representative capabilities.
It omits authentication helpers, individual endpoints, local history storage,
and experimental paper trading. Agents may also use the CLI. The MCP sequence
uses a **dividends read**, omitting API transport and authentication.

The safety flow shows **regular CLI live orders**. Human approval is an agent
operating rule; code enforces configuration, execution flags, and confirmation
tokens. Product, sell, and fractional eligibility checks and API responses are
outside the figure. MCP, ops, and conditional orders use the official API only.
Settings and paper-trading requirements remain in the README's safety table.

The previous `official-vs-wts-v2.*` files are retained as earlier artwork; the
READMEs use the new sources above.

## Edit and render

1. Edit the Korean and English HTML together. Keep labels on the 4px grid and
   preserve the prefixed accessible title/description IDs.
2. After editing Korean text, refresh the embedded font subsets from the
   pinned upstream release (standard-library Python only):

   ```bash
   python3 tools/embed_readme_fonts.py
   ```

3. With Python Playwright and its Chromium browser already installed, run:

   ```bash
   python3 tools/render_readme_diagrams.py
   ```

   The renderer requires access to Google Fonts and stops if a required family
   fails to load. It screenshots only the SVG and writes all six PNGs beside
   their HTML sources. No account access or live API request is involved.
4. Inspect all PNGs at README width, then run the existing README checks:

   ```bash
   python3 -m unittest tools.tests.test_readme_visuals
   ```

When Diagram Design is installed, run its `scripts/self_check.py` on each HTML
source. Its package-level `scripts/verify-geometry.py` checks label-mask
placement, and `scripts/lint-skin.py` checks the style contract.

The Korean typography intentionally overrides the skill's default font rules.
Its stock CSS allowlist reports the embedded WOFF2 `url(data:font/woff2;...)`
declarations as non-fragment CSS URLs. Review those font-only findings against
the embedded source/license; other findings still need fixing. The fonts are
packaged locally in the HTML, with no additional remote stylesheet or script.
