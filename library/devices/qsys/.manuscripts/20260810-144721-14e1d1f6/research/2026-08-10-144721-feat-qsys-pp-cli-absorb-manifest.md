# Q-SYS CLI — Absorb Manifest (equipment persona)

> Reused from prior run `20260808-153253-09b3492e` (approved at gate), refreshed
> by the mandatory novel-features subagent (reprint reconciliation) on
> 2026-08-10. Two prior features re-scored to drop; three new added.

## Persona
AV integrator / designer: needs **product specs**, **how to configure
equipment**, **how to connect equipment**, and **BOM-time compatibility /
deprecation checks**. Control-scripting (pin-level QRC/Lua) is out of scope per
user correction in the prior run.

## Sources (combo — priority confirmed 2026-08-08)

| Source | Serves | Surface | Verified |
|---|---|---|---|
| `qsys.com` (PRIMARY) | product specs | 271 product pages -> linked spec-sheet / manual PDFs | sitemap 1,865 URLs; PDF HTTP 200 `application/pdf`; `pdftotext` extracts real spec values; this run re-probed 200 |
| `help.qsys.com` | configure, connect, compatibility | 753 `.htm` pages | sitemap complete; plain GET 200; no JS, no auth, no bot protection; `probe-reachability` = `standard_http` this run |

Product families on qsys.com: loudspeakers (104), q-sys (95), power-amplifiers
(27), mixers (7), software-and-firmware (7), discontinued-products (6).

## Competitive Landscape (re-checked 2026-08-10)
Every existing Q-SYS tool is a QRC control client or control-plane MCP
(`@q-sys/qrwc`, `qsys-qrc-client`, `qrc-client-js`, `qsys-qrc-py`, `pyqsys`,
`gagehelton/qsys`, plus NEW `qrwc-svelte-mcp` and `qsys-mcp` — both control-plane,
require a live Core IP). **No Q-SYS docs CLI, no docs MCP, no offline reference
exists.** The docs-index gap is intact; `qrwc-svelte-mcp`'s own README links the
same help docs this CLI indexes.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Full-text documentation search | help.qsys.com Flare search | `qsys-pp-cli search` | FTS5 offline, ranked, spans BOTH sites in one query |
| 2 | Read a documentation page | Flare page render | `qsys-pp-cli page get` | Clean text, no browser chrome, `--json` |
| 3 | Product overview | qsys.com product page | `qsys-pp-cli product get` | Joined with help-site overview + spec PDF in one payload |
| 4 | Browse products by family | qsys.com product nav | `qsys-pp-cli product list` | `--family loudspeakers`, filterable, `--json` |
| 5 | Product spec sheet | qsys.com PDF download | `qsys-pp-cli product specs` | Extracted searchable text + source PDF URL |
| 6 | Configuration procedures | help.qsys.com per-device pages | `(behavior in qsys-pp-cli product get) --configure` | Config pages resolved from the product, not hunted by hand |
| 7 | Connection / networking guidance | help.qsys.com Networking (12 pages) | `qsys-pp-cli connect` | Wiring, Dante, QoS, multicast, switch setup, addressing |
| 8 | Hardware compatibility matrix | `Hardware_Compatibility_QDS_Version.htm` | `qsys-pp-cli compat list` | 59-row matrix parsed into queryable rows |
| 9 | Upgrade path requirements | `upgrade_path_requirements.htm` | `(behavior in qsys-pp-cli compat) --upgrade-path` | Structured instead of prose-hunting |
| 10 | Deprecation / downgrade notices | Compatibility section | `(behavior in qsys-pp-cli compat) --deprecated` | Surfaces end-of-life status per model |
| 11 | Offline availability | *(nothing offers this)* | `qsys-pp-cli sync` | Whole corpus local; works on locked-down job-site networks |
| 12 | Agent access | *(nothing offers this)* | `(behavior in qsys-pp-cli mcp)` | MCP server; no Q-SYS docs MCP exists today |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|---|
| 1 | Unified product card | `product get <model>` | hand-code (built) | 10 | Joins the qsys.com product page, its spec-sheet PDF text, the help.qsys.com config pages, and connection guidance into one record. That join spans two separate websites and one PDF; no QSC page shows it together. | Use this for one product's full record. Do NOT use it to browse a whole family; use 'product list'. |
| 2 | BOM compatibility check | `compat check <models...> --qds 9.4` | hand-code (built) | 10 | Answers "will this equipment list run on this Designer version" against the parsed 59-row QDS matrix. The website makes you read a 59-row table by eye, one model at a time. Accepts a whole BOM via stdin. | Use this to check whether a list of models is supported on a specific QDS version. Do NOT use it for EOL status; use 'compat deprecated'. For a per-model report spanning support + EOL + specs, use 'bom verify'. |
| 3 | Deprecation / discontinued sweep | `compat deprecated <models...>` | hand-code (built) | 10 | Cross-references deprecation notices and the discontinued-products family against a model list, so an end-of-life part is caught at design time rather than at order time. | Use this to flag deprecated/discontinued models in a list. Do NOT use it for version support; use 'compat check'. For a full per-model sweep, use 'bom verify'. |
| 4 | Connection guidance by model | `connect <model>` | hand-code (built) | 10 | Resolves a specific model to the networking, wiring, and I/O pages that actually apply to its family, instead of a flat 12-page section the user filters by hand. | Use this for model-resolved wiring/networking guidance. Do NOT use it for full product records; use 'product get'. |
| 5 | BOM sweep — one report per model | `bom verify --qds 9.4` (stdin) | hand-code (NEW) | 9 | One per-model report joining products (family, discontinued, spec availability) with compat support for `--qds`; neither site can answer a whole BOM in one pass. | Use this for a complete per-model BOM report (support, EOL, spec availability). Do NOT use it for matrix-by-version lookups; use 'compat check'. Do NOT use it to flag only EOL parts; use 'compat deprecated'. |
| 6 | Version-aware page reads + drift warning | `page get <topic> --version 9.4` | hand-code (NEW) | 9 | Stores a version column on pages during harvest of the versioned trees (`/q-sys_9.4/`, `/q-sys_9.6/`, `/q-sys_10.0/` — all verified HTTP 200), filters `page get` by it, and warns when the local doc's major differs from the target. | Use the --version flag to read a help page as of a specific QDS version. Do NOT use --version to check hardware support; use 'compat check'. For current-release docs call 'page get' without --version. |
| 7 | Coverage / integrity report | `coverage` | hand-code (built) | 7 | Reports how many of the 271 products actually resolved a spec-sheet PDF and how many pages parsed, making extraction drift a visible number instead of a silent empty result. | none |
| 8 | UC platform integration lookup | `integrations <model>` | hand-code (NEW) | 7 | Matches a product model to the 34 Application_Integration pages by local FTS + join; no page on either site indexes certifications by device. | Use this to find which UC platforms (Teams/Zoom/Meet) a device is certified/integrated with. Do NOT use it for wiring or networking; use 'connect'. |

