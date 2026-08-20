# coffee-goat Session 2 — novel-features brainstorm + cut (audit trail)

> Subagent output from Phase 1.5 Step 1.5c.5. Used as input to the absorb manifest (Step 1.5d) and research.json (Step 1.5e). Full transcript preserved for retro / dogfood debugging.

## Customer model

**P1 — the diehard enthusiast (10-bag home brewer).** *Today:* Maintains a notes app of bean labels, has 24 roaster site bookmarks, watches Hoffmann/Hedrick reviews on YouTube and tries to remember which roaster he saw mentioned, looks up Coffee Review scores in a separate tab, and consults WBC recipes on Reddit threads. Cannot answer "given the 10 bags actually on my counter, what should I brew tomorrow morning to chase the God cup," because no tool joins his cellar + Hoffmann transcripts + Coffee Review scores + WBC recipes. *Weekly ritual:* Sunday morning palate review — re-rank the 10 bags, watch the week's Hoffmann drop, log brews, and queue 1–2 restocks. *Frustration:* The most signal-rich source he consumes (YouTube reviews of specific bags he owns) is a 20-minute video he can't grep. He cannot answer "did Hoffmann or Hedrick ever review this exact bag" in one motion.

**P2 — Maya, the home barista on a Niche + Linea Mini.** *Today:* Pulls 2–4 espressos daily, watches Lance Hedrick for technique (WDT, puck prep, flow), and dials in espresso by feel over a week per bag. Uses Beanconqueror sometimes but it doesn't know her bag's roast date relative to peak espresso window, doesn't know Hedrick's espresso commentary on the bean, and can't compare today's shot to where she was on day 9. *Weekly ritual:* Daily dial-in log + a Saturday "is this bag dead yet" check on each espresso bag in rotation. *Frustration:* Espresso drifts day-by-day as the bag ages — she has no view that overlays her own rating curve on the per-method peak-freshness window. She also can't quickly find the Hedrick clip where he explains the technique adjustment for her current bean's process.

**P3 — Devon, the small-roastery competitive-intel hire.** *Today:* Bookmarks The Barn, April, La Cabra, Sey, Glitch product pages, scrapes by hand for new producers and processes, watches WBC results when they trend, and pieces together "what won what" from Sprudge and Reddit. Cannot answer "which roasters currently carry beans tied to the last three World Brewers Cup winners" or "which producer is showing up at five+ roasters this season." *Weekly ritual:* Monday catalog sweep across the elite 24 + Friday Sprudge guide skim + post-competition recipe dig. *Frustration:* Recipe lineage is fragmented — WBC champion's recipe lives on Sprudge, the bean lives on a roaster site, the roaster carries a similar lot months later, and nothing connects them.

**P4 — The household agent (operates on behalf of P1/P2).** *Today:* Answers "what should I brew next from the shelf," "find a cafe near me with specialty beans," "did any creator review this bag." Currently must shell-out to ad-hoc scripts. *Weekly ritual:* Every coffee-related prompt the household issues. *Frustration:* No deterministic, structured-output tool exists for "join my cellar with editorial signal and give me one recommendation" — and no tool surfaces cafe finder + champion recipes + creator transcripts as MCP tools.

## Survivors

| # | Feature | Command | Score | Personas | Source |
|---|---------|---------|------|---------|--------|
| 1 | Cross-roaster FTS search | `search` | 10 | P1/P3/P4 | prior-keep |
| 2 | Bayesian dial-in | `dial-in` | 10 | P1/P2 | prior-keep |
| 3 | Cellar + freshness | `shelf` | 10 | P1/P2/P4 | prior-keep |
| 4 | Restock watch | `watch` | 10 | P1/P3 | prior-keep |
| 5 | Multi-bean compare | `compare` | 9 | P1/P3 | prior-keep |
| 6 | Closest twin | `twin` | 10 | P1/P3/P4 | prior-keep |
| 7 | Blind cupping calibration | `blind-cup` | 9 | P1 | prior-keep |
| 8 | FX landed cost | `fx` | 9 | P1 | prior-reframe (`// pp:novel-static-reference` on shipping table) |
| 9 | Producer tracking | `producer` | 10 | P3/P1 | prior-keep |
| 10 | Refill plan | `refill-plan` | 10 | P1/P2/P4 | prior-keep |
| 11 | Roaster style fingerprint | `roaster-style` | 9 | P3 | prior-keep |
| 12 | Rating drift diagnostic | `drift` | 9 | P2/P1 | prior-keep |
| 13 | What to drink next | `whats-next` | 9 | P1/P2/P4 | prior-keep |
| 14 | Creator review lookup | `creator-review <bean>` | 9 | P1/P4 | new |
| 15 | God cup recommender | `god-cup` | 10 | P1/P4 | new (user vision verbatim) |
| 16 | Championship replay | `champion-replay <year>` | 9 | P1/P3 | new |
| 17 | Champion lineage | `champion-lineage --producer X` | 8 | P3 | new |
| 18 | Creator radar | `creator-radar` | 8 | P1/P4 | new |
| 19 | Cafe near | `cafe-near "<location>"` | 8 | P4 | new |
| 20 | Cafe trip planner | `cafe-trip "<city>"` | 7 | P1/P3 | new |
| 21 | Palate map | `palate-map` | 8 | P1/P2 | new |
| 22 | Bag lifecycle | `bag-life <bean>` | 8 | P2 | new |
| 23 | Espresso school | `espresso-school <bean>` | 7 | P2 | new |
| 24 | Review gap | `review-gap` | 7 | P1 | new |
| 25 | Review consensus | `review-consensus <bean>` | 8 | P1/P4 | new |
| 26 | Producer discovery | `producer-discovery` | 8 | P3 | new |
| 27 | Roaster benchmark | `roaster-bench <slug>` | 7 | P3 | new |
| 28 | Transcript search | `transcript-search "<q>"` | 7 | P1/P4 | new |
| 29 | Champion shop | `champion-shop` | 8 | P3/P1 | new |
| 30 | Personal roast window | `roast-window` | 7 | P1/P2 | new |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `shelf-sprudge` | Brief explicitly states cafe→bean linkage is not viable; correlating shelf with city guides is conceptually muddled and unverifiable | `cafe-trip` |
| `brew-twin` | Redundant with `twin` + `whats-next` together; same data, same join, same persona — sibling kill | `twin`, `whats-next` |

