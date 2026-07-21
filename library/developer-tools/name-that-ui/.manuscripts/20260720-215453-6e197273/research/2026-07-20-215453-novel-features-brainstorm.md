## Customer model

### Mira — coding agent repairing a UI from an imprecise ticket

**Today (without this CLI):** Mira receives requests such as “fix the pale pill behind the menu-bar icon,” searches the web for likely terminology, opens several component references, and guesses which framework symbol the request means. She cannot reliably distinguish the whole component from one of its named parts.

**Weekly ritual:** Several times each week, she converts visual or colloquial requests into implementation plans, code edits, and debugging prompts.

**Frustration:** The source ticket, canonical component name, relevant anatomy, and implementation API live in different vocabularies, so a plausible guess can send the repair down the wrong path.

### Evan — frontend engineer choosing among similar interaction patterns

**Today (without this CLI):** Evan compares tabs for comboboxes, selects, dropdowns, popovers, and tooltips, then manually extracts anatomy, accessibility cautions, and implementation guidance into a ticket or coding-agent prompt.

**Weekly ritual:** Before implementing or reviewing a UI feature, he validates that the selected pattern matches the interaction intent and accessibility expectations.

**Frustration:** Even after identifying a component, assembling a compact, source-backed implementation packet requires repetitive page hopping and copy/paste.

### Priya — macOS engineer translating product language into platform APIs

**Today (without this CLI):** Priya searches product terminology, Apple framework names, and UI references independently, then reconciles AppKit and SwiftUI symbols by hand.

**Weekly ritual:** She translates design requests and bug reports into exact macOS component, part, AppKit, and SwiftUI terminology.

**Frustration:** A single concept can have different names in plain language, component anatomy, AppKit, and SwiftUI, with no unified crosswalk.

### Theo — design-system author maintaining implementation guidance

**Today (without this CLI):** Theo periodically revisits component and style references, compares them with internal specifications and source code, and manually determines whether newly published guidance affects existing documentation or implementations.

**Weekly ritual:** He reviews design-system prose and implementation inventories for ambiguous terminology, stale guidance, and inconsistent API naming.

**Frustration:** He can see that the upstream catalog changed, but cannot mechanically identify which internal prose or source files are affected.

## Candidates (pre-cut)

