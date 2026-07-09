# Novel Features Brainstorm — workspace-admin (reprint 2026-07-06)

## Customer model

**Marta — the education-IT super admin at a virtual school (example.org).**
- **Today (without this CLI):** She keeps a folder of GAM one-liners for semester churn, three Admin Console tabs pinned (Users, Org Units, Devices), and a Google Sheet titled "Offboarding checklist v7" with eight manual steps per departure. When a teacher leaves mid-semester she runs suspend, password reset, token revoke, Drive transfer, delegation, group removal, and OU move as separate GAM commands, checking each off by hand. She cannot answer "did every step complete for the last five departures" without scrolling terminal history.
- **Weekly ritual:** Friday departures and role changes — every week during the school year at least one staff or contractor account is offboarded, and cohort transitions at semester boundaries multiply this by dozens.
- **Frustration:** No single command runs the whole offboard, no record of what completed, and a skipped step (an un-revoked token, an un-transferred Drive) surfaces weeks later as an incident.

**Dev — the delegated security auditor / compliance lead.**
- **Today (without this CLI):** A GAT Labs trial tab, the Admin Console Reports section, and a growing pile of Sheets exports. To answer "which files are shared anyone-with-link, who owns them, and which OU are those owners in," he has to script per-file permission listing across the domain — the Drive API simply cannot ask the domain-wide question. Third-party app review means eyeballing raw OAuth scope strings with no risk tiering.
- **Weekly ritual:** The Monday-morning exposure sweep — external Drive shares, newly authorized OAuth apps, 2SV laggards, dormant accounts — assembled into a report for the compliance file.
- **Frustration:** Every question that joins two Google APIs (file → owner → OU → last login) is impossible without persisted data; stateless tools like GAM and gws make him re-pull everything every time.

**Priya — the security-ops admin and agent operator.**
- **Today (without this CLI):** She drives Claude against MCP tools for triage, but for a suspected compromise she falls back to raw Reports API queries in four tabs — login, admin, drive, token activity — and stitches a timeline by hand into a doc. Checking whether the attacker planted forwarding rules or auto-delete filters means one Gmail settings call per user in the blast radius.
- **Weekly ritual:** Alert Center triage plus the BEC-indicator sweep: new external forwarding, suspicious delegates, filters that skip-inbox-and-delete.
- **Frustration:** Cross-stream timeline reconstruction is manual, and domain-wide Gmail-settings sweeps are N impersonated API calls with nowhere to persist the answers.

## Candidates (pre-cut)

