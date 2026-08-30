# Q-SYS CLI Brief (reprint, 2026-08-24)

## API Identity
- Domain: professional AV — QSC Q-SYS audio/video/control platform documentation
- Users: AV integrators, designers, and commissioning techs (NOT control programmers — see Users below)
- Data profile: three vendor web surfaces, no API, no auth, no bot protection. All plain HTTP 200.
  - `help.qsys.com` — configuration, networking, compatibility. **Zero electrical specs** (verified: CX-Q page has no watts/ohms/THD).
  - `www.qsys.com` — product pages linking spec-sheet PDFs. The numbers live here, and only here.
  - `support.qsys.com` — FAQ / "awareness" articles (e.g. removed hardware support in QDS 10.0). **Not indexed by the prior print — gap.**

## Users
1. **The AV integrator pricing a job.** Has an equipment list. Needs to know: does this run on the Designer version the client is standardized on, is any of it end-of-life, and what are the actual electrical numbers for the submittal. Works from a laptop, often on a job site with hostile or absent network.
2. **The commissioning tech on site.** Has hardware in a rack and a deadline. Needs wiring/networking guidance for a specific model, and needs the docs as of the Designer version actually installed — not today's.
3. **The designer building a submittal package.** Needs spec text and source PDF URLs per model, in bulk, without clicking through 271 product pages.

Explicitly NOT a user: the Q-SYS control programmer. Control-pin lookup, QRC/Lua snippet emission, and live Core control were considered and **dropped by the user**. No Q-SYS Core is reachable from this machine, so those commands would ship unverified.

## Top Workflows
1. **BOM sweep before quoting** — paste an equipment list, get per-model Designer-version support, EOL status, and spec-sheet availability in one pass. This is the ritual that saves a redesign.
2. **Version-support check against a standardized Designer release** — "the client is on 9.4 / 10.0; does this list run?" QDS 10.0 removed hardware support for several devices (some carried to June 2028 on an LTS branch), so this is live and changing.
3. **Spec lookup for a submittal** — one model, full spec text plus the source PDF URL, ready to paste into a bid table.
4. **Wiring/networking guidance for a model being commissioned** — the networking pages that actually apply, not the whole section.
5. **Version-pinned doc read** — read a help page as of the Designer version installed on site, from the versioned tree (`/q-sys_9.4/`, `/q-sys_10.0/`).

## Table Stakes
Nothing in the ecosystem does this job. Every existing tool is live-Core control or design-file management:
- `tomsfaire/q-sys-mcp` — MCP server for live Core control (faders, mutes, snapshots, Lua over QRC). Different job entirely.
- `qsys-tools/qrc-client-js` — Node.js QRC external control client. Live control.
- `mckay115/QSC-QSYS-Launcher` — design-file creation/revisioning/versioning. Different job.
- The two QSC websites themselves — neither can answer a compatibility question across an equipment list.

Table stakes therefore reduce to: match what the websites give you (specs, config pages, compat matrix) and beat them by joining the three sources locally and answering list-shaped questions.

## Data Layer
- Primary entities: `products` (~266, of which ~168 resolve a spec-sheet PDF), `pages` (~766 help pages), `compat` (~51 rows)
- Corpus builder is a hand-authored `harvest` command walking both sitemaps + PDF text extraction. **The generated `sync` is a no-op here** — every spec endpoint is `response_format: html` or `binary`, so `defaultSyncResources()` has nothing JSON to fetch (upstream cli-printing-press#4342).
- FTS/search: full-text across page text and extracted spec text.
- Spec text requires `--with-pdfs` (pdftotext). Prior print left it unextracted by default, so specs were not searchable — the single biggest usability gap.

## Reachability Risk
- None. `help.qsys.com/Content/Hardware/Hardware_Overview.htm` -> HTTP 200; `www.qsys.com/products-solutions/` -> HTTP 200. No auth, no bot protection, no challenge.

## Product Thesis
- Name: qsys-pp-cli
- Why it should exist: QSC splits the answer to every integrator question across three sites, and none of them can take an equipment list as input. This CLI joins all three into local SQLite and answers spec, configuration, wiring, and compatibility questions offline — from a job site with no usable network — including list-shaped questions neither website can answer at all.

## Build Priorities
1. Corpus: `harvest` across all three sources, with spec-PDF text extraction ON by default (the prior print's biggest miss).
2. BOM-shaped commands: `bom verify`, `compat check`, `compat deprecated` — the list-in, report-out ritual.
3. Unified `product get` (spec + config + wiring in one record) and `connect` (wiring guidance by model).
4. Version-pinned `page get`, `integrations`, `coverage`.
5. Fix the two known defects: SQLite `mmap_size(0)` + `journal_mode(TRUNCATE)` (SIGBUS on concurrent reads), and `product resources <family> <slug>` signature documented correctly.
