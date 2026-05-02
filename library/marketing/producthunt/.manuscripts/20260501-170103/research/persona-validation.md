# Persona-First Validation of Novel Features

## Personas (user-named)

### Persona A: Indie Founder Launching This Week / Next Week
- **Rituals:** browser-tab-refresh on launch day, scroll comments looking for real questions, eyeball the leaderboard to compare standing
- **Pre-launch frustrations:** "What's a 'good' launch in my category look like at hour 6? Hour 12? End of day? I have no benchmarks." / "When are competitors launching this week — should I shift dates?"
- **Day-of frustrations:** "I keep refreshing the leaderboard. I miss real customer questions in the comment flood. I can't tell if I'm catching up to the leader or falling behind."
- **Post-launch frustrations:** "Did I do well relative to the day's cohort? What do retro stats look like vs the top 5?"

### Persona B: Marketing / Competitive Research
- **Rituals:** weekly "what launched in our category" sweep, monthly category trend report, on-demand competitor launch deep-dives, slide-deck-ready snapshots
- **Frustrations:** "PH UI is for browsing, not analysis. Search by tagline keyword is non-existent. Historical trends require scraping."
- **Wants:** brand-mention tracker, look-alike competitor finder, trend deltas vs prior month, calendar of when launches happen

## Re-scoring my original 11 novel features against these personas

| # | Original feature | Founder | Marketer | Verdict |
|---|---|---|---|---|
| T1 | `posts trajectory <slug>` | 5/5 launch-day must | 3/5 track competitor's curve | **KEEP** (foundation for N1, N2) |
| T2 | `topics momentum --since 7d` | 3/5 pre-launch slot pick | 5/5 trend reports | **RESHAPE → category snapshot** (subsumes) |
| T3 | `posts compare slug1 slug2 ...` | 4/5 benchmarking | 5/5 competitive set | **KEEP** |
| T4 | `auth onboard` | onboarding (not feature) | onboarding | **KEEP** (folded) |
| T5 | `doctor` auth-stage-aware | diagnostic | diagnostic | **KEEP** (folded) |
| T6 | `posts comments-digest <slug>` (generic FTS) | 5/5 launch-day triage | 4/5 comment mining | **RESHAPE → comments-questions** (sharper) |
| T7 | `posts since 6h` | 2/5 agent feature | 3/5 quick check | **KEEP**, mark agent-native |
| T8 | `topics watch <slug> --min-votes 200` | 2/5 pre-launch competitor monitor | 4/5 daily monitoring | **KEEP** but down-weight |
| T9 | `collections outbound-diff --since 7d` | 1/5 editorial niche | 1/5 editorial niche | **DROP** — neither persona needs it. "What can SQLite do" smell. |
| T10 | `context --topic ai --json` | 1/5 agent only | 1/5 agent only | **KEEP** but mark agent-native |
| T11 | `whoami` with budget | 2/5 diagnostic | 2/5 diagnostic | **KEEP** (folded) |
| T12 | RSS-tier upgrade hint | onboarding | onboarding | **KEEP** (folded) |

## NEW persona-driven features (the ones I missed)

| # | Feature | Founder | Marketer | Score |
|---|---------|---------|---------|-------|
| N1 | `posts launch-day <my-slug>` — your launch + today's top 5, trajectories side-by-side | 5/5 | 4/5 | **9/10** — replaces refreshing the leaderboard |
| N2 | `posts benchmark --topic <slug>` — percentile curves from accumulated history (hour-6 / hour-12 / end-of-day for top-10 / top-50) | 5/5 | 4/5 | **9/10** — sets realistic targets, "is hour 6 with 84 votes good?" |
| N3 | `posts questions <slug>` — comments that look like genuine questions vs cheerleading (regex + heuristic) | 5/5 | 3/5 | **8/10** — launch-day MUST, sharper than generic digest |
| N4 | `category snapshot --topic <slug> --window weekly\|monthly` — slide-deck brief: leaderboard + momentum delta + top topics | 3/5 | 5/5 | **9/10** — direct marketing artifact |
| N5 | `posts grep --term "claude" --since 7d` — find launches mentioning a term in tagline/description | 2/5 | 5/5 | **8/10** — brand-mention tracker, marketer staple |
| N6 | `posts lookalike <slug>` — find prior launches in same topic with overlapping tagline tokens | 4/5 | 5/5 | **8/10** — competitive set discovery |
| N7 | `launches calendar --topic <slug> --week WNN` — what launched what day, with hour-of-day | 5/5 pre-launch | 4/5 | **8/10** — slot picking, "is Tuesday a slow day in this topic" |

## Final persona-validated novel-feature list

**Founder-launch-day cluster (5 commands, all 8/10+):**
1. `posts launch-day <my-slug>` (NEW, 9/10) — your launch vs today's top 5, side-by-side trajectory
2. `posts benchmark --topic <slug>` (NEW, 9/10) — percentile curves from local history
3. `posts trajectory <slug>` (KEPT, 9/10) — single-launch votes-over-time
4. `posts questions <slug>` (RESHAPED from comments-digest, 8/10) — surface genuine Q&A
5. `posts compare slug1 slug2 ...` (KEPT, 8/10) — column-aligned comparison

**Marketer-research cluster (4 commands, all 8/10+):**
6. `category snapshot --topic <slug> --window weekly` (RESHAPED from topics momentum, 9/10) — slide-deck brief
7. `posts grep --term "X" --since 7d` (NEW, 8/10) — keyword/brand mention across launches
8. `posts lookalike <slug>` (NEW, 8/10) — competitive-set discovery
9. `launches calendar --topic <slug> --week W18` (NEW, 8/10) — slot-picking calendar

**Cross-persona / monitoring (1 command):**
10. `topics watch <slug> --min-votes 200` (KEPT, 7/10 — down-weighted for founder fit, kept for marketer scheduled-jobs)

**Agent-native (2 commands):**
11. `posts since 6h` (KEPT, 7/10) — local-first time-window
12. `context --topic <slug> --json` (KEPT, 7/10) — single-call agent snapshot

**Folded (4 enhancements to existing commands):**
- `feed` no-auth tier with upgrade hint
- `whoami` with rate-limit budget
- `auth onboard` (dev-token primary, --oauth alternate)
- `doctor` auth-stage-aware

**DROPPED:**
- `collections outbound-diff` — neither persona requested it; "what can SQLite do" feature
- `comments-digest` generic FTS — replaced by sharper `posts questions` aimed at the actual launch-day use case
- `topics momentum` — subsumed by `category snapshot`

## Net change
- 11 transcendence commands → 12 standalone novel commands (5 founder + 4 marketer + 1 monitoring + 2 agent-native)
- Plus 4 folded enhancements (feed, whoami, auth onboard, doctor)
- 0 stubs
- Every novel command now answers a specific persona ritual or frustration, not just "the data shape allows it"

## Bar to beat
None of the 12 novel commands exist in any competing tool (jaipandya MCP, abandoned CLIs, npm SDKs).
