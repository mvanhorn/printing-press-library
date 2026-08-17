# Crestron.com CLI — Absorb Manifest

## Landscape

Every existing Crestron tool is **device-side**, not **catalog-side**:

| Tool | Kind | Scope | Platform |
|---|---|---|---|
| Crestron Toolbox | official GUI | push firmware to devices, command console, device discovery, TSID/serial conversion, Device Database | Windows |
| Crestron EDK / PSCrestron | official PowerShell | device automation over IP, firmware upgrade to devices | Windows + dealer account |
| CrestronMasterTool (alastairWH) | community GUI | SFTP download of software/firmware, batch download, version select, silent install | Windows |
| Crestron-EDK-Superscripts (JaytheSpazz) | community PowerShell | pull device info, issue commands across an IP list | Windows + EDK |
| CrestronNVX_Firmware.ps1 (intellectualrockstar) | community PowerShell | autodiscover NVX devices, compare firmware, push if mismatched | Windows + EDK |
| Crestron-FTP-Scripts (oniointeractive) | community PowerShell | FTP transfer helpers | Windows + EDK |
| Spec Sheet Collection | official web app | assemble a set of spec sheet PDFs | Browser |
| Desluca/crestron-mcp | community MCP | Crestron **Home** device control (lights, shades, scenes) | **out of scope** |

**Whitespace:** nobody has a queryable, scriptable, offline mirror of the
**catalog and release library**. CrestronMasterTool is the only catalog-adjacent
tool and it is a Windows GUI that downloads files — it cannot answer questions.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Searchable product list | CrestronMasterTool | `crestron-pp-cli product list` | Offline FTS, `--json`, category + family filters, pipes to jq |
| 2 | Filter products by category | CrestronMasterTool | (behavior in `crestron-pp-cli product list`) `--category` from the 127-path taxonomy | Full taxonomy synced locally with live product counts |
| 3 | Switch software vs firmware category | CrestronMasterTool | (behavior in `crestron-pp-cli resource search`) `--category` maps to the `c` param | All 8 categories, not just 2 |
| 4 | Select a specific firmware version | CrestronMasterTool | `crestron-pp-cli firmware list` | Every version with date, sortable, `--json`, no account needed to *see* versions |
| 5 | Batch download firmware/software | CrestronMasterTool | `crestron-pp-cli download` | Range-request resume, checksum, `--dry-run`, scriptable |
| 6 | Silent install | CrestronMasterTool | *(not absorbed — out of scope)* | We download; installation stays with Toolbox |
| 7 | Product/device database lookup | Crestron Toolbox Device Database | `crestron-pp-cli product get` | Works without Windows or Toolbox; JSON-LD-backed |
| 8 | Show product specifications | crestron.com Specifications tab | `crestron-pp-cli product specs` | Parsed key/value pairs, `--select`, offline after sync |
| 9 | Spec sheet collection | Crestron Spec Sheet Collection (web) | (behavior in `crestron-pp-cli resource search`) `--type "Spec Sheets"` + `download` | Scriptable bulk pull instead of click-to-collect |
| 10 | Browse catalog by category | crestron.com navigation | `crestron-pp-cli catalog` | Whole tree in one command, with counts |
| 11 | Download spec sheets / manuals / certs / drawings | crestron.com Resource Library | `crestron-pp-cli download` | All `/getmedia/` asset classes, no login required |
| 12 | Per-product resource list | crestron.com Resources tab | `crestron-pp-cli product resources` | One command instead of navigating a tab |
| 13 | Related / accessory models | crestron.com Models & Accessories tab | (behavior in `crestron-pp-cli product get`) `--related` | Graph is synced, so it is traversable offline |
| 14 | Firmware release notes | crestron.com firmware detail page (login) | `crestron-pp-cli firmware notes` | Cookie session imported from Chrome; notes become **searchable** |
| 15 | Resource search sort + paginate | crestron.com Resource Library | (behavior in `crestron-pp-cli resource search`) `--sort relevance\|date\|name`, `--limit` | Relevance/date/A-Z/Z-A all exposed; auto-pagination |
| 16 | Cross-category resource search | crestron.com Resource Library (`c=0`) | (behavior in `crestron-pp-cli resource search`) default `--category all` | Single query spans firmware, docs, certs, drawings |
| 17 | Browser login for gated content | crestron.com sign-in | `crestron-pp-cli auth login` | Reads HttpOnly cookies from Chrome via CDP; no password handling |

