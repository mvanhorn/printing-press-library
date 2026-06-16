# Intel CLI Enhancement Plan — ideas borrowed from TheMattBerman's kits

**Date:** 2026-06-16
**Scope:** Analysis + plan. No code written yet.
**Subjects:** `traffic-intel-pp-cli`, `ecommerce-intel-pp-cli` (private repo) + a new `ads-intel-pp-cli`
**Sources studied:** `TheMattBerman/x-algo-skill`, `seo-kit`, `meta-ads-kit`, `google-ads-copilot`; local `meta-campaign` skill (`~/.agents/skills/meta-campaign`)

> **Decisions locked (2026-06-16):** Intel CLIs will become **actionable** (Apply layer is IN scope, not optional). **`ads-intel` is a go.** **DataForSEO is dropped** — we already have Ahrefs + GSC + GA4 (no Semrush account, so the semrush CLI is not a usable source). Memory loop + movers and the confidence gate are committed P1.

> **Codex review (2026-06-16): REVISE → incorporated.** Codex inspected the actual CLI source and flagged: the Apply safety model was too thin for real-money writes, `ads-intel` v1 was too big, undo isn't truly reversible, child-CLI write surfaces (Shopify meta, Meta Ads mutations) are **not proven to exist**, snapshot storage had no retention/provenance design, and creative correlations risk being claimed on weak data. All folded into §3–§7 below. Net effect: ads-intel v1 is **read-only**; the only v1 mutation anywhere is **exact-negative-keyword adds on one ad platform**, gated hard.

---

## 1. What we have today

Both CLIs are **local-first aggregators**: `sync` pulls JSON from child CLIs into a local profile model, then read-only analysis commands turn that into recommendations. Today they **recommend; they never act** — that changes (§4).

- **traffic-intel:** `money-pages`, `query-revenue`, `explain-drop`, `refresh-queue`, `opportunity-gap`, `quick-wins`, `revenue-at-risk`, `refresh-brief`, `cannibalization`, `topic-clusters`, `source-coverage`, `internal-link-plan`, `experiment-plan`, `forecast-impact`, `stale-winners`, `digest weekly`.
- **ecommerce-intel:** `dashboard`, `opportunities`, `action-plan`, `geo-audit`, `money-products`, `category-actions`, `product-actions`, `merchandising-link-plan`, `inventory-risk`, `restock-winners`, `cannibalization`, `category-clusters`, `source-coverage`, `forecast-impact`, `explain-drop`, `query-revenue`.

**Child CLIs available (verified, both repos):** `ahrefs`, `google-analytics`, `google-search-console`, `google-ads`, `meta-ads`, `amazon-ads`, `shopify`, `klaviyo`. (`semrush` CLI also exists but there's no Semrush account, so it's not a usable source.) On PATH today: all except `meta-ads-pp-cli` (exists in library, just needs `go install`).

The gaps are **architectural**, not coverage.

## 2. What Berman's kits do that we don't (the transferable ideas)

| Idea | Source | What it is |
|---|---|---|
| **Compounding memory loop** | seo-kit, both ad kits | snapshot → diff vs. last snapshot → act on what's _already moving_ → log learnings → next cycle is smarter |
| **Confidence-as-a-gate** | google-ads-copilot | assess data trust FIRST; low confidence hard-blocks downstream recs |
| **Refuse-to-fabricate** | google-ads-copilot (PMax fallback) | when data is insufficient, show raw rows + decline to compute derived metrics |
| **Draft → Apply → Verify → Undo** | both ad kits | safe mutation: dry-run artifact, PAUSED-only, reversal registry, `undo` |
| **Status header + prioritized tiers** | google-ads-copilot | reports lead with freshness/confidence/mode; findings ranked fix-first/quick-win/strategic/refinement |
| **Opinionated filters** | seo-kit (Strike Zone 5–20), x-algo (archetypes) | sharp prioritization beats exhaustive dumps |
| **Durable Intent Map** | google-ads-copilot | persisted buyer-intent model per product/page that compounds |
| **Source-grounded + confidence-tagged knowledge** | x-algo | mark code-confirmed vs. inferred; "relative not absolute" scoring |

---

## 3. Enhancements to EXISTING CLIs (ranked)