## Phase Gate 1.5 user additions (Session 2, round 2)

After the gate showcase the user added:

- **Persona 5 — Mike, espresso enthusiast with a Decent DE1.** Programmable espresso machine with per-shot pressure/flow/temp profiles. *Today:* logs shots in DE1's own app but doesn't cross-reference shot-data with bean attributes or with Hedrick's technique commentary. *Weekly ritual:* daily espresso pulling + post-shot adjustment based on TDS/yield. *Frustration:* Decent's data lives in its own silo; he wants the shelf + freshness + drift view alongside it. **Note: full Decent DE1 shot-file integration stays deferred to v0.2 per Session 1 decision. Mike's espresso-focused workflows are already served by `god-cup --method espresso`, `espresso-school`, `drift`, `bag-life`, `champion-replay --competition wbc`, and `roast-window`.**

- **Persona 1 (P1) method detail:** pour-over primary on V60, Origami Air, Sibarist SD-1 (filter); "soup" immersion via Oxo Rapid Brewer. Flows into the absorbed `method_profiles` table; Phase 3 will seed the registry with these methods + filter accessories.

User-supplied feature ideas (scored at gate review):

- **`friend-pick <friend>` (8/10) — ADD.** Recommend a bag from the user's shelf (or current market) matching an imported friend's palate profile. Joins imported `palate_profile` with cross-roaster corpus + the user's `brews`. Requires an export/import format (`palate-export <name> --out <file>` / `palate-import <file>`) so users can share profiles. Persona P1 + a new "social" axis.
- **`flavor-wheel` (8/10) — ADD.** Map the user's brew ratings onto the official SCA Coffee Tasters' Flavor Wheel hierarchy (fruity → berry → blackberry, etc.). Local aggregation over `brews ⋈ roaster_products.descriptors` joined against an embedded SCA wheel taxonomy (curated static reference, `// pp:novel-static-reference`). Complements `palate-map`. Persona P1 + P2.
- **`beans quick` / `beans add --url` (UX, not a new transcendence row).** Friction-reducing UX on the existing `beans add` flow: accept a roaster URL, a fuzzy product slug, or interactive selection. Phase 3 build priority for the `beans` resource. Not added to the transcendence table.

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---|---|---|
| `search` | keep | Core P1/P3/P4 ritual; passes all checks |
| `dial-in` | keep | P1/P2 weekly ritual centerpiece |
| `shelf` | keep | P1/P2/P4 daily question; freshness curve unchanged |
| `watch` | keep | P1/P3 cron-safe restock |
| `compare` | keep | P1/P3 cross-roaster delta tables |
| `twin` | keep | P1/P3/P4 sold-out replacement core flow |
| `blind-cup` | keep | P1 palate calibration |
| `fx` | reframe | Curated `roaster_shipping` table must carry `// pp:novel-static-reference` |
| `producer` | keep | P3 headline workflow; Session 2 adds champion lineage which complements |
| `refill-plan` | keep | P1/P2/P4; strengthened by new creator-coverage signal |
| `roaster-style` | keep | P3 unchanged |
| `drift` | keep | P2; `bag-life` and `roast-window` adjacent but distinct |
| `whats-next` | keep | P1/P4 explicitly need this; `god-cup` extends without replacing |
