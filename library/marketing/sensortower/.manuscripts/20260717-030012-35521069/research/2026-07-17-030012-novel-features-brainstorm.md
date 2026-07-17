# Novel Features Brainstorm — sensortower-pp-cli (audit trail)

Subagent run: Step 1.5c.5, first print (`PRIOR_RESEARCH_PATH=none`, Pass 2(d) omitted).
Full response preserved for retro/dogfood debugging. Only `### Survivors` flows into the
manifest; Customer model and Killed candidates live here.

## Customer model

**Persona A — Priya, indie iOS developer with one paid utility app.**

*Today:* Every Monday she opens app.sensortower.com in one tab and App Store Connect in
another. Types her app's name into catalog search, clicks to the profile, reads the
`Top Paid → Productivity → US → iPhone` rank off the page. Screenshots it into a Notes file
because the dashboard will not remember it tomorrow. Repeats for three rivals. Cannot answer
"did I actually move up over the last month, or does it just feel that way?" — her only
history is the screenshots she remembered to take. Knows the paid tier is not happening at
her revenue.

*Weekly ritual:* Monday rank check on her app plus three named rivals, two chart types, one
country. Roughly a dozen dashboard page loads — not coincidentally about where the 429 wall sits.

*Frustration:* The dashboard is amnesiac. She is manually re-creating a time series the
service could trivially have kept, and her archive is a folder of PNGs she cannot diff.

**Persona B — Marcus, competitive-intel analyst at a 40-person mobile games studio.**

*Today:* A rival jumps eleven spots in Top Grossing → Games → US overnight and his PM pings
him asking why. He opens the rival's profile, scrolls version history looking for a release
near the spike, opens the IAP list to see whether a new price tier appeared, then flips to
category rankings to check whether the whole category moved or just this app. Four tabs, ten
minutes, correlating "they shipped 30.3 on the 4th" against "they climbed on the 5th" by
eyeballing two lists side by side. Knows the revenue estimate is junk — the studio's own
numbers are nothing like what Sensor Tower says about them — so he treats money as vibes and
ranks as fact.

*Weekly ritual:* Reactive teardowns, two or three a week, plus a standing Friday category sweep.

*Frustration:* The release-timeline-versus-rank-move correlation is the entire question, and
it is the one thing no screen shows him. He does it by hand with a finger on the monitor.

**Persona C — Dana, UA/growth marketer on a cross-platform fintech app.**

*Today:* Runs the same app on iOS and Google Play, constantly asked which platform is
"winning." The free surface makes her establish by hand that the iOS app id and the Android
package are the same product before she can compare anything. Then compares a rank in
`Top Free → Finance → US → iPhone` against a rank in the Android Finance chart and squints,
because the ladders are not the same ladder. Watches new entrants in Finance because a new
competitor with a budget ruins her CAC — finds them by scrolling top-100 and trying to
remember which names were there last week.

*Weekly ritual:* Monday: scan the Finance top chart on both platforms for unrecognized names,
then re-check her own app's dual-platform standing.

*Frustration:* "Who is new in Finance since last week" is a set-difference question answered
with human memory against a 100-row list, twice, on two platforms.

**Persona D — Aaron, seed-stage VC analyst doing mobile diligence.**

*Today:* A founder pitches "we're the #3 app in our category and growing." He confirms #3
today and has no way to check whether that is a two-year trend or a one-week fluke, because
the dashboard shows now and only now. Knows the revenue estimate is unusable — has read the
threads where owners at $90K MRR are shown at $40K — so wants rank trajectory, release
cadence, chart position, not the money figure. On free tier because he diligences forty
companies a quarter.

*Weekly ritual:* Two or three diligence pulls a week: is this chart position durable, is the
team shipping, is the category rising or is the app rising within it.

*Frustration:* Every trajectory claim a founder makes is unfalsifiable on the free surface,
because the free surface has no past tense.

## Candidates (pre-cut)