### P1 — Compounding memory + movers detection `[committed]`
**Problem:** `sync` overwrites; no week-over-week diff or learnings log, so nothing compounds.
**Build:**
- Snapshot every `sync` to `~/.traffic-intel-pp-cli/snapshots/<profile>/<date>.json` (+ ecom equivalent). **Storage design is required, not incidental** (today `SaveData` overwrites one `<profile>-data.json`): each snapshot carries a **schema version**, the **source-command versions** used, the **date range**, and **input hashes** for provenance; add **retention + compaction** (e.g. keep daily for 30d, weekly thereafter) so it doesn't grow unbounded.
- New command `movers`: climbers / droppers / new Strike-Zone entrants / new revenue-at-risk vs. last snapshot, with human-readable callouts ("3 pages entered the Strike Zone", "LCP +400ms").
- Persisted `learnings.md` per profile; `explain-drop` and `experiment-plan` outcomes append.
- Reframe `digest weekly` to lead with movers and "act on what's already moving" (highest-leverage idea across all four repos).

### P1 — Confidence-as-a-gate (+ refuse-to-fabricate) `[committed]`
**Problem:** `source-coverage` is advisory; `forecast-impact`/`revenue-at-risk` compute happily on thin/broken data.
**Build:**
- A `confidence` score per profile (High / Medium / Low / Broken) from source coverage, freshness, sample size, conversion/revenue signal presence.
- Gate: forecasting and revenue-at-risk **caveat or refuse** under Low/Broken; print "fix tracking first" (mirrors google-ads-copilot blocking budget scaling).
- Refuse-to-fabricate: when a page/product lacks GA4 revenue or GSC impressions, present raw evidence and skip derived metrics rather than imputing.
- Borrow tracking-confidence checks: duplicate counting, micro-conversion pollution, `conversions` vs `all_conversions` divergence (where Shopify/GA4 allows).
- **Child-CLI schema contract (prerequisite for confidence/movers/apply).** Today's sync parsers flatten arbitrary JSON and guess field names — fine for reports, not for trust-gated logic. Require child CLIs to emit schema + version metadata; **fail closed on unknown/unsupported versions** for anything feeding confidence, movers, or Apply (reports may still degrade gracefully).

### P2 — Report shape: status header + prioritized tiers
- Mandatory header on `dashboard`/`action-plan`/`digest`: profile, data range used, source coverage, confidence, mode.
- Re-tier `action-plan`/`opportunities` into **Fix-first → Quick-win → Strategic → Refinement** with cross-item dependencies ("close tracking gap before trusting this forecast").
- Date-range fallback protocol: start 30d; if empty/thin widen to 90d/12mo and _announce_ the range used.

### P2 — Opinionated prioritization filters
- Make the **Strike Zone (positions 5–20)** an explicit, named filter in `opportunity-gap`/`quick-wins` with defend(1–4)/move(5–20)/ignore(21+) framing.
- For `refresh-brief`/`experiment-plan`, add an x-algo-style **content scorer**: archetype/intent detection → relative score → structured strengths / weaknesses / suggestions, labeled "relative, not absolute."

### P3 — Durable Intent Map (ecommerce-intel first)
- Persist a buyer-intent model per product/collection/query (transactional / comparison / research / informational), updated each sync, feeding `category-actions` and `merchandising-link-plan`.

### P3 — Enrich `geo-audit` with seo-kit's anti-AI-Overview tests
- Add concrete GEO checks: "could an AI Overview answer this from public sources?" gates, experience-anchor / schema (Article/FAQ/HowTo) / `llms.txt` coverage, answer-engine readiness scoring.

---

## 4. Apply layer — make the intel CLIs ACT (Draft → Apply → Verify → Undo) `[committed — in scope]`

The CLIs stop being read-only. Berman's ad kits give the safe pattern — but Codex was right that "mode dial + dry-run + undo" is necessary, not sufficient, for real-money/live-store writes. The hardened model:

- **Prerequisite — the write surface must exist first.** Codex verified `shopify-pp-cli` is mostly read + order tagging (no product/collection meta-write contract), and `meta-ads-pp-cli` is read-heavy. **Before any aggregator Apply, add the narrow write commands to the child CLIs themselves**, each with its own dry-run + tests. No aggregator writes through a child surface that doesn't yet exist.
- **Drafts:** formalize recommendations as markdown/JSON draft artifacts under `<profile>/drafts/` (target, detail, risk, reversibility, evidence, confidence). Drafts carry **idempotency state** so re-runs detect "already applied."
- **Narrow write surface, value-to-risk ordered** (once the child commands exist): traffic-intel **internal-link insertion** first (most reversible), then page **meta/title**; ecommerce-intel **Shopify meta/copy** later. Never destructive (no deletes, no bulk overwrites).
- **Safety controls (all required, not optional):**
  - Explicit **`--live-approved` CLI flag** to mutate — **no env-only live mode**. Modes: `mock` / `read-only` / `live-approved`.
  - **Per-account allowlist** + caps: max-changes-per-run, and for ads max-budget-delta / max-daily-budget.
  - **Idempotency keys** per action; detect already-applied negatives, existing meta values, already-paused entities, unchanged budgets, stale drafts — skip cleanly.
  - **Advisory lock** per account/profile so two runs can't write concurrently.
  - **Pre-write snapshot** of the target's current state; **read-after-write verification** (re-query to confirm); **partial-failure handling** (one action's failure doesn't corrupt the batch).
  - Dry-run table + typed `confirm`; execute one-at-a-time; append-only audit log.
