# art-goat — state of the CLI (2026-05-21)

Snapshot taken after the v1.1 + scorecard + review-fix session. Pairs with the
two retro docs in this same `proofs/` directory (v1 generator hang + multi-source
aggregator findings; v1.1 scorer bug finding).

## 1. Feature surface area (everything shipped today)

**Contemplative spine (the soul):**
- `sit [id]` — atomic contemplative timer with reflection capture. Flags: `--duration`, `--source`, `--mode`, `--launch` (HTML+browser), `--inline` (terminal graphics), `--no-image`, `--dry-run`
- `today` — anti-repeat + journal-aware diversity pick with "why this today" derivation
- `presence` — random piece + prompt without a timer
- `random` — bare random pick, no prompt
- `journal write` — capture reflection (with or without an associated work)
- `journal search "term"` — FTS5 over reflection history
- `journal stats` — reframed metrics (breadth, variety, region, mood drift) with streak labeled "if you want to know"
- `journal export` — one-way SQLite-to-Markdown mirror at `~/.art-goat/journal/` (override via `ART_GOAT_JOURNAL_PATH`)
- `journal opt-in --show-streak` — opt-in per-sit streak greeting
- `journal compact --confirm` — VACUUM + FTS5 rebuild (admin)

**Federated cross-source surface:**
- `browse` — paginate unified works with `--source`, `--medium`, `--region`, `--limit`, `--offset` filters
- `similar <work-id>` — FTS over medium/period/region/creator with structured fallback
- `compare <a> <b>` — field-by-field side-by-side
- `artist <name>` — chronological cross-source listing by creator
- `coverage` — practice breadth as %: sources/regions/mediums sat vs available; `--top N` cap
- `gaps` — top-N unsat regions and mediums

**Infrastructure (generator-emitted):**
- `sync` (generic), `sources sync` (per-source), `sources list`
- `doctor` — health check, auth state, cache report
- `auth set-token/status/logout`
- `agent-context` — emit JSON describing the CLI tree for agents
- `feedback [text]` and `feedback list` — local-first feedback capture
- `profile save/use/list/show/delete` — saved flag-set profiles
- `which "<capability>"` — natural-language command resolver
- `import` — JSONL ingest
- `api` — endpoint browser
- `planetary apod-get` — typed AIC/APOD endpoint (the only typed MCP tool today)

## 2. Data sources

| Source | Slug | Auth | License | Backing | Live-verified |
|---|---|---|---|---|---|
| Art Institute of Chicago | `aic` | none | CC0/CC-By | Live API | Yes |
| NASA APOD | `apod` | DEMO_KEY (or `NASA_API_KEY`) | Public domain | Live API | Yes |
| Metropolitan Museum of Art | `met` | none | CC0 | Live API | Yes (5 works pulled) |
| Cleveland Museum of Art | `cleveland` | none | CC0 | Live API | Yes (5 works pulled) |
| Rijksmuseum | `rijks` | required (`RIJKSMUSEUM_API_KEY`) | Public domain | Live API | No (auth-error path verified; no key set this session) |
| Smithsonian Open Access | `smithsonian` | DEMO_KEY (or `SMITHSONIAN_API_KEY`) | CC0 | Live API | Partial — Solr-quote bug fixed via direct curl probe; binary surfaced rate-limit error cleanly. Full live-pull pending fresh DEMO_KEY quota or a personal api.data.gov key |
| Te Papa Tongarewa | `tepapa` | required (`TEPAPA_API_KEY`) | Te Papa terms | Live API | No (auth-error path verified; no key set this session) |
| National Palace Museum Taiwan | `npmtw` | none | Public domain | **Static-curated subset** (9 famous works; Wikimedia Commons image URLs verified) | Yes (parser exercises the static list) |

## 3. Manual work needed to get full access

Free signups required (all four are no-cost, no credit card):

