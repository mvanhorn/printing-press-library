# Novel Features Brainstorm — slack reprint (run 20260801-150754-271fdf29)

Subagent: `general-purpose`, agentId `a527c01df74f33979`. Reprint mode — Pass 2(d) reconciliation ran against `manuscripts/slack/20260409-073625/research.json`.

> **Audit-trail gap, recorded honestly:** the subagent's first response was truncated in delivery. `## Customer model`, `### Survivors`, `### Killed candidates`, and `## Reprint verdicts` were all recovered (the first two via a targeted re-emit request). The `## Candidates (pre-cut)` section was lost and was **not** reconstructed — reconstructing it would mean inventing an audit trail. The kill list below is the surviving record of what was cut.

## Customer model

**Dana — the on-call engineer who wakes to 200 unread messages.** Comes off a 12-hour rotation, opens four pinned channels (`#incidents`, `#alerts-prod`, `#eng-platform`, a DM with the SRE lead), scrolls each hunting for messages naming her or her service. Terminal already has deploy logs open; Slack is the one tool she must leave the terminal for. *Weekly ritual:* twice-weekly shift-handoff catch-up over the last 12-24 hours, then a handoff note of what's still open. *Frustration:* separating noise from "a human asked me a question and is still waiting." The signal exists in the data — a thread whose last reply isn't hers, on a message mentioning her — and no Slack surface computes it. Slack's unread badge counts volume, not obligation.

**Marco — the terminal-resident whose own history evaporated.** Free-plan workspace. Remembers a retry-backoff decision from "sometime in the spring," searches `backoff`, gets three results all from the last six weeks. The message isn't deleted — it's past the 90-day wall. He can't request an export (admin-only, public channels only, raw JSON). So he re-asks in `#eng` and re-litigates the decision. *Weekly ritual:* searching Slack several times a week for something he knows was said, as a substitute for documentation never written. *Frustration:* results silently truncate at the retention wall with no indication a wall was hit — he cannot distinguish "nobody said this" from "Slack forgot."

**Priya — the team lead who gardens her own channels but is not an admin.** Six-person team, ~15 channels she's responsible for, regular member with her own app and token — no admin console, no `admin.*`, no export. Quarterly she's asked which channels can be archived and whether `@platform-oncall` still reflects reality; she answers by clicking the channel browser and eyeballing dates. *Weekly ritual:* a Monday sweep — which channels went quiet, where volume shifted, who stopped participating, is anyone answering in `#platform-help`. *Frustration:* every channel-level question is one paginated API call per channel, and no answers are comparable without building a spreadsheet.

**Atlas — the agent operating on the user's behalf in a Claude Code session.** Asked "what did the team decide about the migration?", it calls `conversations.list`, picks channels, calls `conversations.history` per channel, gets text full of `<@U04AB9XYZ>` and `<#C07QQ2L>` tokens it can't read, then calls `users.info` per unique ID. Every question re-pays full pagination. Reaching for `search.messages` with a bot token returns `not_allowed_token_type` — the family boundary discovered the hard way. *Weekly ritual:* answering ad-hoc human questions about Slack content many times a day, each a fresh multi-call chain with no memory between them. *Frustration:* the chain — six tool calls and an ID-resolution loop for one sentence-shaped question, when the user's stated goal this run is a *smaller* agent-facing surface.

## Survivors

| # | Feature | Command | Persona | Score | Buildability | Evidence |
|---|---|---|---|---|---|---|
| 1 | Archive Recall | `recall "<query>"` | Marco, Atlas | 10/10 | hand-code | Pain points #1/#2; prior `archive-search` scored 9/10 and was never built; no competitor has offline search. Works on a bot token where `search.*` returns `not_allowed_token_type`. |
| 2 | Catch-up | `catchup --since 24h` | Dana, Atlas | 9/10 | hand-code | Top Workflow #4; collapses a 4-command chain into one call, serving the stated smaller-surface goal. |
| 3 | Archive Coverage | `archive coverage` | Marco | 8/10 | hand-code | The mirror is only trustworthy if its coverage is inspectable; prior print shipped no way to see it. |
| 4 | Stale Thread Radar | `threads stale` | Dana, Priya | 8/10 | hand-code | Top Workflow #4; shipped April, retained. Thread grouping is a Slack-specific pattern with no API equivalent. |
| 5 | Channel Health | `health` | Priya | 7/10 | hand-code | Top Workflow #5; export/admin gating means non-admin leads have no other way to compare channels. `--dying` absorbs the retired `channels quiet`. |
| 6 | User Activity | `users activity <user>` | Priya, Dana | 6/10 | hand-code | Shipped April; serves the handoff/standup ritual. |
| 7 | Identity Card | `users whois <id\|@handle\|email>` | Dana, Atlas | 6/10 | hand-code | Every Slack payload returns opaque IDs; collapsing Atlas's per-ID resolution loop serves the smaller-surface goal. |