- **Undo is best-effort, NOT a guarantee.** Reversal registry stores the inverse action, but ad-account pauses/budget changes affect auctions, learning state, pacing, and already-spent money — say so explicitly in output; never imply a clean rollback.
- **Confidence-gated:** no writes under Low/Broken confidence (ties to §3 P1).

Deliberate scope expansion. Sequence it last, after P1 analysis upgrades land, so writes are only ever proposed on trusted data — and after the child write-commands exist.

---

## 5. New CLI: `ads-intel` `[committed — go]`

Mirror the traffic-intel architecture for **paid**: aggregate `google-ads` + `meta-ads` + `amazon-ads` child CLIs into one local model, then port **google-ads-copilot's methodology**. Codex flagged the original scope as too big and "all child CLIs in hand" as overclaimed (presence ≠ capability: Meta Ads is read-heavy, Amazon has some automation, Google has generic mutate surfaces). So v1 is **read-only**.

### 5.v1 — read-only aggregator + auditor `[v1 scope]`
- **Sync + account-status header** on every output: account/CID, active/suspended/dormant, date range used, tracking confidence, mode.
- **Tracking-confidence first** (gates everything downstream), then a **deterministic structural audit** (waste, structure, fatigue, kill/scale thresholds from the meta-campaign rules) as PASS/WARN/FAIL.
- **Negative-keyword candidates** and other findings emitted as **draft artifacts only** — no writes.
- **Cross-channel view:** `budget-shift` analysis (where spend is winning vs. bleeding across Google/Meta/Amazon) — the thing single-platform kits can't do. Read-only.
- Date-range fallback protocol + refuse-to-fabricate (don't compute CPA/ROAS when the API didn't return cost rows).
- **Check catalog as data (from `AgriciDaniel/claude-ads`, 6.1k★).** Encode the structural audit as a versioned YAML/JSON catalog the Go CLI reads — stable check IDs, severity multipliers (Critical 5 / High 3 / Med 1.5 / Low 0.5), per-platform category weights summing to 100, PASS/WARN/FAIL bands — with a `go test` asserting catalog↔command coverage + scoring determinism (their bidirectional CI check, ported). Finding schema `{id, severity, platform, title, impact, action, owner, eta_days}` + a Quick-Wins selector (`severity∈{high,critical} AND fix<15min`, sorted by severity×impact).
- **Borrow the accuracy-noted heuristics, not just thresholds:** wasted spend = term with **>$10 spend AND 0 conv** (PASS <5% / WARN 5–15% / FAIL >15% of spend); zero-conv kill = >3 keywords with >100 clicks & 0 conv; **don't flag BROAD+Manual-CPC (legacy BMM)** — only BROAD under Smart Bidding; classify a campaign "brand" by **>50% branded keyword text**, not its name; count **shared** negative lists so covered campaigns aren't false-flagged. Layer Repo-2's per-platform benchmark bands (CTR/CPC/CPM/ROAS/frequency) + 4-tier search-intent taxonomy as supporting data.
- **Quality-gate discipline (never mutate during learning):** even read-only recommendations are gated — never advise an edit during a campaign's active learning phase; verify the tracking/consent stack before issuing any optimization. This posture is what the §5.v2 mutation inherits.

### 5.v2 — one narrow mutation `[after v1 + child write-commands exist]`
- **Exact-negative-keyword adds, ONE platform first** (Google Ads), behind the full §4 hardened Apply model (allowlist, caps, idempotency, pre-write snapshot, read-after-write verify, `--live-approved` flag).
- **Pause keyword/ad-group and set-budget stay draft-only** — explicitly deferred past v2. They affect learning/pacing/spend and undo can't truly reverse them.

### 5.v3 — creative intelligence (the `meta-campaign` taxonomy) `[deferred]`

