# Allrecipes — Novel Features Brainstorm (subagent output)

This is the full audit trail from the Phase 1.5c.5 novel-features subagent
spawn for run `20260503-154931`. The Customer model and Killed candidates
sections are persisted here for retro/dogfood reference; the Survivors
become the transcendence table in the absorb manifest.

## Customer model

**Persona 1 — Weeknight Cook Casey (home cook)**
- Today: Opens Allrecipes on phone at 5:30pm, scrolls past ads and a 12-paragraph life story to find the ingredient list, half the time forgetting they're missing an egg.
- Weekly ritual: Picks 3-4 dinners on Sunday, screenshots ingredient lists, makes a hand-merged shopping list.
- Frustration: The site can't answer "what proven recipe can I make in 25 min with chicken thighs and spinach I already have?"

**Persona 2 — Meal-Plan Agent (Claude/MCP host driving the CLI)**
- Today: Asked by user to "plan three quick dinners and give me a grocery list," has to chain raw scrapes across multiple URLs and reconcile units in-prompt.
- Weekly ritual: Multi-recipe synthesis — pick → fetch → scale → aggregate → export.
- Frustration: Without typed JSON-LD and a reverse ingredient index, every plan re-parses the same recipes; results are stochastic and ad-polluted.

**Persona 3 — Cookbook Curator Cam (gift-giver / hobbyist)**
- Today: Wants to bundle "20 best Italian weeknights" as a markdown/PDF gift, currently copy-pastes recipe by recipe.
- Weekly ritual: Themed collections (holiday, regional, dietary) packaged for friends or personal use.
- Frustration: No tool produces a curated, attributed cookbook from Allrecipes; the site's "save" feature doesn't export as portable data.

**Persona 4 — Dietary-Constrained Dani (gluten-free / vegan / low-carb cook)**
- Today: Allrecipes' on-site diet pages have spotty coverage; recipes labeled "gluten-free" still hide gluten in soy sauce or marinades.
- Weekly ritual: Browses dish categories then manually scans ingredient lists to confirm the recipe is actually safe.
- Frustration: Needs strict ingredient-pattern filtering across the local corpus, not the site's loose keyword tagging.

## Candidates (pre-cut)

| # | Candidate | Source | Kill/keep | Raw score | Notes |
|---|-----------|--------|-----------|-----------|-------|
| 1 | Pantry match | (a) Casey, (d) prior-keep, (c) cross-entity | KEEP | 10/10 | Anchor; only command answering "what can I cook with what I have" |
| 2 | Bayesian top-rated | (b) service-specific, (d) prior-keep | KEEP | 10/10 | Site rating sort surfaces 1-review 5-stars |
| 3 | Reverse ingredient index | (c) cross-entity, (d) prior-keep | KEEP | 9/10 | Native search title-only |
| 4 | Quick weeknight | (a) Casey, (d) prior-keep | KEEP | 10/10 | Site has no numeric time cap |
| 5 | Personal cookbook export | (a) Cam, (d) prior-keep | KEEP | 9/10 | Composes browse + Bayesian + markdown |
| 6 | Dietary filter on cache | (a) Dani, (d) prior-keep | KEEP | 9/10 | Site mis-tags GF |
| 7 | Doctor with Cloudflare clearance diagnosis | (b) service-specific, (d) prior-keep, (e) user vision | KEEP | 10/10 | Cloudflare cf-mitigated header detection |
| 8 | Multi-recipe grocery list | (d) prior-built | DROP from candidate pool — already absorbed (#19) | n/a | |
| 9 | Pantry-aware grocery list | (a) Casey, (c) cross-entity | KEEP | 8/10 | Differentiates from absorbed grocery-list |
| 10 | Made-It rank | (b), (f) | KILL | 7/10 | Speculative; overlaps Bayesian |
| 11 | Swap-aware search | (a), (c) | KILL | n/a | Pairing logic fuzzy without LLM |
| 12 | Meal-plan generator | (a), (b) | KILL | n/a | Scope creep |
| 13 | Review sentiment summary | (b) | KILL | n/a | LLM dependency |
| 14 | Nutrition rollup | (a), (c) | KILL | 6/10 | Niche; covered by jq pipe |
| 15 | Random "surprise me" | (a) | KILL | n/a | Sibling kill: `top-rated | shuf -n 1` |
| 16 | Cuisine cookbook | (a) | KILL | n/a | Folded into `cookbook --cuisine` flag |
| 17 | Stale-cache report | (b), (f) | KILL | 6/10 | Machine-owned freshness already covers it |

## Survivors and kills

### Survivors

(8 transcendence features — see absorb manifest for full table format)

### Killed candidates

| Candidate | Kill reason | Closest surviving sibling |
|-----------|-------------|--------------------------|
| Swap-aware search (#11) | Substitution-pairing logic is fuzzy and unverifiable without LLM normalization | `with-ingredient` (#3) |
| Meal-plan generator (#12) | Scope creep — balancing constraints across N days is application-shaped | `quick` (#4) + agent loop |
| Review sentiment summary (#13) | LLM dependency; mechanical n-gram is low-signal | Absorbed `reviews <url>` (manifest #7) |
| Nutrition rollup (#14) | Niche persona overlap; 6/10 below survivor floor | `recipe --json --select nutrition` + `jq` |
| Random suggestion (#15) | One-liner: `top-rated <q> \| shuf -n 1` | `top-rated` (#2) |
| Cuisine cookbook (#16) | Sibling kill: folded into `cookbook --cuisine` | `cookbook` (#5, reframed) |
| Made-It rank (#10) | Speculative user-pain; below survivor floor | `top-rated` (#2) |
| Stale-cache report (#17) | Thin user-pain; machine-owned freshness already addresses | Built-in freshness in covered command paths |

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---------------|---------|---------------|
| Pantry match (`pantry`) | Keep | Persona fit, 10/10, prior command preserved |
| Bayesian top-rated (`top-rated`) | Keep | Persona fit, 10/10, prior command preserved |
| Reverse ingredient index (`with-ingredient`) | Keep | Persona fit, 9/10, prior command preserved |
| Quick weeknight (`quick`) | Keep | Persona fit, 10/10, prior command preserved |
| Personal cookbook export (`cookbook`) | Reframe | Same command, adds `--cuisine` flag |
| Multi-recipe grocery list (`grocery-list`) | Drop | Already in absorb manifest as #19 |
| Dietary filter on cache (`dietary`) | Keep | Persona fit, 9/10, prior command preserved |
| Doctor with Cloudflare diagnosis (`doctor`) | Reframe | Diagnostic shifts to `cf-mitigated` + clearance-cookie prescription |
