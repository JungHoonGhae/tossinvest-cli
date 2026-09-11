# README diagrams

README에는 **사용 흐름과 연결 구조 두 장**을 표시합니다. 문구·레이아웃·생성 도구를
모두 저장소에서 관리하므로, 수정 후 한 명령으로 HTML과 PNG를 다시 만들 수 있습니다.

The README shows **two figures: the read workflow and the connections**. Copy,
layout, and build tools live in the repository. One command rebuilds the
standalone HTML and PNG exports after an edit.

## 수정하고 다시 만들기 / Edit and rebuild

1. 문구는 [`readme-content.json`](readme-content.json)의 `ko` / `en`에서 수정합니다.
   각 언어의 `workflow`는 사용 흐름, `overview`는 연결 구조입니다.
   Edit the bilingual copy in this JSON; the same keys exist in both languages.
2. 배치·색상·아이콘은 [`build_readme_diagrams.py`](../tools/build_readme_diagrams.py)에서
   수정합니다. `text`, `rect`, `icon`, `arrow`는 다른 그림에도 재사용할 수 있는 SVG
   구성 요소입니다. Edit the builder for layout, palette, or reusable SVG primitives.
3. 저장소 루트에서 실행합니다. From the repository root, run:

   ```bash
   make readme-diagrams
   ```

   한·영 HTML 생성 → 한국어 폰트 포함 → 2× PNG 내보내기를 순서대로 실행합니다.
   It builds both languages, embeds Korean fonts, and exports the four PNGs.
4. README 폭에서 이미지를 확인하고 문구·원본·이미지를 함께 커밋합니다.
   Review at README width and commit the copy, builder changes, and outputs together.

<details>
<summary>최초 환경 설정 · 개별 실행 / First-time setup and individual steps</summary>

Python 3.11+를 사용합니다. Use Python 3.11+:

```bash
python3 -m venv .venv-readme
.venv-readme/bin/python -m pip install -r tools/requirements-readme-diagrams.txt
.venv-readme/bin/python -m playwright install chromium
make readme-diagrams PYTHON=.venv-readme/bin/python
```

Playwright is pinned in the requirements file. Font embedding downloads the
unmodified Pretendard v1.3.9 subsets; Latin fonts load from Google Fonts during
rendering. PNGs need neither installed fonts nor network access.

For individual steps:

```bash
python3 tools/build_readme_diagrams.py
python3 tools/embed_readme_fonts.py
python3 tools/render_readme_diagrams.py
python3 -m unittest tools.tests.test_readme_visuals
```

The HTML builder uses only the standard library and works offline. Font
embedding and PNG rendering need network access. The renderer waits for fonts
and fails if a required family is unavailable. Generated HTML is portable and
can be edited directly for a one-off copy; permanent changes belong in the JSON
or builder, because the next full build replaces the generated HTML.

</details>

## Outputs

| Figure | 한국어 원본 | English source | Code reference |
|---|---|---|---|
| 로그인 → 조회 → 활용 / Read workflow | [HTML](readme-workflow.html) · [PNG](readme-workflow.png) | [HTML](readme-workflow.en.html) · [PNG](readme-workflow.en.png) | [Agent guide](../AGENTS.md), [MCP catalog](../internal/mcp/catalog.go) |
| CLI·MCP와 API 연결 / Connections | [HTML](readme-overview.html) · [PNG](readme-overview.png) | [HTML](readme-overview.en.html) · [PNG](readme-overview.en.png) | [Hybrid routing](../internal/hybrid/client.go), [architecture](../docs/architecture.md) |

## Design and scope

- Types: a simplified **flowchart** for the linear read workflow and an
  **architecture** diagram for connections. No specialized semantic pattern is
  needed. Each has at most five nodes and four arrows.
- Frame: `fit`, `960 × 376` for the workflow and `960 × 456` for connections.
  PNG export at 2× (`1920 × 752` and `1920 × 912`).
- Palette: paper `#f5f5f5`, ink `#2d3142`, muted `#4f5d75`, accent `#eb6c36`,
  The user selected the shipped default palette.
- The user's visual reference calls for simple figures where they help. This
  Diagram Design editorial variation uses line icons and a dark focus group.
  All arrows show the forward workflow or a connection, so the separate legend
  and individual arrow labels have been omitted.
- Korean fonts: **Pretendard Variable** for titles, names, and prose, as
  requested. Unmodified [Pretendard v1.3.9](https://github.com/orioncactus/pretendard/tree/v1.3.9)
  subsets and their SIL Open Font License are embedded in each Korean HTML.
- English fonts: Instrument Serif for titles and Geist for names and prose.
  Technical labels use Geist Mono in both languages. These Latin fonts use
  Google Fonts and may substitute local fonts offline. PNG typography is fixed.
- Figures are static and contain inline SVG with descriptive `title` / `desc`
  elements. README images have equivalent alt text and adjacent explanations.

The workflow shows representative **reads through a web session**, from phone
approval to results. Official-key setup, session renewal, command options, and
live-order execution are omitted. AI answers are written by the connected agent.

The overview summarizes **default read routing**. Both CLI and MCP use the same
binary; agents can also use the CLI. Official keys are preferred for supported
reads, and WTS-only capabilities use a web session. Per-command routing,
authentication helpers, and local history are omitted. MCP, ops, and conditional
orders use the official API only. The README keeps the user-facing order checks;
full execution requirements live in the safety guide and configuration docs.

The `30+` label counts supported, official-API-absent capability rows in the
[support matrix](../website-fumadocs/content/docs/reference/support-scope.mdx),
excluding local CSV export, order preview, and experimental paper trading.
It describes user capabilities, not a count of distinct HTTP endpoints or a
claim that no other project offers them. Recheck the matrix against the current
official contract when updating this label.

## Validation

When Diagram Design is installed, run its `scripts/self_check.py` on each HTML
source. Its package-level `scripts/verify-geometry.py` checks label-mask
placement, and `scripts/lint-skin.py` checks the style contract.

The Korean typography intentionally overrides the skill's default font rules.
Its stock CSS allowlist reports the embedded WOFF2 `url(data:font/woff2;...)`
declarations as non-fragment CSS URLs. Review those font-only findings against
the embedded source/license; other findings still need fixing. The fonts are
packaged locally in the HTML, with no additional remote stylesheet or script.

## Earlier diagrams

The earlier [MCP sequence](mcp-discovery.html), [live-order flow](order-safety.html),
their English versions and PNGs, and `official-vs-wts-v2.*` are retained as
reference artwork. They are not embedded in the current READMEs or regenerated
by the commands above. See the MCP and safety guides for their detailed policies.