The `meta-campaign` skill (local: `~/.agents/skills/meta-campaign`) is a creation tool, but its taxonomies **invert into audit/classification logic** — the one thing Ads Manager structurally can't do: explain *why* a creative works. **Deferred out of v1** (Codex: classification reliability + hallucinated-correlation risk). When built, guardrails are mandatory: **min sample sizes + spend thresholds before claiming any "winner," label thin findings "directional only,"** and store every LLM/vision label with its **confidence + review status — never as fact.** Port these:

- **Creative classification on three orthogonal dimensions**, then correlate each against CTR/ROAS/spend from the insights API:
  - **Angle** (5): Pain Point / Social Proof / Curiosity Gap / Transformation / Differentiation.
  - **Hook** (6): Question / Statement / Number / Command / Story-POV / Challenge.
  - **Image archetype** (20): before-after, comparison-table, strong-copy, chat, fake-social, notification, meme, listicle, product-hero, apple-notes, question, process, collage, quotes, hand-writing, device-mockup, claim, bullets, offer, social-post.
  - The skill's literal pattern strings ("Still…?", "From … to …", "Join … others") make LLM/vision classification reliable; output e.g. *"Pain-Point + Question-hook + before-after = your winning combo; you're underspending it."*
- **Structural auditor (deterministic PASS/WARN/FAIL)** from `campaign-structure.md` — transcribe its rules into checks against live Graph API objects: 3–5 ad sets/campaign, ≥$10–20/day per ad set, CBO-vs-budget-size fit, naming-convention regex, missing retargeting campaign, missing 4:5 / 9:16 formats, **frequency > 3 (fatigue)**, **CTR < 0.5% past 2× target CPA (kill)**, budget jumps > 2×/day, ad sets stuck in learning (<50 events/wk).
- **Copy linter** from `copy-guidelines.md` — char limits (headline 40 visible, hook-in-first-125), vague-CTA list, jargon/hype detection, brand-name front-loading; for image ads the computable ≤20%-text / contrast / safe-zone rules.
- **Competitor creative gap** — pull competitor ads from the **Meta Ad Library** (the skill's Stage 5 approach), classify with the same taxonomy, surface "competitors run chat + social-post; you never have."
- **Liftable artifact:** `extract_brand.py` (pure Python, no API) scrapes a URL's palette/fonts/logo with curated CMS/common-color denylists — drop-in competitor brand scraper for ecommerce-intel/traffic-intel too.

This is the feature that turns `ads-intel` from a passive reporter into an opinionated creative+structural auditor. Most of it is transcription from existing tables, not net-new design.

## 6. Other new CLIs — considered and parked

- ~~`dataforseo` child CLI~~ — **dropped.** Ahrefs (keywords + backlinks) + GSC (your real positions) + GA4 cover it. No Semrush account, but Ahrefs already fills that role.
- `link-intel` — seo-kit's link workflows (unlinked mentions, broken-link prospecting, orphan detection, hub-and-spoke). **Fold into traffic-intel** as commands rather than a separate CLI.
- `health-intel` — PageSpeed Insights (LCP/INP/CLS) + crawl audit, trended via the §3 memory loop. Optional later; PageSpeed Insights is a clean printing-press CLI candidate. Parked.
- `demand-intel` — frontrun/reddit-ai-kit style category foresight (customer voice + live search demand). Exploratory, parked.

---

## 7. Build sequence (revised per Codex review)

1. **Memory snapshots + movers** (traffic-intel first, then ecommerce-intel) — with the storage design from §3 (schema version, provenance, retention).
2. **Confidence gate + refuse-to-fabricate** + the child-CLI schema contract (fail-closed for trust-gated logic).
3. **Report headers + prioritized tiers** (+ Strike Zone filter) — cheap, high perceived value.
4. **`ads-intel` read-only** — aggregator + account-status header + tracking confidence + deterministic structural/waste audit + draft artifacts (§5.v1).
5. **First mutation, narrowly:** add the child write-command for **exact negative keywords on one platform**, then the §4 hardened Apply for it only (idempotency, verification, rollback audit) — §5.v2.
6. **Later:** child Shopify write-commands → traffic/ecommerce Apply (internal links, then meta); pause/budget ad mutations; creative classification (§5.v3).

## 8. Resolved decisions
- **Read-only?** No — Apply layer is in scope (§4).
- **`ads-intel`?** Yes — build it (§5).
- **DataForSEO?** No — Ahrefs + GSC + GA4 cover it; no Semrush account but Ahrefs fills that role (§6).
- **Memory loop + movers / confidence gate?** Yes — both committed P1 (§3).