| # | Candidate | Command | One-liner | Persona | Source | Kill/keep inline verdict | Long Description |
|---|-----------|---------|-----------|---------|--------|--------------------------|------------------|
| 1 | Offboard workflow | `workflow offboard` | Full departing-user lifecycle in one command with per-step ledger and `--dry-run` | Marta | (d) prior-keep + (a) | Keep — brief workflow #1, mechanical, same auth | redirect read-only intent to `audit user360` |
| 2 | User 360 audit | `audit user360` | One per-user posture report joining Drive, security, apps, devices, Gmail settings, groups, activity | Priya, Dev | (d) prior-keep + (e) x-mcp `user_security_snapshot` | Keep — spec-declared MCP intent this reprint | redirect timeline to `audit reconstruct`, mutation to `workflow offboard` |
| 3 | External sharing audit | `audit external-shares` | Every file shared anyone-with-link or externally, joined to owner and owner's OU | Dev | (d) prior-keep + (c) | Keep — brief workflow #2; local join Drive API can't do | prior Long referenced `audit domain-graph`; revalidated in Pass 3 |
| 4 | OAuth app risk audit | `audit app-risk` | Tier every third-party app Low/Mod/High by curated scope-risk table, users-per-app | Dev | (d) prior-keep | Keep — static reference table `// pp:novel-static-reference` | none |
| 5 | Login and geo-failure audit | `audit logins` | Failed-login bursts, new-country logins, dormant accounts from synced activity joined to users | Dev, Priya | (d) prior-keep + (c) | Keep — "no login since N days" is store-only | redirect single-user forensics to `audit reconstruct` |
| 6 | Incident reconstruction | `audit reconstruct` | One user's login/admin/drive/token activity merged into a single ordered timeline | Priya | (d) prior-keep + (a) | Keep — mechanical interleave, no LLM | redirect posture to `audit user360`, domain-wide to `audit logins` |
| 7 | Domain connection graph | `audit domain-graph` | Per-external-domain edge list from Drive permissions + Gmail sender/receiver pairs | Dev | (d) prior-drop candidate | Soft kill — Gmail edges need domain-wide message-metadata sync not in the model | none |
| 8 | Forwarding/delegation sweep | `audit forwarding` | All users with external forwarding, external sendAs, or delegates | Priya | (b) Gmail DWD + (c) | Keep-lean — merge with #9 | — |
| 9 | Malicious filter sweep | `audit filters` | Filters that auto-forward, auto-delete, or skip inbox, across all users | Priya | (b) + (c) | Merge into #8 as `audit email-exposure` | — |
| 10 | Nested group expansion | `groups expand` | Recursively flatten nested groups into effective direct-user membership | Marta, agents | (e) x-mcp `group_membership_expand` + (b) | Keep — recursive CTE over store | redirect per-user membership to `audit user360` |
| 11 | Domain posture scorecard | `audit posture` | One-shot domain scorecard: 2SV %, dormant count, external-share count, high-risk apps | Dev | (a) | Soft kill — dashboard aggregation of #3/#4/#5 | none |
| 12 | Orphaned-file audit | `audit drive-orphans` | Files owned by suspended/deleted users | Marta | (c) | Kill — monthly; one `sql` query | none |
| 13 | Onboard workflow | `workflow onboard` | Create user, set OU, add groups from template | Marta | (a) | Kill — absorbed by framework CSV ops | none |
| 14 | OU tree view | `orgunits tree` | Render OU hierarchy with user/device counts | Marta | (b) | Kill — occasional; thin view | none |
| 15 | Stale device audit | `audit devices-stale` | ChromeOS devices with no sync in N days, by OU | Marta | (b) | Kill — monthly; `search`/`sql` covers it | none |
| 16 | New-apps delta | `audit new-apps` | OAuth apps newly authorized since last sync | Dev | (c) | Kill — requires snapshot history (feasibility 0) | none |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Source | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|--------|--------------|--------------|----------|------------------|
| 1 | Offboard workflow | `workflow offboard` | 10/10 | Marta | prior (kept) | hand-code | Sequences Directory suspend/password/signOut, tokens delete, Drive permission transfer + Data Transfer insert, Gmail delegation/forwarding, device wipe, members delete, OU move with per-step ledger and `--dry-run`. | Brief Top Workflow #1; GAM requires hand-maintained checklist | Use this command to execute or dry-run a departing user's lifecycle. Do NOT use it to review an account without changing it; use 'audit user360' instead. |
| 2 | User 360 audit | `audit user360` | 10/10 | Priya, Dev | prior (kept) | hand-code | Joins synced users, drive_files, tokens, devices, gmail settings, group_members, activities in local SQLite into one posture report. | Brief #5; x-mcp intent `user_security_snapshot`; GAT paid-GUI only | Use this command for a current-state posture snapshot of one user. Do NOT use it for a chronological activity timeline; use 'audit reconstruct' instead. Do NOT use it to make changes on departure; use 'workflow offboard' instead. |
| 3 | External sharing audit | `audit external-shares` | 10/10 | Dev | prior (kept) | hand-code | Local join over synced drive_files × drive_permissions filtered to anyone/anyoneWithLink/external-domain, joined to users for owner OU. | Brief #2; GAT+ flagship; Drive API can't answer domain-wide | Use this command for the domain-wide external Drive exposure sweep. Do NOT use it for one user's Drive footprint; use 'audit user360' instead. |
| 4 | OAuth app risk audit | `audit app-risk` | 9/10 | Dev | prior (kept) | hand-code | Rolls up synced tokens by clientId, maps scopes to a curated static risk-tier table (`// pp:novel-static-reference`), counts users per app. | Brief #3; Directory returns raw scopes; GAT parity | none |
| 5 | Login and geo-failure audit | `audit logins` | 9/10 | Dev, Priya | prior (kept) | hand-code | Anti-joins synced login activities against users for dormancy, aggregates failures/new-geo from the activities table. | Brief #4; Reports API can't express "no login since N days" | Use this command for the weekly domain-wide login-anomaly and dormant-account review. Do NOT use it for a single user's forensic timeline; use 'audit reconstruct' instead. |
| 6 | Incident reconstruction | `audit reconstruct` | 9/10 | Priya | prior (kept) | hand-code | Merges one user's synced login/admin/drive/token activity into a single timestamp-ordered timeline from the local store. | Brief #4; no single Reports call produces a merged stream | Use this command for one user's chronological post-compromise timeline. Do NOT use it for current-state posture; use 'audit user360' instead. Do NOT use it for domain-wide anomalies; use 'audit logins' instead. |
| 7 | Email exposure sweep | `audit email-exposure` | 8/10 | Priya | new | hand-code | Sweeps synced gmail forwarding/sendAs/delegates/filters across all users, flagging external forwarding, external sendAs, delegates, and forward/delete filters. | Data layer syncs gmail settings; forwarding/auto-delete are standard BEC indicators (GAT Email audit area) | Use this command for the domain-wide sweep of forwarding rules, delegates, and suspicious filters. Do NOT use it for one user's full email settings; use 'audit user360' instead. |
| 8 | Nested group expansion | `groups expand` | 9/10 | Marta, agents | new | hand-code | Recursive CTE over synced group_members (cycle-safe, drain-first) flattening nested groups into effective direct users. | x-mcp intent `group_membership_expand`; Directory members.list doesn't recurse | Use this command to flatten a nested group into its effective direct-user membership. Do NOT use it to list one user's group memberships; use 'audit user360' instead. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|------------|---------------------------|
| `audit domain-graph` (prior) | Gmail sender/receiver edges require domain-wide per-user message-metadata sync not modeled; Drive half reduces to `analytics --type drive_permissions --group-by domain`; quarterly cadence. | `audit external-shares` |
| `audit filters` | Merged into `audit email-exposure`. | `audit email-exposure` |
| `audit posture` | Dashboard aggregation of three surviving audits; `analytics --group-by` covers counts. | `audit user360` |
| `audit drive-orphans` | Monthly cadence; one `sql` join over drive_files + users.suspended. | `audit external-shares` |
| `workflow onboard` | Bulk provisioning absorbed by framework CSV ops with `--dry-run`. | `workflow offboard` |
| `orgunits tree` | Occasional browse view over one table. | `groups expand` |
| `audit devices-stale` | Monthly fleet hygiene; `search`/`sql` over chromeos_devices covers it. | `audit logins` |
| `audit new-apps` | Delta detection needs snapshot-history infra the store lacks (feasibility 0). | `audit app-risk` |

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---------------|---------|---------------|
| `workflow offboard` | **keep** | 10/10 against Marta; brief workflow #1 unchanged; command reused verbatim. |
| `audit user360` | **keep** | 10/10; now backed by accepted x-mcp intent `user_security_snapshot`; command reused. |
| `audit external-shares` | **keep** | 10/10; Dev's weekly sweep; `Long` rewritten (prior cross-referenced dropped `audit domain-graph`). |
| `audit app-risk` | **keep** | 9/10; curated static reference valid; command reused. |
| `audit logins` | **keep** | 9/10; dormancy anti-join is store-only; command reused. |
| `audit reconstruct` | **keep** | 9/10; Priya's escalation timeline; cross-references survive. |
| `audit domain-graph` | **drop** | Gmail edge aggregation needs domain-wide message-metadata sync the data layer does not model; cadence fails weekly-use; Drive rollup covered by `analytics --group-by domain`. User may override at the Phase 1.5 gate.
