# Q-SYS CLI Brief

> Reused from prior run `20260808-153253-09b3492e` (2 days old, same binary
> v4.30.1). Verified again this run: both origins HTTP 200, `probe-reachability`
> classifies `standard_http`. Landscape re-checked 2026-08-10: two new Q-SYS MCP
> servers exist (control-plane only — docs gap intact).

## API Identity
- Domain: `help.qsys.com` — QSC Q-SYS official documentation (MadCap Flare static site).
  `q-syshelp.qsc.com` 301s to the same host, so there is one canonical origin.
  Second source: `www.qsys.com` — QSC product pages (271) + spec-sheet PDFs.
- Users: AV systems integrators, Q-SYS programmers, control-system developers,
  commissioning techs. People who write Lua control scripts and QRC/QRWC
  integrations against Q-SYS Cores, and integrators quoting equipment.
- Data profile: 753 real `.htm` documentation pages (help.qsys.com) + 271
  product pages with linked spec-sheet PDFs (www.qsys.com).

  | Section | Pages | What it is |
  |---|---:|---|
  | `Schematic_Library` | 384 | Designer component reference — **the gravity center** |
  | `Hardware` | 71 | Cores, peripherals, touchscreens, cameras, amps |
  | `Control_Scripting` | 61 | Lua 5.3 reference + Q-SYS scripting API |
  | `Reflect` | 47 | Reflect Enterprise Manager monitoring |
  | `Application_Integration` | 34 | Third-party / UC integrations |
  | `Core_Manager` | 27 | Core admin UI |
  | `External_Control_APIs` | 11 | QRC, QRWC, ECP protocol docs |
  | `Management_APIs` | 4 | Media/playlist REST APIs |
  | everything else | 114 | Networking, security, redundancy, vCore, RoomSuite… |

## Reachability Risk
- **None.** This run's Phase 1.9 probe: `GET https://help.qsys.com/sitemap.xml`
  → 200 text/xml; `GET https://www.qsys.com/sitemap.xml` → 200 text/xml;
  `probe-reachability https://help.qsys.com` → `standard_http` (0.95), no
  browser, no clearance, no auth. No 403/429 anywhere in prior or current
  research.
- Versioned doc trees are live and fetchable: `/q-sys_9.4/`, `/q-sys_9.6/`,
  `/q-sys_10.0/` all return HTTP 200. Bare `/Content/...` is the current release.

## Source Priority (combo — confirmed 2026-08-08)
- Primary: `qsys-product-specs` (www.qsys.com) — product specs, spec-sheet PDFs.
  Auth: free / public.
- Secondary: `qsys-help-docs` (help.qsys.com) — configure, connect, compatibility.
  Auth: free / public.
- **Economics:** both sources public; no credentials required for either.
- **Inversion risk:** help.qsys.com has a clean complete sitemap; qsys.com specs
  require walking 271 product pages and extracting PDFs. Do NOT let source
  cleanliness demote specs — the user named specs first.
- **Spec storage decision:** searchable extracted text + source PDF URL; no
  fragile per-field parsing.
- **Dropped from scope:** QRC control API (no Core reachable for live
  verification).

## The Real Data Shape (verified, not assumed)
Component pages carry HTML tables of control pins. **The header schema varies**,
the single most important implementation fact:

| Header signature | Frequency in 15-page sample |
|---|---:|
| `Pin Name \| Value \| String \| Position \| Pins Available` | 16 tables (dominant) |
| `Control ID \| Pin Name \| Value \| String \| Position` | 1 |
| `Pin Name \| Control Type \| Value \| String \| Position` | 1 |
| unrelated tables (specs, Core-model limits, `id`-only) | 5 |

Also observed: some component pages have **zero** tables (`spaq_4ch_gpio`), and
some have **three or four** (`av_io`, `streamer_hdmi_switcher`). A parser that
greps for the literal string `Control ID` finds it on ~1 page in 15 and silently
returns nothing for the rest — that exact mistake was caught during research and
must not reach code.

**Parser contract:** treat a table as a control table when its header row
contains `Pin Name`. Normalize to `{control_id, pin_name, control_type, value,
string, position, pins_available}`, deriving `control_id` from `Pin Name` when
no explicit `Control ID` column exists. Expect 0..N tables per page and keep the
per-table grouping (pin group name from the nearest preceding heading).

## Top Workflows
1. **"What are the specs for model X?"** — integrator needs the spec sheet to
   quote or commission. Today: browse qsys.com, find the product, open the PDF.
