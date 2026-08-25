## Absorbed (match or beat everything that exists)

Source for all rows below: Bonusly's official MCP server (`https://bonus.ly/mcp`, ~66 tools total), whose tool docs each name the REST v1 path they mirror. This is the only real competitor with meaningful feature breadth — third-party tooling is a dead Go CLI (0 stars, 2022), a 1-star Python MCP wrapper (3 endpoints: list/create/get bonuses), a 2015 Node client, and a 2020 Hubot integration. Zero Claude Code skills/plugins found. Bonusly's own Zapier integration is deprecated.

**Scope note:** the confirmed user is a regular (non-admin) employee. ~28 of the official tools require `*:administer` scopes that a personal-automation PAT cannot carry (confirmed via three independent Bonusly help-center docs). Those are listed separately below as explicitly out-of-scope, not silently dropped.

### Identity & org chart (`user:read` — all in scope)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | My own profile | MCP `me` | bonusly-pp-cli users me | Offline cache, --json |
| 2 | Resolve a user by id/email/name | MCP `getUser` | bonusly-pp-cli users get | Local fuzzy match against synced roster first, falls back live |
| 3 | Bulk user fetch | MCP `getUsers` | (generated endpoint) users get-bulk | Batches beyond MCP's per-call cap via local cache |
| 4 | Search users | MCP `searchUsers` | bonusly-pp-cli users search | Offline FTS over synced roster |
| 5 | Company metadata | MCP `getCompany` | bonusly-pp-cli company | Cached, --json (single-endpoint resource, auto-promoted bare) |
| 6 | List departments w/ headcount | MCP `listDepartments` | bonusly-pp-cli departments list | Headcount denominator feeds local participation math (see transcendence) |
| 7 | List locations w/ headcount | MCP `listLocations` | bonusly-pp-cli locations list | Same as above |
| 8 | Users in a department | MCP `listUsersInDepartment` | (generated endpoint) departments users | — |
| 9 | Users in a location | MCP `listUsersInLocation` | (generated endpoint) locations users | — |
| 10 | Top-level (no-manager) users | MCP `listTopLevelUsers` | bonusly-pp-cli org top | Entry point for local org-tree walk |
| 11 | Direct reports of a manager | MCP `getDirectReports` | bonusly-pp-cli org reports | Synced locally, joinable to recognition history |
| 12 | Manager chain upward | MCP `getManagerChain` | bonusly-pp-cli org chain | — |
| 13 | Reporting tree downward | MCP `getReportingTree` | bonusly-pp-cli org tree | — |
| 14 | List system user groups | MCP `listSystemUserGroups` | (generated endpoint) groups list-system | — |
| 15 | Get system user group members | MCP `getSystemUserGroup` | (generated endpoint) groups get-system | — |
| 16 | List custom user groups | MCP `listCustomUserGroups` | (generated endpoint) groups list-custom | — |
| 17 | Get custom user group members | MCP `getCustomUserGroup` | (generated endpoint) groups get-custom | — |

### 1:1 meetings (`checkins:read`/`growth:read` — self-scoped, in scope)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 18 | List my 1:1 meetings | MCP `listMeetings` | bonusly-pp-cli meetings list | Local cache, date-range filter |
| 19 | Get a meeting | MCP `getMeeting` | (generated endpoint) meetings get | — |
| 20 | Get a meeting transcript | MCP `getMeetingTranscript` | (generated endpoint) meetings transcript | — |

### Points & balance (`user:read` — in scope)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 21 | Points balance + lifetime stats | MCP `getPointsBalance` | bonusly-pp-cli balance | Snapshotted locally on every `sync` -> historical burn-rate becomes possible (see transcendence) |

