## Customer model

### Review operations lead

**Today (without this CLI):** They export from the Judge.me dashboard, compare storefront counts manually, and cannot know whether an API pull silently stopped at the 10,000-row ceiling.

**Weekly ritual:** Reconcile new reviews, moderation status, and product-level hotspots before merchandising and CX meetings.

**Frustration:** An apparently successful pull can be incomplete, and the raw API mixes storefront-visible and internal populations.

### Customer-insight analyst

**Today (without this CLI):** They download review data, clean it in notebooks, and manually deduplicate syndicated bundle reviews before VoC analysis.

**Weekly ritual:** Export a filtered corpus by date, rating, and product for theme and product-quality analysis.

**Frustration:** Row counts overstate independent customer voices when one body is syndicated across multiple products.

### Commerce integration engineer

**Today (without this CLI):** They maintain one-off curl scripts with tokens in query strings and build bespoke pagination, webhook, and moderation safeguards.

**Weekly ritual:** Verify integrations, inspect webhooks/settings, and support review-operations automations.

**Frustration:** Unsafe mutation defaults and URL-logged credentials create avoidable operational risk.

## Candidates (pre-cut)

| Feature | Command | Persona | Source | Verdict | Long Description |
|---|---|---|---|---|---|
| Completeness-safe mirror | `sync --resources reviews --full` | Review operations lead | user briefing, live defect receipt | keep | Use this command to refresh the canonical local corpus. Do NOT use `reviews index` to claim corpus completeness. |
| Filtered corpus export | `reviews export` | Customer-insight analyst | user briefing, official API | keep | none |
| Population census | `reviews populations` | Review operations lead | live account evidence, Judge.me status docs | keep | Use this command for published/hidden/pending/archive counts. Do NOT use `reviews unique-bodies` for moderation populations. |
| Syndication clusters | `reviews syndication` | Customer-insight analyst | user briefing, receipt body duplication | keep | Use this command to find the same body attached to multiple products. Do NOT use it for a deduplicated export; use `reviews unique-bodies` or `reviews export --unique-bodies`. |
| Unique-body view | `reviews unique-bodies` | Customer-insight analyst | user briefing, receipt counts | keep | Use this command for one representative row per normalized body. Do NOT use it for syndication cluster diagnostics; use `reviews syndication`. |
| VoC phrase miner | `reviews phrases` | Customer-insight analyst | optional user idea | kill: useful implementation requires NLP or brittle token heuristics | none |
| Sentiment classifier | `reviews sentiment` | Customer-insight analyst | speculative | kill: external/LLM dependency | none |
| Background review watcher | `reviews daemon` | Review operations lead | speculative | kill: persistent process scope creep; webhooks already exist | none |
| Storefront screenshot verifier | `reviews storefront-check` | Review operations lead | speculative | kill: requires browser runtime and unrelated page semantics | none |
| Automatic SKU attribution repair | `reviews reassign` | Review operations lead | syndication defect | kill: cannot infer the true product association safely | none |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona Served | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Completeness-safe mirror | `sync --resources reviews --full` | 10/10 | Review operations lead | hand-code | Calls `/reviews/count` and rating-partitioned `/reviews`, detects all-seen page loops, deduplicates IDs, and commits only on exact equality. | Live 12,871-row defect receipt; user requirement; official count endpoint. | Use this command to refresh the canonical local corpus. Do NOT use `reviews index` to claim corpus completeness. |
| 2 | Filtered corpus export | `reviews export` | 9/10 | Customer-insight analyst | hand-code | Reads the verified SQLite review table and emits date/rating/product/population filters as JSON or CSV. | User requirement; downstream commerce-intel plan; official review fields. | none |
| 3 | Population census | `reviews populations` | 9/10 | Review operations lead | hand-code | Aggregates explicit moderation/storefront populations from local published, curated, and hidden fields. | 9,638 published vs 3,229 hidden live evidence; Judge.me four-status documentation. | Use this command for published/hidden/pending/archive counts. Do NOT use `reviews unique-bodies` for moderation populations. |
| 4 | Syndication clusters | `reviews syndication` | 9/10 | Customer-insight analyst | hand-code | Groups the local mirror by normalized SHA-256 body hash and reports bodies attached to multiple product identifiers. | User requirement; 12,871 rows vs 9,552 unique non-empty bodies in receipt. | Use this command to find the same body attached to multiple products. Do NOT use it for a deduplicated export; use `reviews unique-bodies` or `reviews export --unique-bodies`. |
| 5 | Unique-body view | `reviews unique-bodies` | 8/10 | Customer-insight analyst | hand-code | Selects one deterministic representative per body hash from the verified local review table while preserving source row counts. | User requirement; receipt demonstrates syndicated duplicates. | Use this command for one representative row per normalized body. Do NOT use it for syndication cluster diagnostics; use `reviews syndication`. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| VoC phrase miner | Mechanical token frequency would be noisy; meaningful phrases require NLP, which the rubric disallows. | Filtered corpus export |
| Sentiment classifier | Requires an external model or service outside the API. | Filtered corpus export |
| Background review watcher | Persistent daemon scope exceeds a command; Judge.me webhooks cover events. | Completeness-safe mirror |
| Storefront screenshot verifier | Requires a resident browser and storefront-specific rendering. | Population census |
| Automatic SKU attribution repair | Product truth cannot be inferred safely from syndicated bodies. | Syndication clusters |
