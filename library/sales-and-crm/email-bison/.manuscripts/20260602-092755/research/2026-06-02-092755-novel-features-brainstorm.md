# Email Bison CLI — Novel Features Brainstorm (audit trail)

## Customer model

**Persona A — Maya, the agency campaign operator running 12 client workspaces.**
Lives in the campaign-build flow: create -> settings -> schedule -> sequence steps -> attach senders -> attach leads -> resume.
- Today: clicks through the UI workspace by workspace or hand-writes curl. To answer "which of my 40 campaigns across 12 workspaces are launched but sending below their daily cap," she opens each workspace and eyeballs it.
- Weekly ritual: Monday launch sweep — 3-5 new campaigns, copy schedule between clients, confirm each is pushing volume.
- Frustration: no single view across workspaces; cap headroom, "is this actually live," and schedule drift are invisible without manual clicking.

**Persona B — Dev, the deliverability manager babysitting ~400 sender inboxes.**
Bulk-connects SMTP/IMAP, tags accounts, patches settings when accounts degrade.
- Today: lists senders per workspace, scans tags by eye; bounce signals live inside replies, no consolidated board.
- Weekly ritual: health audit — find dead/disconnected senders, rebalance attachments, spot ESP concentration.
- Frustration: sender state, tag state, and campaign attachment are three separate surfaces; nothing joins "sender X is disconnected AND still on 3 live campaigns."

**Persona C — Priya, the lead-gen VA triaging the master inbox each morning.**
Filters replies, marks interested, replies in-thread, pushes interested replies to follow-up campaigns.
- Today: opens master inbox per workspace, filters manually, one workspace at a time.
- Weekly ritual: daily AM triage + weekly stale-lead sweep.
- Frustration: no cross-campaign "interested since yesterday" roll-up; leads stuck mid-sequence rot invisibly.

## Survivors (transcendence, >=5/10)

| # | Feature | Command | Score | Persona |
|---|---------|---------|-------|---------|
| 1 | Campaign cap headroom | `campaigns headroom` | 8 | Maya |
| 2 | Sender health board | `senders health` | 8 | Dev |
| 3 | Interested-reply roll-up | `replies interested --since` | 7 | Priya |
| 4 | Stale-lead detector | `leads stale --days N` | 7 | Priya |
| 5 | Sequence-variant win rates | `campaigns variants <id>` | 6 | Maya |
| 6 | Launch readiness preflight | `campaigns preflight <id>` | 7 | Maya |
| 7 | Reply triage queue | `replies triage` | 6 | Priya |

## Killed candidates

| Feature | Kill reason | Sibling |
|---------|-------------|---------|
| Bounce clustering | bounces-as-reply-rows unverified; feeds C2 | senders health |
| ESP-mismatch | niche/monthly, single-source evidence | senders health |
| Cross-workspace status | needs multi-workspace sync data-model addition | campaigns headroom |
| Schedule-template diff | one-off, subsumed by preflight | campaigns preflight |
| Lead dedupe | thin evidence, monthly hygiene | leads stale |
| Custom-variable coverage | sibling of preflight merge-tag check | campaigns preflight |
| Daily activity digest | composite re-query of C1-C4 | replies triage + interested |
| Workspace inventory | sibling aggregate, folded with cross-workspace | campaigns headroom |
| Sent-volume trend | reporting nicety, not a decision | campaigns headroom |