1. **NASA APOD personal key** — `NASA_API_KEY` (or `ART_GOAT_API_KEY`). Lifts the 30 req/hr DEMO_KEY ceiling to ~1000/hr. Signup at https://api.nasa.gov/. Without it, full-archive sync (~11,000 entries since 1995) takes >7 hours.
2. **Smithsonian / api.data.gov key** — `SMITHSONIAN_API_KEY` (or `ART_GOAT_API_KEY`). Same api.data.gov backend as NASA; same DEMO_KEY ceiling. Signup at https://api.data.gov/signup/. Without it, only ~30 req/hr.
3. **Rijksmuseum key** — `RIJKSMUSEUM_API_KEY` (or `ART_GOAT_RIJKS_KEY`). Free, 10k req/day quota. Signup at https://data.rijksmuseum.nl/object-metadata/api/. Required to access any Rijks records.
4. **Te Papa key** — `TEPAPA_API_KEY` (or `ART_GOAT_TEPAPA_KEY`). Free, header-based (`x-api-key`). Signup at https://data.tepapa.govt.nz/docs/. Required to access any Te Papa records.

No signup required: AIC, Met, Cleveland, NPM Taipei (static), APOD (DEMO_KEY works), Smithsonian (DEMO_KEY works, capped).

Total practical setup time for full coverage: ~10 minutes across all four signup forms. The CLI surfaces the exact env var name in its error when a required key is missing, so the discovery loop is fast.

## 4. Outstanding / future work

**v2 candidates (explicitly deferred):**
- `path --theme=X` — multi-piece curated walk over a theme (last v1-locked feature still unshipped)
- **NPM Taipei live API** — promote from static-curated to live source if/when the museum publishes a queryable JSON endpoint. Their open-data portal at `digitalarchive.npm.gov.tw/opendata` is a search UI only today.

