# Google Workspace Admin CLI — Absorb Manifest

Sources absorbed from: **GAM7** (GAM-team/GAM), **GAMADV-XTD3** (taers232c), official **`gws`** CLI
(googleworkspace/cli), **GAT Labs** (GAT+/Flow/Unlock audit areas), **taylorwilsdon/google_workspace_mcp**,
**orvice/google-workspace-mcp**. Every row is a feature we MUST ship. The bulk of the absorbed API surface is
generator-emitted typed endpoint commands `(generated endpoint)`; hand-built workflows and the GAT-style audits
live in the Transcendence table.

Disposition prefixes: `(generated endpoint)` = covered by the merged spec's typed endpoint surface;
`workspace-admin-pp-cli <path>` = hand-built/promoted command (Phase 3 gate verifies it resolves);
`(behavior in workspace-admin-pp-cli <path>) ...` = flag/mode inside a command.

## Absorbed (match or beat everything that exists)

### Users (Directory)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List/get users (query, OU, fields) | GAM `print users` | (generated endpoint) users list | Offline store, `--json/--select/--csv`, SQL-joinable |
| 2 | Create user | GAM `create user` | (generated endpoint) users insert | `--dry-run`, agent-native |
| 3 | Update user (name, OU, recovery) | GAM `update user` | (generated endpoint) users update/patch | typed exit codes |
| 4 | Suspend / unsuspend | GAM `suspend user` | (behavior in workspace-admin-pp-cli users update) suspended on/off | offline status from store |
| 5 | Delete / undelete user | GAM `delete/undelete user` | (generated endpoint) users delete + undelete | dry-run guard |
| 6 | Aliases CRUD | GAM `create alias` | (generated endpoint) users aliases list/insert/delete | bulk-friendly |
| 7 | Photo get/update/delete | GAM `update photo` | (generated endpoint) users photos get/update/delete | — |
| 8 | Reset password | GAM `update user password` | (behavior in workspace-admin-pp-cli users update) password | random/specific |
| 9 | Make admin / revoke admin | GAM `create admin` | (generated endpoint) role-assignments insert/delete | offline role audit |

### Groups, Members, Group Settings, OUs, Roles (Directory + Groups Settings)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 10 | Groups CRUD | GAM `create/update/delete group` | (generated endpoint) groups list/insert/update/delete | offline store |
| 11 | Members add/remove/list | GAM `update group add member` | (generated endpoint) members list/insert/delete/patch | bulk, dry-run |
| 12 | Group settings (whocanpost/join/moderate) | GAM `update group` | (generated endpoint) groupssettings get/update | full settings audit |
| 13 | Org units CRUD + move | GAM `create/update org` | (generated endpoint) orgunits list/insert/update/delete | tree view (transcendence) |
| 14 | Roles + privileges list | GAM `print adminroles` | (generated endpoint) roles list + privileges list | offline role/priv map |
| 15 | Role assignments list/assign/revoke | GAM `print admins` | (generated endpoint) role-assignments list/insert/delete | who-has-what audit |

### Devices (Directory)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 16 | ChromeOS devices list/get | GAM `print cros` | (generated endpoint) chromeosdevices list/get | offline asset store |
| 17 | ChromeOS move OU / action (disable/deprovision) | GAM `update cros action` | (generated endpoint) chromeosdevices update/action | dry-run |
| 18 | Mobile devices list/get | GAM `print mobile` | (generated endpoint) mobiledevices list/get | offline store |
| 19 | Mobile device action (wipe/approve/block) | GAM `update mobile action` | (generated endpoint) mobiledevices action/delete | dry-run guard |

### Reports & Audit (Reports)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 20 | Activity reports (login/admin/drive/token/mobile/meet) | GAM `report <app>` | (generated endpoint) activities list | synced into store, joinable |
| 21 | Usage reports (user/customer) | GAM `report usage` | (generated endpoint) userUsageReport/customerUsageReport get | trend over store |