2. **"How do I configure X?"** — device configuration procedures from
   help.qsys.com, resolved from the product rather than hunted by hand.
3. **"How do I wire/connect X?"** — networking, wiring, I/O guidance for the
   model's family instead of a flat 12-page section.
4. **"Will this BOM run on QDS 9.4?"** — hardware compatibility matrix checked
   against a whole equipment list at once.
5. **"Which parts in this list are EOL/deprecated?"** — deprecation notices +
   discontinued-products family cross-referenced locally.

## Table Stakes
- Full-text search across both sites' content with ranked results
- Fetch a page / product as clean readable text (not raw HTML)
- Browse products by family, components by section
- Offline operation after an initial sync
- `--json` output for every command, agent-consumable

## Data Layer
- Primary entities:
  - `product` (271) — model, family, url, overview_text, spec_pdf_url, spec_text, discontinued
  - `page` (753) — url, section, title, text (help.qsys.com)
  - `compat_row` (59) — qds_version, release_date, added_hardware, removed_hardware
- Sync cursor: both sitemaps + per-item content hash. Docs change per release,
  so a weekly `stale_after` is right.
- FTS/search: FTS5 over product spec text AND page text; product lookup is a
  different query shape from prose search, so both get an index.

## Competitive Landscape (re-checked 2026-08-10)
Every existing Q-SYS tool is a **QRC control client / control-plane MCP**.
Not one indexes the documentation or product specs.

| Tool | Language | What it does |
|---|---|---|
| `@q-sys/qrwc` | JS/npm | Official QSC WebSocket control library |
| `qsys-qrc-client` | JS/npm | Type-safe QRC control client |
| `qsys-tools/qrc-client-js` | JS | External control client |
| `qsys-qrc-py` | Python | QRC over TCP socket |
| `pyqsys` | Python | QRC JSON-RPC 2.0 wrapper |
| `gagehelton/qsys` | Python | Core/Control/ChangeGroup wrapper |
| `qrwc-svelte-mcp` | JS/npm | NEW (2026) control-plane MCP: connects to a live Core via QRWC, lists components/controls in a running design. Needs Core IP. 4★, experimental |
| `qsys-mcp` | npm | NEW (2026) control-plane MCP (npm; page Cloudflare-challenged, control-plane by naming/context) |

There is **no Q-SYS docs CLI, no docs MCP, and no offline Q-SYS reference of any
kind.** The competitive bar for a docs CLI is the Flare site's own search box.
`qrwc-svelte-mcp`'s own README links the same help docs this CLI indexes — every
control tool needs the pin/component reference, and this CLI is the missing
lookup layer beneath the entire ecosystem.

## Pain Points (evidence-backed)
1. QSC's own documented answer for discovering control names is "select the
   Component, Tools → View Component Controls Info" — requires the Designer
   software, a licence, and the component already placed in a design.
2. Pin data is scattered across 384 separate web pages with no cross-page index.
3. Version drift: help.qsys.com defaults to latest; a tech commissioning a 9.4
   system reads 10.0 behavior with no warning.
4. No offline access — field techs on locked-down or air-gapped job-site
   networks cannot reach the docs at all.

## Product Thesis
- **Name:** `qsys-pp-cli`
- **Why it should exist:** Every Q-SYS integration and quote starts with "what
  is this control called / what are the specs / will this run on my version",
  and the only sanctioned way to answer is to open a Windows-only design tool or
  read a 59-row matrix by eye. This CLI turns 753 help pages + 271 product pages
  into a local, queryable index — searchable, version-aware, available offline,
  and emitted as JSON an agent can act on.

## Build Priorities
1. Data layer: both sitemaps → pages/products → parsed components + spec PDFs,
   FTS5 on both.
2. `search`, `page get`, `product get`, `product list` — table stakes.
3. `compat check` / `compat deprecated` — BOM-time safety, the differentiators.
4. Version-aware fetch (`--version 9.4`) against the versioned trees.
5. `connect <model>` — family-resolved wiring/networking guidance.

## Notes for Generation
- No auth. Do not invent an auth section; `auth.type: none`.
- `response_format: html` with HTML extraction; transport is plain
  `standard_http` (no Surf, no browser clearance, no cookie import).
- Phase 5 live dogfood is MANDATORY here — the API is public and freely
  testable, so there is no credential excuse to skip it.
- Cache auto-refresh is OFF (corpus is built by hand-authored `harvest`, not
  generated sync; pre-read refresh fired a doomed sitemap fetch and dumped 406
  HTML into stdout — fixed in prior run, do not re-enable).