**Improvements surfaced by code review (printed-CLI tasks):**
- Tests for the freshness helper and 11 new commands — already added in this session
- `RandomWork` uses `ORDER BY RANDOM() LIMIT 1` — scales poorly past ~50k works; a sampling approach would help if anyone runs Smithsonian `--full` (50k cap) or AIC `--full`
- Source-builder live-verify gap (Smithsonian's Solr quoting was missed by the doc-driven subagent) — argues for a "run sources sync against the real API as part of the build pipeline" step

**Generator-side improvements (separate from art-goat):**
- The scorer bug filed in this session — issue #1743 (`type_fidelity` sampling)
- v1 retro's two findings: AIC OpenAPI generator hang, multi-source aggregator pattern docs

## 5. Strategic opportunities

These aren't "features I forgot" — they're directions the CLI is *uniquely positioned* to take that aren't obvious from the v1.1 surface:

### a) The "compare across centuries" play

`compare aic:24645 met:436532` already works field-by-field. But the most interesting cross-museum comparisons are between *similar works the user has actually sat with*. A `journal compare` command that picks two sits from the user's history and contrasts the works + reflections would turn the journal into a longitudinal practice log instead of a flat list. The store has everything needed; the command is ~80 LOC.

### b) Mood-aware `today`

Right now `today` rotates against medium/region/source. But the journal captures mood (1-5). A heavy-mood sit followed by a deliberately calmer pick is a real contemplative practice pattern (rotate energy, not just visuals). `today --mode bridge-from-last` could read the last sit's mood and pick toward a target — no new data, just smarter scoring.

### c) Federated artist arcs

`artist "hokusai"` lists works chronologically. The unbuilt move is `artist "hokusai" --arc` that groups works into stylistic periods using the existing `period` + `date_start` columns and renders a 5-line career narrative. The data is already there; the command is interpretation. This is the kind of thing the `insight` scorer dimension is meant to reward, and it's where the federated multi-museum corpus *uniquely* outperforms any single-museum app.

### d) The "no museum has this" gallery

`gaps` shows you regions/mediums you've never sat with. The flip side — works in your corpus whose source/medium/region appears *nowhere else* — is a discoverable rarity surface. `coverage --orphans` or `unique --by region` would surface "this is the only Bauhaus piece across all 8 sources you've synced." Real value for a contemplative practice: it frames the corpus as a curated map rather than a uniform list.

### e) Spaced repetition for reflection review

`journal search` is keyword-based. A `journal revisit --age 1y` that surfaces sits from N days/months/years ago is a tiny addition with outsized practice value — "what was I noticing on this day a year ago" is the kind of feature that turns a journal into a practice. ~30 LOC, leverages `started_at`.

### f) The OmniMuseum positioning

Eight sources is enough that the *primary* value proposition isn't "AIC + APOD with a timer" anymore — it's "one practice, every major open museum API." The README still leads with the AIC+APOD framing from v1. Repositioning the README opening + the SKILL trigger phrases ("museum aggregator", "open access art practice", "8-source contemplative tool") could meaningfully shift how agents discover and recommend it.

### g) Federated `today` cap

Right now `today` picks anti-repeat against your sit history. With 8 sources synced (~7,500+ works), the diversity score is dominated by source-not-recently-sat-with. A `today --weight region:3,medium:2,source:1` flag (or a `today.toml` config) would let users tune their own rotation policy. Currently the policy is hardcoded.

### h) Reprint with AIC-as-primary-scaffold (new opportunity surfaced 2026-05-21 by issue #1739)

**Context:** Issue #1739 was the v1 retro's P1 finding — the generator hung indefinitely on AIC's 103-endpoint OpenAPI 3.1 spec, which forced v1 to pivot to a minimal hand-authored APOD spec as the primary scaffold and a hand-coded AIC client. The maintainer (tmchow) closed #1739 the same day as "already fixed" on `origin/main`: AIC now parses in **2.60s** (or 8.05s with `--validate`) and the generated CLI passes all gates.

**Why it matters now:** The v1 build's pivot away from AIC was forced by the hang. With the hang fixed, the cleaner shape is AIC as the primary spec, with the 7 other sources still hand-authored under `internal/source/`. Concretely:

- AIC's hand-coded `internal/source/aic/client.go` (~5 endpoints) would become generator-emitted with **all 103 endpoints**, fully typed.
- The typed AIC MCP surface would clear the scorer's `toolDesignMinEndpoints` threshold, lifting `mcp_tool_design` and `mcp_surface_strategy` out of "unscored" status. Could lift total score 3-5 points.
- The other 7 sources stay where they are — they aren't spec-driven and aren't affected.

**Cost:** This is a `printing-press-reprint art-goat` exercise, not a small edit. The reprint workflow is designed for exactly this case — it carries prior research and prior novel features into the reprint reconciliation rather than dropping them. Manuscripts, novel commands (presence/random/browse/similar/compare/artist/coverage/gaps, journal commands), store schema, MCP annotations all need to be re-merged.

**When to do it:** Not now. Ship v1.1 first, collect real-world feedback (which sources do users actually run, which novel commands get used, what does `feedback list` collect over a few weeks), then decide. A reprint without that signal is regen for regen's sake. If usage tells you AIC is the dominant source, the typed-surface gain justifies the merge cost. If users mostly run Met/Cleveland/Rijks/Smithsonian instead, the reprint argument weakens — those are all hand-authored either way.

### Recommended priority (if you want a planned next session)

1. **(f) README/SKILL repositioning** — free, immediate, doesn't need code
2. **(c) federated artist arcs** — high-leverage on existing data
3. **(e) spaced-repetition revisit** — ~30 LOC, big practice payoff
4. **(a) journal compare** — turns flat journal into a longitudinal log
5. **(b) mood-aware today** — best scoring tweak; uses existing mood column
6. **(h) reprint with AIC-as-primary-scaffold** — only after weeks of v1.1 feedback says AIC is the dominant source; otherwise the regen-merge cost outweighs the typed-surface gain

## Pointers

- Library: `~/printing-press/library/art-goat/`
- Manuscripts: `~/printing-press/manuscripts/art-goat/20260521-062440/`
- v1 retro: `~/printing-press/manuscripts/art-goat/20260521-062440/proofs/20260521-150000-retro-art-goat-pp-cli.md`
- v1.1 retro: `~/printing-press/manuscripts/art-goat/20260521-062440/proofs/20260521-095145-retro-art-goat-pp-cli.md`
- Filed retro issue: https://github.com/mvanhorn/cli-printing-press/issues/1743
- Scorecard at snapshot: 91/100 Grade A
- All tests green, build clean, `publish-validate` PASS — ready to publish via `/printing-press-publish art-goat`
