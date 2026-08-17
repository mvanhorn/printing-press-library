# Crestron.com CLI — Novel Features Brainstorm (audit trail)

## Customer model

**Persona 1 — Marco, AV Service Manager at a mid-size integrator (~40 installed models across 60 client sites).**
Today: keeps a spreadsheet of installed models and eight crestron.com tabs open; per model he searches, filters to Software & Firmware, sorts by date, reads the top row, and compares by eye. No view on the site answers "what changed across a list of models."
Weekly ritual: Monday walk of the whole model list, updating a "latest version seen" column — 40 models, five-plus clicks each, on a slow site.
Frustration: it is O(models) manual navigation producing no durable record, and it is unreliable — releases are family-scoped, so one entry covering TSW-570/TSW-770/TSW-1070/TSS-770/TSS-1070/TS-770/TS-1070 is the current firmware for seven models, and searching his exact model can miss the release that governs it.

**Persona 2 — Priya, Crestron programmer / commissioning technician.**
Today: signs in, opens one version page, reads the change log, opens the previous version in another tab, and eyeballs the difference. Version and date are public; notes and binary need the cookie session.
Weekly ritual: for each job, pull the current firmware zip plus every change log between what is installed on site and what is current.
Frustration: change logs are per-version pages with no diff and no search, so "when did they fix HDCP passthrough on DM-NVX" means opening a dozen login-gated pages one at a time.

**Persona 3 — Dan, spec writer / design engineer producing CSI submittals.**
Today: per model he opens the Resources tab and hand-downloads the spec sheet, CSI Guide Spec, manual, certificates, Security Reference Guide, and CAD/Revit families — roughly twenty assets per product, each a separate click. Crestron's own Spec Sheet Collection tool covers one of those categories.
Weekly ritual: assemble one submittal package per active project, 10 to 25 models.
Frustration: hundreds of individual downloads per submittal, with missing asset classes discovered only after the package is assembled.

**Persona 4 — Ellen, estimator / systems designer pricing refresh projects off as-built lists.**
Today: reads discontinued status off the `Inactive/Discontinued` catalog path and reads the replacement off whatever the product page happens to render.
Weekly ritual: triage a 20-to-50-line as-built list into current, discontinued-with-replacement, and discontinued-no-successor.
Frustration: no status field to query and no bulk path — a 50-line list is 50 manual page visits.

## Candidates (pre-cut)

1. **Fleet firmware status** `fleet status --file fleet.txt` — per-model installed vs latest applicable version, date, days behind, resolved through series. Marco. Source (a)+(c)+(b). Keep.
2. **Project BOM audit** `fleet audit --file bom.csv` — same input, lifecycle answers. Ellen. Source (a). Sibling risk with 1 and 7.
3. **Release-note full-text search** `search "<text>" --type firmware_release` — FTS over synced notes and change logs. Priya, Marco. Source (c)+(e). Keep.
4. **Release-note diff** `firmware diff <model> <from> <to>` — line diff of two change logs. Priya. Source (b). Keep.
5. **New-firmware feed** `firmware changed --since 30d` — date filter, no fleet scope. Subset of 1.
6. **Submittal package builder** `submittal <model...> --out <dir>` — full asset set per model plus coverage table. Dan. Source (b)+(a). Keep.
7. **Lifecycle and replacement trace** `lifecycle <model>` — status plus successor chain. Ellen. Source (b)+(c). Keep.
8. **Spec table comparison** `specs compare <A> <B>` — align 68 spec rows across 12 sections. Dan, Ellen. Source (c). Keep.
9. **Series roster** `series <name>` — expands a series to member SKUs. Plumbing consumed by 1/3/4/8.
10. **Release coverage map** `firmware covers <version>` — inverse of the same join.
11. **Library coverage report** `coverage --category <path>` — audits Crestron's doc hygiene, nobody's job.
12. **Pricing lookup** `price <model>` — fails verifiability, handler returns `[]` for sampled SKUs.
13. **Catalog analytics** `analytics --type product --group-by category` — framework emits it free, no ritual consumes it.
14. **Offline catalog mirror** `mirror --out <dir>` — scope creep; its useful form is 6.

Kill/keep checks applied inline: no candidate requires an LLM (the diff is mechanical, the search is FTS); no candidate calls a service outside the brief; the two auth-dependent ones degrade rather than fail logged-out; none synthesizes API responses locally. Local-store commands carry `// pp:data-source local|auto`, `hintIfUnsynced`/`hintIfStale`, and drain parent `*sql.Rows` before per-model follow-up queries.

## Survivors and kills

### Survivors