### Recognition (`recognition:read`/`recognition:write` — in scope)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 22 | Give recognition | MCP `giveRecognition` | bonusly-pp-cli give | Structured flags synthesize the `+N @mention #hashtag` reason-string DSL so users never touch it; `--dry-run` shows the exact string that would be sent. Hand-built top-level command wrapping the generated `recognition create` passthrough (which takes the raw `reason` string directly) |
| 23 | Recognition received by a user | MCP `getRecognitionReceived` | bonusly-pp-cli recognition received | Synced locally, joins to org chart offline |
| 24 | Recognition given by a user | MCP `getRecognitionGiven` | bonusly-pp-cli recognition given | Same |
| 25 | Group recognition recipient count | MCP `getGroupRecognitionRecipientCount` | (generated endpoint) recognition group-count | — |
| 26 | Company recognition feed (dept/loc/team/hashtag/type filters) | MCP `getRecognitionFeed` | bonusly-pp-cli recognition feed | Full sync -> offline FTS5 search, richest non-admin dataset in the whole API |
| 27 | List recognition type values | MCP `listRecognitionTypes` | (generated endpoint) recognition list-types | — |
| 28 | Semantic + keyword recognition search | MCP `searchRecognitions` | (behavior in bonusly-pp-cli search --type recognition) | Framework-generated `search --type recognition` already gives free FTS5 keyword search over synced recognition rows once `sync` has run; no dedicated hand-built command needed. Scoped honestly to keyword search — the vector/semantic half of the MCP tool's hybrid retrieval has no local equivalent |
| 29 | Last-recognized-at for a batch of users | MCP `getRecognitionGivenToUsers` | bonusly-pp-cli recognition last-given | Batches past MCP's 20-user cap via local roster |
| 30 | Get a single recognition | MCP `getRecognition` | (generated endpoint) recognition get | — |
| 31 | Edit a recognition (within 24h) | MCP `updateRecognition` | (generated endpoint) recognition update | — |
| 32 | Delete (undo) a recognition (within 24h) | MCP `deleteRecognition` | (generated endpoint) recognition delete | — |

### Awards & incentives (`awards:read`/`recognition:write` — in scope)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 33 | List claimable/manual awards | MCP `listAwards` | bonusly-pp-cli awards list | — |
| 34 | Show a single award | MCP `showAward` | (generated endpoint) awards get | — |
| 35 | Claim a claimable incentive | MCP `claimIncentive` | bonusly-pp-cli incentives <id> | `--dry-run` support (single-endpoint resource, auto-promoted bare, takes positional id) |
| 36 | Give a custom award | MCP `giveAward` | bonusly-pp-cli awards give | — |

### Rewards & redemptions (`rewards:read` — in scope, own data only)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 37 | Get a single redemption (own) | MCP `getRedemption` | (generated endpoint) redemptions get | — |
| 38 | List my own redemptions | MCP `getMyRedemptions` | bonusly-pp-cli redemptions list-mine | Synced locally, feeds spend-forecast transcendence feature |

### Explicitly OUT OF SCOPE for this build (admin-gated; not silently dropped)

All require `*:administer` scopes that a personal/regular-employee PAT cannot be granted, per three independent Bonusly help-center confirmations:

- **Award admin CRUD:** `adminListAwards`, `adminShowAward`, `adminCreateAward`, `adminUpdateAward`, `adminDeleteAward` (`awards:administer`)
- **Approval workflow:** `listAwardApprovalRequests`, `getAwardApprovalRequest`, `approveAwardApprovalRequest`, `denyAwardApprovalRequest` (`awards:administer`)
- **Reward admin actions:** `adminRewardsApproveRedemption`, `adminRewardsDeclineRedemption`, `adminRewardsRefundRedemption`, `adminRewardsFulfillRedemption`, `adminRewardsUnfulfillRedemption`, `adminRewardsTotalsReport`, `adminRewardsRedemptionsReport` (`rewards:administer`)
- **User admin:** `adminCreateUser`, `adminUpdateUser`, `adminDeactivateUser`, `adminGetUser`, `adminListUsers`, `adminActivateUser`, `adminGetGiveBalance`, `adminCreateGiveBalanceIncrement` (`user:administer` / `rewards:administer`)
- **Company reporting:** `adminParticipationReport`, `adminUsersLastRecognized` (`recognition:administer`/`reports:administer`)
- **MCP usage telemetry:** `adminGetMcpUsageTotals`, `adminGetMcpUsageDetails` (`reports:administer`/`finance:administer`)
- **Analytics API** (`/api/public/analytics/*`, snapshot+cursor+tombstone sync of RecognitionEvents/AnalyticsUsers/group-stats/templates) (`analytics:administer`) — this is the best-designed sync surface in the entire API and it is the one we cannot use. Document as an honest known-gap, do not attempt a workaround.