1. **Agent context pack** — `context-pack --component <slug> [--style <slug>] [--framework <name>]`; source (e), serves Mira and Evan. Assemble a bounded, source-backed implementation packet across component definition, anatomy, APIs, prompts, style signals, and cautions. Keep: it uses synced records and cross-entity joins, requires no LLM or external service, and produces an agent-shaped result that no single endpoint provides. Long Description: `none`.
2. **Terminology lint** — `lint <path>`; source (a), serves Mira and Theo. Scan prose, prompts, or specifications for colloquial or ambiguous UI terms and emit canonical candidates with source-backed replacements. Keep, mechanically reframed: it performs exact, alias, fuzzy-phrase, and local FTS matching rather than LLM rewriting; ambiguous matches remain explicitly unresolved. Long Description: `Use this command for prose, prompt, and specification terminology checks. Do NOT use this command to identify UI APIs already present in source code; use 'inventory' instead.`
3. **Framework crosswalk** — `crosswalk <concept>`; source (c), serves Priya. Join a concept’s plain-language names, aliases, parts, AppKit symbols, SwiftUI symbols, and ARIA/HTML mappings into one table. Keep: it joins multiple synced entity types and is more than a renamed translation or component-API endpoint. Long Description: `none`.
4. **Project UI inventory** — `inventory <path>`; source (a), serves Mira and Theo. Scan a source tree for known UI API symbols and canonical names, then produce a source-linked component inventory. Keep: a bounded filesystem scan plus local symbol index is verifiable and needs neither a browser nor a language model. Long Description: `Use this command to map UI components and API symbols currently present in source code. Do NOT use this command to check prose terminology; use 'lint' instead.`
5. **Catalog change impact** — `impact <path> --since <snapshot>`; source (c), serves Theo and Priya. Match changed catalog records against a project’s detected UI symbols and report source files whose guidance may be stale. Keep: it joins catalog snapshots, changed records, and a deterministic source scan; scope stays within one command and local data. Long Description: `Use this command to find project files affected by guidance changes between catalog snapshots. Do NOT use this command for a current-state source inventory; use 'inventory' instead.`
6. **Implementation handoff document** — `handoff --component <slug> [--style <slug>]`; source (a), serves Evan. Cut as a sibling duplicate: its useful content is the same cross-entity packet as `context-pack`, differing mainly in presentation.
7. **Review checklist** — `checklist --component <slug> [--style <slug>]`; source (b), serves Evan. Cut: checklist output belongs inside `context-pack`; a separate command would fragment the same evidence.
8. **Prompt canonicalizer** — `rewrite-prompt <path>`; source (e), serves Mira. Cut under the LLM-dependency check: a safe mechanical rewrite would merely duplicate `lint` findings and replacements.
9. **Style token generator** — `style tokens <slug>`; source (b), serves Evan. Cut under verifiability and reimplementation checks: style signals and code starting points do not define authoritative production token values.
10. **Screenshot classifier** — `classify-image <path>`; source (a), serves Mira. Cut: it requires an image model or browser-adjacent service outside the public NameThatUI contract.
11. **Continuous guidance watcher** — `watch-impact <path>`; source (a), serves Theo. Cut under scope creep: it requires a persistent process and duplicates the existing one-shot updates plus surviving `impact` workflow.
12. **Universal design-system mapper** — `map-system <concept> --system <name>`; source (b), serves Evan. Cut under external-service and verifiability checks: the brief does not establish authoritative third-party design-system mappings in NameThatUI’s data.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Agent context pack | `context-pack --component <slug> [--style <slug>] [--framework <name>]` | 10/10 | hand-code | This uses synced component, part, API, prompt, style-signal, and caution records in local SQLite to compute one bounded provenance-preserving packet with no external dependencies; implementation declares `// pp:data-source local`, calls sync/staleness hints, and drains rows before follow-up queries. | **Persona:** Mira and Evan. **Weekly:** yes, before UI implementation or repair. **Wrapper:** no; no endpoint returns this selected cross-entity packet. **Transcendence:** local joins plus agent-shaped output. **Sibling kill:** `handoff` was presentation-only duplication, and `checklist` is better expressed as a packet view. NameThatUI provides anatomy, APIs, prompts, styles, and cautions; the brief explicitly requires compact coding-agent context. | `none` |
| 2 | Terminology lint | `lint <path>` | 9/10 | hand-code | This uses local FTS over synced names, aliases, fuzzy phrases, parts, and API symbols to compute source-backed terminology findings from files with no external dependencies; implementation declares `// pp:data-source local` and calls all-resource sync/staleness hints. | **Persona:** Mira and Theo. **Weekly:** yes, during ticket, prompt, and design-spec review. **Wrapper:** no; the service searches one query but does not audit documents. **Transcendence:** local corpus scan plus catalog matching. **Sibling kill:** `rewrite-prompt` required generative rewriting, while lint preserves deterministic findings and explicit ambiguity. The brief identifies colloquial naming and hallucinated terminology as core pains; NameThatUI and Component Gallery both index aliases. | `Use this command for prose, prompt, and specification terminology checks. Do NOT use this command to identify UI APIs already present in source code; use 'inventory' instead.` |
| 3 | Framework crosswalk | `crosswalk <concept>` | 9/10 | hand-code | This uses synced element, alias, part, API-symbol, and translation tables in local SQLite to compute a unified terminology matrix with no external dependencies; implementation declares `// pp:data-source local`, calls sync/staleness hints, and drains primary matches before joining mappings. | **Persona:** Priya. **Weekly:** yes, whenever product language becomes AppKit or SwiftUI implementation work. **Wrapper:** no; it joins translation and component records across frameworks. **Transcendence:** cross-entity local query with platform-shaped output. **Sibling kill:** `map-system` was killed because arbitrary third-party mappings lack authoritative source data. The brief explicitly identifies plain language, AppKit, SwiftUI, and ARIA/HTML naming divergence; NameThatUI exposes both API mappings and macOS translations. | `none` |
| 4 | Project UI inventory | `inventory <path>` | 8/10 | hand-code | This uses a deterministic filesystem scan and the synced local API-symbol/name index to compute file-to-component matches with no external dependencies; implementation declares `// pp:data-source local` and calls all-resource sync/staleness hints before matching. | **Persona:** Mira and Theo. **Weekly:** yes, before UI repairs, reviews, and design-system maintenance. **Wrapper:** no; NameThatUI does not inspect a local project. **Transcendence:** local source evidence joined to the catalog. **Sibling kill:** `screenshot classifier` was killed because it needs an unavailable model, while inventory uses verifiable source symbols. The brief centers coding and repair workflows and provides exact framework symbols suitable for deterministic matching. | `Use this command to map UI components and API symbols currently present in source code. Do NOT use this command to check prose terminology; use 'lint' instead.` |
| 5 | Catalog change impact | `impact <path> --since <snapshot>` | 8/10 | hand-code | This uses local catalog snapshots, changed component/style records, and a deterministic project symbol scan to compute affected source files with no external dependencies; implementation declares `// pp:data-source local`, calls sync/staleness hints, and drains changed-record rows before file-match queries. | **Persona:** Theo and Priya. **Weekly:** yes, after the regular catalog sync or design-system review. **Wrapper:** no; updates alone do not connect changes to project code. **Transcendence:** snapshot diff joined to local source evidence. **Sibling kill:** `watch-impact` was killed because a persistent watcher adds infrastructure without improving the one-shot result. NameThatUI exposes RSS/sitemap freshness, the brief plans catalog snapshots, and Theo’s researched frustration is determining which internal artifacts a change affects. | `Use this command to find project files affected by guidance changes between catalog snapshots. Do NOT use this command for a current-state source inventory; use 'inventory' instead.` |

### Killed candidates

| feature | kill reason | closest-surviving-sibling |
|---|---|---|
| Implementation handoff document | It repackages the same component/style evidence as the context packet and differs mainly in human-facing formatting. | Agent context pack |
| Review checklist | Its checklist is an output view of the same selected anatomy and caution data, not an independent workflow. | Agent context pack |
| Prompt canonicalizer | Reliable free-form rewriting requires an LLM; a mechanical version collapses into terminology findings. | Terminology lint |
| Style token generator | NameThatUI style guidance does not provide authoritative production token values, making generated tokens synthetic and difficult to verify. | Agent context pack |
| Screenshot classifier | It requires image understanding outside the public API, local catalog, and no-browser constraint. | Project UI inventory |
| Continuous guidance watcher | A persistent background watcher exceeds command scope and duplicates a one-shot project impact query. | Catalog change impact |
| Universal design-system mapper | Arbitrary third-party mappings are not supported by the brief’s authoritative data and would require unsupported external sources. | Framework crosswalk |