### Tokens / Third-party apps (Directory)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 22 | List user OAuth tokens/apps | GAM `print tokens` | (generated endpoint) tokens list/get | feeds app-risk score (transcendence) |
| 23 | Revoke token (block app for user) | GAM `delete token` | (generated endpoint) tokens delete | bulk revoke (transcendence) |

### Alert Center
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 24 | List/get alerts (filter) | GAM `print alerts` | (generated endpoint) alerts list/get | synced, joinable with users |
| 25 | Delete/undelete alert + feedback | GAM `delete alert` | (generated endpoint) alerts delete/undelete + feedback create | — |

### Data Transfer (offboarding)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 26 | List transfer apps | GAM `print transferapps` | (generated endpoint) applications list | — |
| 27 | Create/get data transfer (Drive/Calendar) | GAM `create datatransfer` | (generated endpoint) transfers insert/get/list | used by offboard workflow |

### Drive
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 28 | List/get files (query, fields) | GAM `print filelist` | (generated endpoint) drive files list/get | offline file store, SQL |
| 29 | Permissions/ACL CRUD | GAM `create drivefileacl` | (generated endpoint) drive permissions list/create/update/delete | bulk remediation (transcendence) |
| 30 | Transfer ownership | GAM `transfer ownership` | (behavior in workspace-admin-pp-cli offboard) permissions+owner | used by offboard |
| 31 | Shared drives CRUD + members | GAM `print shareddrives` | (generated endpoint) drive drives list/create/update/delete | offline store |
| 32 | About / storage quota | gws `drive about` | (generated endpoint) drive-about get | per-user quota report |

### Gmail
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 33 | Messages list/get/search | GAM `print messages` | (generated endpoint) gmail messages list/get | per-user (DWD), offline meta |
| 34 | Messages modify/trash/delete | GAM `modify messages` | (generated endpoint) gmail messages modify/trash/delete | dry-run |
| 35 | Delegation add/remove/list | GAM `add delegate` | (generated endpoint) gmail settings delegates list/create/delete | offboarding |
| 36 | Filters CRUD | GAM `create filter` | (generated endpoint) gmail settings filters list/create/delete | audit (transcendence) |
| 37 | Forwarding + sendAs + vacation | GAM `forward`/`sendas`/`vacation` | (generated endpoint) gmail settings forwarding/sendAs/vacation | offboarding + signature mgmt |
| 38 | Labels CRUD | GAM `create label` | (generated endpoint) gmail users_labels list/create/delete | — |
| 39 | IMAP/POP settings | GAM `imap`/`pop` | (generated endpoint) gmail settings getImap/updateImap/getPop/updatePop | — |

### Framework (generated baseline — every printed CLI)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 40 | Offline sync of all entities | GAT metadata scan | (behavior in workspace-admin-pp-cli sync) --resources | the GAT engine, offline |
| 41 | Full-text search over synced metadata | GAT filtered audits | (behavior in workspace-admin-pp-cli search) | offline, agent-native |
| 42 | SQL over local store | none (GAT GUI only) | (behavior in workspace-admin-pp-cli sql) | cross-API joins |
| 43 | doctor / health + auth scope check | GAM `check serviceaccount` | (behavior in workspace-admin-pp-cli doctor) | per-scope diagnostics |
| 44 | Bulk CSV ops with per-row error isolation | GAM `csv ... multiprocess` | (behavior in generated endpoint commands) --dry-run + stdin | adaptive rate limiting |

## Transcendence

Transcendence rows are produced by the Step 1.5c.5 novel-features subagent (customer model → candidates →
adversarial cut). The subagent output is appended below as `### Survivors` after Phase 1.5c.5 completes; those
rows (each with a `Buildability` tag) are the hand-code commitment surfaced at Phase Gate 1.5.

### Survivors (from Step 1.5c.5 novel-features subagent — REPRINT 2026-07-06 — 8 survivors, all hand-code)