This 28-tool exclusion list is the direct cause of this CLI's product thesis: the analytics a regular employee wants (participation, equity, spend forecasting) exist in Bonusly's own admin tooling, and this CLI reconstructs an honest, non-admin-scoped approximation of them from the ~38 tools above.

## Transcendence (only possible with our approach)

Full brainstorm + adversarial-cut audit trail: `research/2026-08-10-154500-novel-features-brainstorm.md` (subagent run, 2 rounds — round 1 rejected for contract violations, round 2 accepted).

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|-------------------|
| 1 | Recognition budget audit | `recognition audit --dept <name>` | 9/10 | hand-code | Joins synced recognition feed against department headcounts (absorbed row #6) to compute spend-vs-budget per team, entirely offline — the admin Participation Report does this but is `reports:administer`-gated | none |
| 2 | Personal recognition history search | `recognition search-mine` | 8/10 | hand-code | FTS5 index over a local sync scoped to the caller's own given+received recognition — distinct from the company-wide feed search, searchable with zero network calls once synced | none |
| 3 | Points burn-rate / forfeiture forecast | `balance history` | 8/10 | hand-code | Diffs successive local snapshots of the points-balance endpoint (absorbed row #21), which the live API only ever returns as a single point-in-time value with no history | none |
| 4 | Neglected-teammate finder | `recognition gap --manager <id>` | 8/10 | hand-code | Joins the org-tree direct-reports endpoint (absorbed row #11) against recognition-received history to flag teammates not recognized within N days — the admin `adminUsersLastRecognized` tool does exactly this but is `reports:administer`-gated; this is the honest non-admin approximation | none |
| 5 | Department hashtag/values trend audit | `recognition values --dept <name>` | 7/10 | hand-code | Aggregates hashtag frequency across synced feed rows, scoped by department headcount (absorbed row #6) — no live endpoint returns pre-aggregated hashtag trends | none |
| 6 | Redemption spend forecast | `redemptions forecast` | 6/10 | hand-code | Simple linear projection over the caller's own local redemption history (absorbed row #38); must be described honestly as a basic trend line, not a sophisticated model | none |

**Excluded from this table by editorial call (not the subagent's):** "Full Recognition Feed Sync" (`sync --resources recognition`) scored 5/10 but is `spec-emits` — it is the generated `sync` command every printed CLI has, not a differentiator. Kept out of the transcendence count; it remains Priority-0 foundation work (see brief's Build Priorities). "Give recognition," "Fuzzy user search," "Quick balance check," and admin-scoped candidates were killed by the subagent as already-absorbed, wrapper, or auth-gated respectively — see the brainstorm audit trail for full kill reasoning.

**Hand-code commitment: 6 features, ~50-150 LoC each plus `root.go` wiring, all tagged `hand-code`. Zero `spec-emits` transcendence rows** (the one candidate that would have been `spec-emits` was excluded per the note above as non-differentiating). This is a real scope commitment Phase 3 must deliver in full — no mid-build downgrades to stub without returning to this gate for re-approval.

## Known Gap (surface prominently at Phase Gate 1.5 and in the README)

Bonusly's own Analytics API (`/api/public/analytics/*`) is the best sync design in the entire product — async NDJSON snapshot, resumable `{recomputed_at, row_key}` cursor, `tombstone:true` soft-deletes. It requires `analytics:administer`, an admin/Organization-plan scope this CLI's target user cannot obtain. This CLI's `sync` therefore cannot detect deletions and must page the live feed until it reaches already-seen records rather than resuming from a true change-data-capture cursor. This is an honest, permanent limitation of building for the non-admin tier — not a bug to fix later.
