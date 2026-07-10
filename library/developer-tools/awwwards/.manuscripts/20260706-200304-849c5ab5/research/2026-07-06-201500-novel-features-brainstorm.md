# Novel Features Brainstorm — awwwards (subagent audit trail)

Subagent: general-purpose, 3-pass (customer model -> 2x candidates -> adversarial cut).
Prior research: none (first print). Full verbatim output follows.

## Customer model

**Mara — art director at a 6-person studio.** Today: three pinned Awwwards tabs every Monday (SOTD feed, /websites/e-commerce/, dark-mode collection); one-filter-at-a-time means "dark e-commerce from European studios" = an afternoon of tab-clicking; screenshots cards into Figma by hand; cannot answer "which scored well on usability" without opening every detail page. Weekly ritual: Monday inspiration sweep - 8-12 reference sites per client brief + palettes + jury validation. Frustration: the one-filter-at-a-time wall.

**Diego — creative front-end developer at an agency.** Today: deep-dives 2-3 winners/week to reverse-engineer techniques from tag lists and Developer Award sub-scores; /elements/footer/ is a flat unranked screenshot wall disconnected from parent-site scores. Weekly ritual: pattern research by section before building each hero/404/footer. Frustration: no "heroes from 9+ scoring sites" view - clicks through one at a time.

**Suki — design-trends writer with a weekly newsletter.** Today: hand-tallies two weeks of winners in a spreadsheet every Thursday; historical questions ("is brutalism fading vs Q1?") unanswerable; community RSS/scraper tools all died by 2018. Weekly ritual: quantified trend snapshot across tags/colors/fonts/tech. Frustration: aggregation impossible on the site.

**Alex — indie builder who ships landing pages with a coding agent.** Today: agent designs from training-data taste; grounding it in what wins awards means pasting screenshots into prompts; jury scores locked in SSR HTML. Weekly ritual: every build starts with a context-gathering step - "what does great look like for this kind of site" as machine-readable input. Frustration: no queryable, offline, agent-shaped design-intelligence source exists.

## Candidates (pre-cut) — 14 generated

C1 trends (keep) · C2 context (keep) · C3 palette-match (keep) · C4 elements-top (keep) · C5 studio (keep) · C6 compare (soft) · C7 juror (soft) · C8 benchmark (reframe into context) · C9 grab (kill-leaning: thin wrapper) · C10 new-since (kill-leaning: duplicates latest+sync) · C11 divergence (soft, speculative) · C12 devtop (kill-leaning: duplicates top) · C13 cooccur (kill-leaning: marginal over trends) · C14 annual (kill-leaning: fails weekly-use by construction)

All passed LLM-dependency, external-service, and auth kill-checks (mechanical extraction/aggregation over single already-specced source; no auth exists).

## Survivors and kills

### Survivors (5, all hand-code)

| # | Feature | Command | Score | Buildability | Persona | Evidence |
|---|---------|---------|-------|--------------|---------|----------|
| 1 | Trend snapshot | trends --by tag\|color\|tech\|font --since 90d [--vs 90d] | 10/10 | hand-code | Suki | Brief Top Workflow 4 verbatim |
| 2 | Design context pack | context --category <c> [--tag --tech --color] | 10/10 | hand-code | Alex | Brief User Vision verbatim |
| 3 | Fuzzy palette match | palette-match <hex> --distance <n> | 8/10 | hand-code | Mara/Alex | Data Layer; server is exact-hex only |
| 4 | Ranked elements | elements-top <type> --dim design [--min 8] | 8/10 | hand-code | Diego | Brief Top Workflow 3 |
| 5 | Studio profile | studio <name> | 6/10 | hand-code | Mara/agencies | Brief Top Workflow 5 + credits data |

Pass-3 force-answers recorded per survivor (weekly use, wrapper check, transcendence proof, sibling kill, buildability, long-desc validity) — all passed; see conversation transcript for full text.

### Killed candidates (9)

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C6 Site compare | "occasionally" = soft kill; two inspect --json calls cover it | inspect (absorbed) |
| C7 Juror analytics | no weekly ritual; curiosity feature | studio |
| C8 Score benchmark | single number, not a command; ships as benchmarks block inside context | context |
| C9 Moodboard grab | thin wrapper: find --select thumbnail_url | xargs curl | palette-match |
| C10 New-since diff | duplicates latest + sync --since | trends |
| C11 Jury/community divergence | speculative, zero demand evidence | trends |
| C12 Developer-award ranking | absorbed top already ranks any dimension | elements-top |
| C13 Tech co-occurrence | marginal over trends scoped by tech | trends |
| C14 Annual digest | yearly cadence fails weekly-use bar | trends |
