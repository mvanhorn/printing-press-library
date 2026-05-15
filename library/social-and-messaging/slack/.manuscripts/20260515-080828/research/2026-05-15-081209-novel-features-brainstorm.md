# slack-pp-cli — Novel Features Brainstorm (reprint, run 20260515-080828)

> Output of the Phase 1.5c.5 novel-features subagent. Reprint of slack v1
> (run 20260513-191044). Prior research.json shipped with novel_features=[]
> by deliberate v1 scope trim; Pass 2(d) reconciled against the
> reconstructed prior design (prior-research-reconstructed.json).

## Customer model

Four personas, grounded in the brief's Users, Top Workflows, and User Vision.

### Persona 1 — Erick-as-Leader (weekly L10 + weekly digest)
Today: opens 6+ channels + 15 DMs in the browser three times a week (customer
sweep, L10 prep, digest), each in a different shape; the digest mis-renders
`<!subteam^S…>` as raw IDs. Weekly ritual: Mon/Tue L10 prep, Tue 9am L10,
Thu weekly digest into 2 Notion pages. Frustration: reads the same 12 channels
+ 15 DMs three times in three shapes; wants real per-report volumes for 1:1 prep.

### Persona 2 — Erick-as-CSM-Coach (intervention on a flagged customer)
Today: csm-platform-v2 fires ~30 alerts/day to #csm-signals; reconstructs a
customer's cross-source history by tab-switching Attio/Slack/Asana. Weekly
ritual: `customer-intel` across 5 channels + 4-5 CSM DMs. Frustration: wants
one screen — customer × 14d-Slack × deal-stage × open-task × call-action-items
— and to know if the CSM followed through.

### Persona 3 — Cron-as-Agent (4 production crons + 8 internal skills)
Today: 4 crons hardcode the xoxp token, call `conversations.history` per
channel, ~7,000 calls/week, no Retry-After, no DM-read audit. Frustration:
May-2025 cap would silently truncate; no audit of which agent read which DM.

### Persona 4 — Marjorie / Adrian (CSM teammates DM'd FROM Erick by the agent)
Low-confidence persona; narrative-trace system of record is csm-platform-v2,
not pp-slack. Justifies at most one candidate (cut).

## Candidates (pre-cut)

16 candidates. Reprint reconciliation (d) re-scored the prior 8 novel_features;
persona-driven (a/b) and cross-entity (c) added 8 fresh candidates; user
briefing (e) yielded zero net-new (all 12 User Vision verbs are in the absorb
manifest's Absorbed table).

Reprint (d): C1 customer-intel-deep 9/10 prior-keep; C2 dm-engagement 8/10
prior-reframe; C3 action-followthrough 8/10 prior-keep; C4 reactions summarize
7/10 prior-reframe; C5 unreads 7/10 prior-keep; C6 usergroups list 7/10
prior-keep; C7 goal-channel-pulse 6/10 prior-reframe; C8 agent-audit 6/10
prior-keep.

Fresh (a/b/c): C9 dm-quote (kill), C10 permalink-resolve (kill), C11
reaction-impact (kill — subsumed by C4), C12 canvas (kill), C13 huddle-snippets
(kill), C14 escalation-path (kill — subsumed by C1), C15 alert-trace (kill —
csm-platform-v2 reimplementation), C16 truncation-guard (kill — absorbed as
pagination behavior).

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability proof |
|---|---------|---------|-------|---------|--------------------|
| 1 | Customer Intel Deep | `customer-intel-deep "Sonria" --window 14d` | 9/10 | P2 | pp-slack `messages` FTS5 + `ATTACH DATABASE` to pp-attio/pp-asana/pp-fathom SQLite; deterministic per-company timeline, every line cited. |
| 2 | DM Engagement | `dm-engagement --report all --window 14d` | 8/10 | P1 | 3-mirror join: pp-slack `messages WHERE is_im` + pp-asana `tasks.assignee_id` + pp-fathom `call_participants`; one volume row per report. |
| 3 | Action Follow-through | `action-followthrough --report marjorie --window 14d` | 8/10 | P2 | pp-fathom `action_items` ⨯ pp-slack FTS5 joined on assignee + time window; binary `followed_up` + permalink. |
| 4 | Reactions Summarize | `reactions summarize --channel "#the-wolf-of-atom" --window 7d` | 7/10 | P1 | Local `reactions JOIN messages` GROUP BY emoji; top messages + fixed emoji-class buckets (no NLP). |
| 5 | Unreads Priority | `unreads --priority` | 7/10 | P1 | xoxp branch: `messages.ts` vs `sync_state.last_read` per channel; DM>partner>internal>broadcast buckets. xoxc/client.counts gated to v1.1. |
| 6 | Usergroups Render | `usergroups list` + emit-time substitution | 7/10 | P1+P3 | Local `usergroups` table; pre-emit regex `<!subteam^S…>` → `@handle`. Fixes the digest mis-render bug. |
| 7 | Goal Channel Pulse | `goal-channel-pulse --quarter current` | 6/10 | P1 | pp-asana `goals` ⨯ pp-slack `messages` filtered by explicit `slack_channel:` Rock YAML field; per-Rock 7d volume + stalled flag. |
| 8 | Agent Audit | `agent-audit --window 7d` | 6/10 | P3 | `SELECT FROM audit_log JOIN channels` — pure local-mirror read of pp-slack's own mandatory audit table. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C9 dm-quote | Thin wrapper over `chat.getPermalink`; redaction is an absorbed flag | customer-intel-deep |
| C10 permalink-resolve | Thin wrapper — regex + one mirror lookup | customer-intel-deep |
| C11 reaction-impact | Subsumed by `reactions summarize --user me` | reactions summarize |
| C12 canvas | Slack Canvas spec coverage unconfirmed; no weekly persona | pp-notion |
| C13 huddle-snippets | `calls.*` marked not-relevant for AtomChat | none |
| C14 escalation-path | Subsumed by absorbed customer-intel + customer-intel-deep | customer-intel-deep |
| C15 alert-trace | csm-platform-v2 is system of record — reimplementation kill | customer-intel-deep |
| C16 truncation-guard | Ambient pagination correctness, already absorbed | conversations history / sync |

## Reprint verdicts

All 8 prior novel_features survive — 5 keep, 3 reframe, 0 drop. Personas
unchanged from v1; the prior design was already a tight adversarial cut.

| Prior feature | Verdict | Justification |
|---------------|---------|---------------|
| Customer Intel Deep | Keep | Re-scores 9/10 vs P2; command unchanged. |
| DM Engagement | Reframe | "Heatmap" overstates a volume table — keep command, drop heatmap framing. |
| Action Follow-through | Keep | Re-scores 8/10 vs P2; command unchanged. |
| Reactions Summarize | Reframe | Scope narrowed — "sentiment bucket" → mechanical fixed emoji-class counts (clears LLM-dependency check). |
| Unreads Priority | Keep | xoxp mirror-scan branch ships; xoxc/client.counts gated to v1.1. |
| Usergroups Render | Keep | Bug-fix-mandated by brief §Data Layer line 38. |
| Goal Channel Pulse | Reframe | Requires explicit `slack_channel:` Rock YAML field — no title-heuristic. |
| Agent Audit | Keep | Policy-mandated by brief §Privacy Posture line 58. |