**All 7 are `hand-code`** — zero `spec-emits`. Post-generate scope is 7 Cobra command files plus registration wiring.

### Long Descriptions (agent-facing sibling redirects, carried into Phase 3 Cobra `Long`)

- **`recall`** — Use for messages in the local archive, including those older than Slack's 90-day wall, with thread context and resolved names. Do NOT use for a quick single-resource lookup of channels or users; use `search --type` instead.
- **`catchup`** — Use for "what happened while I was away" — new volume, mentions of you, threads awaiting your reply. Do NOT use to profile a named third party; use `users activity` instead.
- **`threads stale`** — Use to list unanswered threads across the whole archive ranked by age. Do NOT use for a personal since-I-was-away summary; use `catchup` instead.
- **`health`** — Use to score and compare channels by volume, posters, median first-reply latency, idle days; `--dying` filters archive candidates. Do NOT use for raw grouped counts over a time bucket; use `analytics --type messages --group-by channel` instead.
- **`users activity`** — Use to profile where a single named person is active. Do NOT use for your own since-I-was-away summary; use `catchup` instead.
- **`users whois`** — Use to resolve an opaque ID, @handle, or email into one identity card with shared channels, timezone, DND, last-seen. Do NOT use when you need the raw profile payload; use `users info` instead.
- **`archive coverage`** — none.

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| ID Hydration Filter (`hydrate`) | Belongs as default behavior on read commands with a `--raw-ids` escape hatch, not a standalone command nobody deliberately invokes. | `recall` |
| Usergroup Coverage (`usergroups coverage`) | Quarterly cadence, and needs channel-membership data the sync model doesn't carry. | `health` |
| Reaction Highlights (`highlights`) / prior `funny` | No persona has a ritual this serves; reaction ranking survives as a flag, not a command. | `recall --min-reactions` |
| Response Times (`response-times`) | Monthly management metric, not a weekly ritual; `health` already reports median first-reply latency as a column. | `health` |
| Activity Trends (`trends`) | Reimplements an absorbed framework command — `analytics --type messages --group-by channel`. | `analytics` (framework) |
| Quiet Channels (`channels quiet`) | Folded — a single threshold filter over the aggregate `health` already computes. | `health --dying` |
| Conversation Network (`network`) | No persona ritual; produces a graph nobody acts on weekly. | `users activity` |
| Sync Daemon (`daemon`) | Persistent background process — explicit rubric scope-creep kill; the one-command version is `sync --since 24h` under the user's own scheduler. | `sync` (framework) |
| Desktop Notifications (`notifications`) | Requires OS integration outside the spec plus a long-running process — two rubric kills at once. | `catchup` |
| Sync Budget Estimator (`sync budget`) | Output is a prediction dogfood cannot verify without running the very sync it exists to avoid. | `archive coverage` |

## Reprint verdicts

| Prior feature | Prior command | Verdict | Justification |
|---|---|---|---|
| Channel Health Report | `health` | **Keep** | Persona fit (Priya's Monday sweep), 7/10, already shipped; name reused, gains `--dying` from folded `quiet`. |
| Stale Thread Radar | `threads stale` | **Keep** | Top Workflow #4, 8/10; thread reply-ownership is Slack-specific with no API equivalent. |
| User Activity Summary | `users activity` | **Keep** | Serves handoff/standup ritual at 6/10; name reused unchanged. |
| Offline Digest | `digest` | **Reframe → `catchup`** | Right idea, wrong shape: prior was a whole-workspace window summary; the persona need is obligation-shaped and personal. **Rename — `digest` disappears.** |
| Archive Search | `archive-search` (planned, never built) | **Reframe → `recall`** | Highest prior score (9/10), direct hit on pain point #1, but a plain search wrapper duplicates absorbed `search --type messages`; reframed to add thread context, name resolution, retention-wall annotation. |
| Response Time Analytics | `response-times` | **Drop** | Monthly cadence; its one useful number is already a `health` column. |
| Channel Activity Trends | `trends` | **Drop** | Reimplements absorbed `analytics --type messages --group-by channel`. |
| Quiet Channel Detector | `channels quiet` | **Drop** | Folded into `health --dying`. |
| Funny Digest | `funny` | **Drop** | No persona ritual; scores below 5/10 on persona fit. |
| Conversation Network | `network` (never built) | **Drop** | No persona fit, no weekly action follows. |
| Sync Daemon | `daemon` (never built) | **Drop** | Persistent background process; rubric scope-creep kill. |
| Desktop Notifications | `notifications` (never built) | **Drop** | OS-level integration outside the spec plus a resident process. |