1. **Weekly watch digest** — `watch digest` — Persona A, D — source (a) — keep
2. **Category movers / new entrants** — `movers <category>` — Persona C, B — source (a)+(b) — keep
3. **Release-vs-rank teardown** — `teardown <app-id>` — Persona B, D — source (a)+(f) — keep
4. **Free-vs-grossing divergence** — `divergence <category>` — Persona B, D — source (b) — keep
5. **Cross-platform comparison** — `compare <ios-id> <android-package>` — Persona C — source (b)+(c) — keep, gated
6. **Publisher portfolio rank weighting** — `portfolio <publisher-id>` — Persona D, B — source (c) — flagged weekly-use
7. **Primary-vs-all rank gap** — `rank-gap <app-id>` — source (b) — flagged fold-in
8. **Release cadence stats** — `cadence <app-id>` — source (b) — flagged fold-in
9. **IAP price ladder** — `iap-ladder <app-id...>` — source (b) — flagged weekly-use
10. **Rating histogram drift** — `rating-drift <app-id>` — source (c) — flagged noise
11. **Category churn heat** — `category-heat` — source (c) — flagged inferred demand
12. **Related-apps competitive set** — `related-graph --depth 2` — source (b) — **cut inline**: depth-2 traversal stalls minutes against a ~13-req budget w/ 240s recovery; depth-1 is already absorbed (#17)
13. **Rate-limit budget status** — `budget` — source (a) — **cut inline**: ELB sends no `Retry-After`/`X-RateLimit-*`, so any number is the CLI's own estimate dressed as an API fact; limiter belongs in the data layer
14. **Estimate honesty annotation** — `apps get --money` — source (b) — **cut inline**: not a feature, a global output invariant (never render 1-sig-fig buckets as precise)

## Pass 3 force-answers (per survivor)

**`movers`** — Weekly: yes, Persona C's Monday Finance sweep is the literal ritual. Wrapper:
no; `rankings ios` returns one snapshot, the new-entrant set difference is impossible from any
single call. Transcendence: local SQLite (prior snapshot) + service-specific pattern
(`previous_rank`). Sibling kill: `category-heat` — same join, inferred demand. Buildability:
`hand-code`. Long-desc validity: names `divergence`, `watch digest` — both survive.

**`teardown`** — Weekly: yes, two or three a week reactively plus Friday sweep. Wrapper: no;
`apps get` returns versions and `category history` returns ranks, but the alignment across the
two is the product. Transcendence: cross-source join (hub object × local rank history) +
agent-shaped output. Sibling kill: `cadence`, `rank-gap` — single derived fields off data
teardown already holds; fold in as columns. Buildability: `hand-code`. Long-desc validity:
names `compare`, `movers` — both survive.

**`watch digest`** — Weekly: yes by construction; it *is* the Monday ritual for A and D.
Wrapper: no; there is no watchlist concept in the API at all. Transcendence: local SQLite is
the only place last week exists. Sibling kill: `rating-drift` — same snapshot-diff mechanic,
too slow-moving for weekly signal. Buildability: `hand-code`; needs a `watchlist` table plus
`watch add`/`watch list` plumbing (Feasibility 1 not 2). Long-desc validity: names `teardown`,
`movers` — both survive.

**`divergence`** — Weekly: yes for Persona B's Friday sweep; D uses per-diligence. Wrapper: no;
requires two chart pulls joined on app_id, no such view on the free surface. Transcendence:
cross-entity local join + service-specific pattern (free/grossing split). Sibling kill:
`iap-ladder` — also monetization, but tiers move on release boundaries → a teardown moment.
Buildability: `hand-code`. Long-desc validity: names `movers` — survives.

**`compare`** — Weekly: yes for Persona C, asked "which platform is winning" on cadence.
Wrapper: no; `apps unified` returns the identity join only — the rank-ladder/cadence/histogram
comparison on top is the feature. Transcendence: cross-source join across two platforms'
mirrors. Sibling kill: `portfolio` — also a rankings join, but episodic not ritual (soft kill
Q1). Buildability: `hand-code`; Feasibility 1 (cookie path + unified identity model).
Long-desc validity: names `teardown` — survives.

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| Publisher portfolio rank weighting (`portfolio`) | Portfolio review is episodic in the brief's workflows, not a weekly ritual — soft kill on Q1. | `movers` |
| Primary-vs-all rank gap (`rank-gap`) | One derived subtraction on a field teardown already fetches; folds in as a column. | `teardown` |
| Release cadence stats (`cadence`) | Strict subset of the `versions[]` series teardown already loads; folds in as output. | `teardown` |
| IAP price ladder (`iap-ladder`) | IAP tiers change on release boundaries — a teardown-moment read, not a weekly one. | `teardown` |
| Rating histogram drift (`rating-drift`) | Rating histograms on a mature app move slowly enough that a weekly diff is mostly noise. | `watch digest` |
| Category churn heat (`category-heat`) | Persona demand inferred rather than evidenced in the brief — Research Backing 0. | `movers` |
| Related-apps competitive set (`related-graph`) | Depth-2 traversal stalls for minutes against a ~13-req budget with 240s recovery; depth-1 already absorbed (#17). | `divergence` |
| Rate-limit budget status (`budget`) | ELB emits no `Retry-After`/`X-RateLimit-*`; would print the CLI's own state estimate as an API fact. Limiter belongs in the data layer. | none (→ Build Priority 1) |
| Estimate honesty annotation (`apps get --money`) | Not a feature — never rendering 1-sig-fig buckets as precise is a global output invariant across every command. | none (global formatting rule) |