**Hand-code commitment:** 8 of 8 novel rows need hand-written Go after generate
(~50-150 LoC each plus wiring). **5 are already built in the seeded tree**
(rows 1-4, 7 — prior run, verified working). **3 are new this run** (rows 5, 6,
8: `bom verify`, `page get --version`, `integrations`).

## Dropped prior features (reprint reconciliation — user may override at gate)
| Prior feature | Verdict | Justification |
|---|---|---|
| `product compare` | drop | Weekly use is "depends"; spec answers come from two `product get` calls. Command stays in the tree, just not a headline feature. |
| `sql` | drop | Escape hatch with no weekly persona ritual; `--agent`/`--select` on dedicated commands serve agent queries. Command stays in the tree. |

## Stubs
None. Every row is shipping scope.

## Data Layer
- `product` (271) — model, family, url, overview_text, spec_pdf_url, spec_text, discontinued
- `page` (753) — url, section, title, text (help.qsys.com); **+ version column (NEW, row 6)**
- `compat_row` (59) — qds_version, release_date, added_hardware, removed_hardware
- FTS5 over product spec text AND page text; product lookup is a different
  query shape from prose search, so both get an index.
- Sync cursor: both sitemaps + per-item content hash. Docs change per release,
  so a weekly `stale_after` is right.

## Risks the user should see before approving
- **Spec-sheet matching is the central risk.** PDF links are not in the qsys.com
  sitemap; they must be scraped off each of 271 product pages, and link
  placement varies by product line. Mitigation is `coverage` reporting the match
  rate as a visible number.
- **Versioned-tree harvest (row 6) adds sync cost.** Each versioned tree is a
  full 753-page corpus; harvesting all three triples harvest time. Default
  harvest stays current-release; `--version` trees are opt-in.
- **PDF text extraction is layout-dependent.** Specs are stored as searchable
  text plus the source PDF URL — no per-field parsing — so layout variance
  degrades search quality rather than silently producing wrong structured
  values. Exact figures are always one click away in the source PDF.
- **Sync cost:** 753 help pages + 271 product pages + up to 271 PDFs. Both are
  static sites with no published rate limit; the CLI must rate-limit itself.