Full brainstorm audit trail (customer model, candidates, killed list, reprint verdicts): `2026-07-06-074744-novel-features-brainstorm.md`.

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|--------------------------|------------------|
| 1 | Offboard workflow | workflow offboard | hand-code | 10 | Sequences Directory suspend/reset/sign-out/token-revoke + Drive ownership transfer + Data Transfer + Gmail delegation/forward + device wipe + group removal + OU move with per-step ledger and `--dry-run` — multi-API, not one endpoint | Use this command to execute or dry-run a departing user's lifecycle. Do NOT use it to review an account without changing it; use 'audit user360' instead. |
| 2 | User 360° | audit user360 | hand-code | 10 | Joins synced users, drive_files, drive_permissions, tokens, devices, gmail_settings, group_members, activities in local SQLite into one per-user posture report | Use this command for a current-state posture snapshot of one user. Do NOT use it for a chronological activity timeline; use 'audit reconstruct' instead. Do NOT use it to make changes on departure; use 'workflow offboard' instead. |
| 3 | External sharing audit | audit external-shares | hand-code | 10 | Local SQLite join over drive_files × drive_permissions for anyone/anyoneWithLink/external-domain shares joined to owner + owner OU | Use this command for the domain-wide external Drive exposure sweep. Do NOT use it for one user's Drive footprint; use 'audit user360' instead. |
| 4 | OAuth app risk audit | audit app-risk | hand-code | 9 | Rolls up synced tokens by app, applies a curated static scope->risk tier table (`// pp:novel-static-reference`), counts users-per-app with member list | none |
| 5 | Login / geo-failure audit | audit logins | hand-code | 9 | Anti-joins synced login activities against users for dormancy; aggregates failed-login bursts + new-country logins from the activities table | Use this command for the weekly domain-wide login-anomaly and dormant-account review. Do NOT use it for a single user's forensic timeline; use 'audit reconstruct' instead. |
| 6 | Incident reconstruction | audit reconstruct | hand-code | 9 | Merges one user's login/admin/drive/token activity rows from the store into a single ordered forensic timeline | Use this command for one user's chronological post-compromise timeline. Do NOT use it for current-state posture; use 'audit user360' instead. Do NOT use it for domain-wide anomalies; use 'audit logins' instead. |
| 7 | Email exposure sweep | audit email-exposure | hand-code | 8 | Sweeps synced gmail forwarding/sendAs/delegates/filters across all users, flagging external forwarding, external sendAs, delegates, and forward/delete filters (BEC indicators) | Use this command for the domain-wide sweep of forwarding rules, delegates, and suspicious filters. Do NOT use it for one user's full email settings; use 'audit user360' instead. |
| 8 | Nested group expansion | groups expand | hand-code | 9 | Recursive CTE over synced group_members (cycle-safe, drain-first) flattening nested groups into effective direct users — backs the x-mcp intent `group_membership_expand` | Use this command to flatten a nested group into its effective direct-user membership. Do NOT use it to list one user's group memberships; use 'audit user360' instead. |

**Hand-code commitment: 8 features** (workflow offboard, audit user360, audit external-shares, audit app-risk, audit logins, audit reconstruct, audit email-exposure, groups expand). All require SQLite joins / multi-API orchestration / custom output beyond the generator's emit path.

**Reprint delta vs prior print (7 features):** kept 6 (offboard, user360, external-shares, app-risk, logins, reconstruct); **dropped** `audit domain-graph` (Gmail edge aggregation needs domain-wide message-metadata sync the data layer does not model; Drive rollup covered by `analytics --group-by domain`); **added** 2 (`audit email-exposure` BEC sweep, `groups expand` backing the new `group_membership_expand` MCP intent). Killed this round: audit domain-graph, audit filters (merged into email-exposure), audit posture, audit drive-orphans, workflow onboard, orgunits tree, audit devices-stale, audit new-apps.