Row 6 is the one deliberate non-absorb: installing firmware onto hardware is
Toolbox/EDK territory and requires being on the device network. Everything else
that any competing tool does, this CLI does — plus `--json`, `--agent`,
`--select`, `--dry-run`, typed exit codes, and a local SQLite mirror.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Persona | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|---------|------------------------|------------------|
| 1 | Fleet firmware status | `fleet status --file fleet.txt` | hand-code | 9/10 | Marco (service manager) | Firmware releases are family-scoped and products group into series, so "which release covers model X" is many-to-many and no single page answers it; we build a `series_model` join from VariantProduct.ashx plus parsed multi-model release titles and match a whole fleet list in one local query. | Use this command to check many models at once against a saved fleet list. Do NOT use it to search release-note text; use 'search --type firmware_release'. Do NOT use it to determine whether a model is discontinued; use 'lifecycle'. |
| 2 | Release-note full-text search | `search "<text>" --type firmware_release` | spec-emits | 9/10 | Priya, Marco | Release notes sit behind forms auth on per-version pages and have never been searchable across versions anywhere; once synced they become a local FTS index, with hits expanded through series so a family release names every affected model. | Use this command to find which firmware version mentions a term across all models. Do NOT use it to compare two specific versions; use 'firmware diff'. Do NOT use it for fleet-wide currency checks; use 'fleet status'. |
| 3 | Release-note diff between versions | `firmware diff <model> <from> <to>` | hand-code | 8/10 | Priya (commissioning tech) | Change logs exist only as per-version HTML pages behind login with no diff view; we resolve the model to its family release, fetch both versions with the stored session, and emit a line diff. | Use this command to see what changed between two firmware versions. Do NOT use it to read a single version's notes; use 'firmware notes'. |
| 4 | Submittal package builder | `submittal <model...> --out <dir>` | hand-code | 9/10 | Dan (spec writer) | ResourceHandler.ashx?dID= returns ~20 assets per product anonymously across CAD, Revit, CSI Guide Specs, Security Reference Guides, manuals and spec sheets; we fan it out over a model list into per-model folders with an asset-class coverage table, where Crestron's own tool covers spec sheets only. | Use this command to assemble a multi-model, multi-asset-class submittal folder. Do NOT use it to fetch one known file; use 'download'. Do NOT use it to merely list what exists; use 'resources'. |
| 5 | Lifecycle and replacement trace | `lifecycle <model>` | hand-code | 8/10 | Ellen (estimator) | ReplacementProductsHandler.ashx is an unadvertised internal endpoint returning replacements for discontinued items; combined with the Inactive/Discontinued catalog path and the dated End-of-Sale notice stream we can walk a successor chain transitively and in bulk, which no visible surface does. | Use this command to determine sellable status and find a successor part. Do NOT use it to list accessories for a current product; use 'product get --related'. |
| 6 | Spec table comparison | `specs compare <A> <B>` | hand-code | 7/10 | Dan, Ellen | The 12-section, 68-row spec table is parsed and stored locally, so two products can be aligned by section header and key — the site renders one product at a time, and same-series models sharing one firmware release differ only in these rows. | Use this command to compare two models field by field. Do NOT use it to display one model's specs; use 'specs'. |

**Hand-code commitment: 5 of 6** (rows 1, 3, 4, 5, 6). Row 2 is `spec-emits` —
the framework's FTS `search` command covers it once `firmware_release` is a
synced resource with notes populated.

Killed candidates and the full customer model are preserved in
`2026-08-02-132209-novel-features-brainstorm.md`.
