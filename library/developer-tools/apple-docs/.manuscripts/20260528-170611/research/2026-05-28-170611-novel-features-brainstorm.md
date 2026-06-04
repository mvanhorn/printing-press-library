# Novel-Features Brainstorm — apple-docs (full audit trail)

## Customer model

**Persona 1 — Mira, the AI coding agent operator**
Mira runs a Claude/Cursor-based pipeline that authors Swift code for client iOS apps. The agent calls out to documentation tools dozens of times per feature: confirming a SwiftUI modifier's iOS-version floor before emitting it, checking which initializer a deprecated symbol points to, pulling sample-code links to ground an answer.

- **Today:** Hops between an MCP server, a Markdown proxy, and four browser tabs. When a build error mentions an API her training data doesn't cover, she manually fetches the doc page, pastes it into the agent's context, and re-runs.
- **Weekly ritual:** Every Monday she regenerates a "WWDC-since-last-month" digest of new/changed APIs. Ad-hoc symbol lookups hundreds of times across the week.
- **Frustration:** Token bloat. Every doc fetch is a 50KB+ JSON dump where the agent only needed `abstract + declaration + iOS introducedAt`.

**Persona 2 — Devon, the visionOS engineer porting an iPad app**
Devon is shipping a visionOS 2 build of a production iPad app. Half his calendar is "find every UIKit/AppKit API that has no visionOS equivalent and figure out what replaces it."

- **Today:** Opens each suspicious symbol's doc page, eyeballs the platforms table, clicks See Also when no visionOS row exists, hopes the alternative isn't itself deprecated.
- **Weekly ritual:** Audits a slice of his app's symbol surface against visionOS availability.
- **Frustration:** There is no way to ask "give me every UIKit symbol used in my project that isn't available on visionOS, with its closest SwiftUI replacement."

**Persona 3 — Priya, the deprecation-tracking technical writer**
Priya writes the "What's deprecated in iOS N" guides that get cited in industry newsletters. Every June she has to produce the canonical list within 72 hours of the WWDC keynote.

- **Today:** Scrapes developer.apple.com with ad-hoc Python, parses `metadata.platforms[].deprecatedAt` by hand. The scraper breaks every June.
- **Weekly ritual:** Pulls a fresh framework index, diffs against last week's snapshot.
- **Frustration:** Diffing two snapshots of a 1MB framework index by hand. Knowing whether a symbol was *renamed* vs *removed* vs *moved between modules* requires reading both pages side-by-side.

## Candidates (pre-cut)

(See subagent response. 16 candidates generated across persona-driven, service-specific, and cross-entity sources.)

## Survivors and kills

### Survivors (8)

| # | Feature | Command | Score | Buildability | Long Description |
|---|---------|---------|-------|--------------|------------------|
| 1 | Token-lean doc projection | `doc get <path> --shape <abstract\|signature\|platforms\|min>` | 9/10 | hand-code | Use this command when an agent needs the minimal viable doc payload. Do NOT use it for the full rendered doc page; use `doc get` (default) or `bundle`. |
| 2 | Cross-platform replacement finder | `port-to <platform> <symbol>` | 8/10 | hand-code | Use to find the API that replaces an unavailable symbol on a target platform. Do NOT use for general similar-API lookup; use `doc similar`. Do NOT use to enumerate all deprecated symbols; use `deprecation-cliff`. |
| 3 | Snapshot diff | `snapshot diff <framework> --from <date> --to <date>` | 9/10 | hand-code | Use for added/removed/deprecated/renamed delta between two saved snapshots. Do NOT use for "what's new since last sync"; use `updates`. Do NOT use for single-version cliff; use `deprecation-cliff`. |
| 4 | Deprecation cliff report | `deprecation-cliff --os <platform> --version <N>` | 9/10 | hand-code | Use for "every symbol Apple deprecated in version N of platform X". Do NOT use for diffing two snapshots; use `snapshot diff`. Do NOT use for per-symbol replacement; use `port-to`. |
| 5 | Conformance graph | `conformance <protocol>` | 7/10 | hand-code | none |
| 6 | Cross-framework symbol grep | `grep <pattern> --kind <kind> --has-platform <platform> --deprecated` | 8/10 | hand-code | Use to find symbols across every synced framework with kind/platform/deprecation filters. Do NOT use for keyword search of titles/abstracts; use `search`. |
| 7 | WWDC ↔ symbol reverse index | `wwdc symbols <session-id>` | 7/10 | hand-code | Use to enumerate every symbol whose doc page cites a WWDC session. Do NOT use for keyword search across WWDC titles; use `wwdc search`. |
| 8 | Agent-shape doc bundle | `bundle <symbol> --depth 1 --max-tokens 4000` | 8/10 | hand-code | Use for a self-contained Markdown context blob for an agent prompt. Do NOT use for a single doc page; use `doc get --markdown`. Do NOT use for sample-code projects; use `sample-code list`. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Bulk project audit | Scope creep — Swift source scanning is a separate domain (~300+ LoC syntactic parser). | `port-to` |
| WWDC digest since N | Redundant — `updates` + `wwdc symbols` cover the same workflow. | `wwdc symbols` + absorbed `updates` |
| Sample-code grep | Wrapper — once cached, `search --type sample-code` covers it. | absorbed `sample-code list` + `grep` |
| Platform availability matrix | Soft-kill on weekly use — quarterly artifact. | `deprecation-cliff` + absorbed `doc platforms` |
| Macro adoption mining | Borderline domain fit + speculative weekly use. Score 4/10. | `grep --kind macro` |
| Framework coverage stats | Monthly use, not weekly. Score 4/10. | `deprecation-cliff` + `grep` |
| Renamed-symbol tracker | Sibling subsumption — collapsed into `snapshot diff --classify`. | `snapshot diff` |
| Replacements | Sibling subsumption — collapsed into `port-to`. | `port-to` |