| # | Feature | Command | Buildability | Score | Persona served | Why only we can do this | Long Description |
|---|---------|---------|--------------|-------|----------------|------------------------|------------------|
| 1 | Fleet firmware status | `fleet status --file fleet.txt` | hand-code | 9/10 | Marco (service manager) | Firmware releases are family-scoped and products group into series, so "which release covers model X" is many-to-many and no single page answers it; we build a `series_model` join from VariantProduct.ashx plus parsed multi-model release titles and match a whole fleet list against it in one local query. | Use this command to check many models at once against a saved fleet list. Do NOT use it to search release-note text; use 'search --type firmware_release'. Do NOT use it to determine whether a model is discontinued; use 'lifecycle'. |
| 2 | Release-note full-text search | `search "<text>" --type firmware_release` | spec-emits | 9/10 | Priya (programmer), Marco | Release notes and change logs sit behind forms auth on per-version pages and have never been searchable across versions anywhere; once synced with a cookie session they become a local FTS index, with hits expanded through series so a family release names every affected model. | Use this command to find which firmware version mentions a term across all models. Do NOT use it to compare two specific versions; use 'firmware diff'. Do NOT use it for fleet-wide currency checks; use 'fleet status'. |
| 3 | Release-note diff between versions | `firmware diff <model> <from> <to>` | hand-code | 8/10 | Priya (commissioning tech) | Change logs exist only as per-version HTML pages behind login with no diff view; we resolve the model to its family release, fetch both versions with the stored session, and emit a line diff. | Use this command to see what changed between two firmware versions. Do NOT use it to read a single version's notes; use 'firmware notes'. |
| 4 | Submittal package builder | `submittal <model...> --out <dir>` | hand-code | 9/10 | Dan (spec writer) | ResourceHandler.ashx?dID= returns ~20 assets per product anonymously across CAD, Revit, CSI Guide Specs, Security Reference Guides, manuals and spec sheets; we fan it out over a model list into per-model folders and print an asset-class coverage table, where Crestron's own tool covers spec sheets only. | Use this command to assemble a multi-model, multi-asset-class submittal folder. Do NOT use it to fetch one known file; use 'download'. Do NOT use it to merely list what exists; use 'resources'. |
| 5 | Lifecycle and replacement trace | `lifecycle <model>` | hand-code | 8/10 | Ellen (estimator) | ReplacementProductsHandler.ashx is an unadvertised internal endpoint returning replacements for discontinued items; combined with the `Inactive/Discontinued` catalog path we can walk a successor chain transitively and in bulk, which no visible surface does. | Use this command to determine sellable status and find a successor part. Do NOT use it to list accessories for a current product; use 'product get --related'. |
| 6 | Spec table comparison | `specs compare <A> <B>` | hand-code | 7/10 | Dan, Ellen | The 12-section, 68-row spec table is parsed and stored locally, so two products can be aligned by section header and key — the site renders one product at a time, and same-series models sharing one firmware release differ only in these rows. | Use this command to compare two models field by field. Do NOT use it to display one model's specs; use 'specs'. |

Pass 3 answers, condensed: all six clear weekly use for their named persona (6 is the softest — design-phase model selection rather than a fixed ritual); none is a thin rename of one page fetch; each draws power from local SQLite, a cross-source join, or a Crestron-specific content pattern; every sibling named in a Long Description either survived or is an already-absorbed shipping command. Hand-code commitment: five of six.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Project BOM audit (`fleet audit --file bom.csv`) | Same file input and query path as `fleet status` for a question `lifecycle` already answers per model. | Fleet firmware status |
| New-firmware feed (`firmware changed --since 30d`) | A date filter with no fleet scope; unfiltered "everything new at Crestron" serves no persona's ritual. | Fleet firmware status |
| Series roster (`series <name>`) | Series membership is the join key four survivors consume internally; shipping it alone exposes plumbing as a feature. | Fleet firmware status |
| Release coverage map (`firmware covers <version>`) | The inverse direction of a join `fleet status` and `search --type firmware_release` already resolve and print. | Fleet firmware status |
| Library coverage report (`coverage --category <path>`) | Audits Crestron's own documentation hygiene, which is nobody's weekly job. | Spec table comparison |
| Pricing lookup (`price <model>`) | Fails verifiability — PublicPricingHandler.ashx returned `[]` for every sampled SKU. | none |
| Catalog analytics (`analytics --type product --group-by category`) | The framework emits it for free and no persona ritual consumes it. | none |
| Offline catalog mirror (`mirror --out <dir>`) | Scope creep — thousands of binary fetches, resumability, and a directory layout to design; its useful one-command form is the submittal builder. | Submittal package builder |
