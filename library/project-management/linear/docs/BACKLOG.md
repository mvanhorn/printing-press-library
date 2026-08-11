# linear-pp-cli coverage backlog

Generated 2026-08-11.

## What this is

The Linear coverage goal produced a 69-row gap matrix at
`.goal-linear-coverage/gap-matrix.json` (generated 2026-08-11T18:26:49Z), a left
join of the live Linear GraphQL surface onto the CLI command surface. Most of
those rows were closed in the same working session. This file is the ledger of
what was deliberately left, why, and what it would cost.

Every one of the 69 rows is accounted for below as **closed**, **partial**, or
**backlogged**. No row is dropped.

**The next goal starts here.** Do not re-run discovery against Linear to find
work. Read this file, pick from the ranked backlog table, and re-derive only the
one thing this file marks as a prerequisite (GAP-062, the stale vendored
`schema.graphql`). The inventories this backlog was cut from stay valid until
Linear ships new root fields:

- `.goal-linear-coverage/api-inventory.json` and `.md`, the live surface
- `.goal-linear-coverage/cli-inventory.json` and `.md`, the pre-session CLI surface
- `.goal-linear-coverage/gap-matrix.json` and `.md`, the join
- `.goal-linear-coverage/contract-state-groups.md` and `contract-reconcile.md`, the two designs written this session
- `CHANGELOG.md`, the Unreleased section, which is the record of what shipped

## Coverage headline

| | Mutations wrapped | Live mutations | Coverage |
|---|---|---|---|
| Before this session | 9 | 361 | 2.5 percent |
| After this session | **77** | 361 | **21.3 percent** |

**Denominator.** 361 is `counts.live_mutations` in `gap-matrix.json`, which is
the count of non-deprecated entries in `api-inventory.json` `mutations[]` (373
total, 12 deprecated). Confirmed independently: every one of the 77 wrapped
names appears in that live set, so none of the numerator is deprecated surface.

**Numerator, method.** Count the distinct GraphQL root field selected by every
`mutation` document in the Go source. The documents are raw string constants,
almost all anonymous (`mutation($id: String!) {`), so the root field is the first
identifier after the opening brace, which is often on the next line. Comment
lines are dropped first. Run from the repository root:

```sh
awk '
  /^[[:space:]]*\/\// { next }
  { line=$0
    while (match(line, /mutation[^{]*\{/)) {
      rest=substr(line,RSTART+RLENGTH); line=rest; sub(/^[[:space:]]*/,"",rest)
      if (match(rest,/^[a-z][A-Za-z0-9_]*/)) { print substr(rest,1,RLENGTH); pend=0 } else pend=1
    }
    if (pend==1 && $0 !~ /mutation[^{]*\{/) { s=$0; sub(/^[[:space:]]*/,"",s)
      if (match(s,/^[a-z][A-Za-z0-9_]*/)) { print substr(s,1,RLENGTH); pend=0 } }
  }' internal/client/*.go internal/cli/*.go | sed -nE '/^[a-z]+[A-Z]/p' | sort -u | wc -l
```

The trailing `sed` keeps only camelCase identifiers, which drops the six Go
keywords and locals (`c`, `id`, `if`, `out`, `return`, `var`) that the raw
extraction picks up from lines mentioning the word mutation in code rather than
in a document. Raw 83, minus those 6, gives **77**.

Restricting the same command to `internal/client/*_queries.go`, the files added
this session, gives **69**. The other 8 are the pre-existing wraps that live
elsewhere: `commentCreate`, `commentUpdate`, `documentCreate`, `documentUpdate`,
`issueArchive`, `issueCreate`, `issueUpdate` and `projectUpdateCreate` in
`internal/cli/`, plus `fileUpload` in `internal/client/upload.go`. Those 9 are
exactly the pre-session baseline, so the session's net new is 68.

## Status of all 69 rows

- **Closed: 44 rows.** GAP-001 through GAP-009, GAP-011 through GAP-020,
  GAP-022 through GAP-037, GAP-039, GAP-040, GAP-042 through GAP-047, GAP-049.
  Each has a `CHANGELOG.md` Unreleased entry. Do not re-open these from the gap
  matrix without reading the changelog first.
- **Partial: 4 rows.** GAP-010, GAP-021, GAP-038, GAP-041. The remainder of each
  is a backlog row below.
- **Backlogged: 21 rows.** GAP-048 and GAP-050 through GAP-069.

Two closed rows carry a caveat worth carrying forward:

- **GAP-017 (output contract) is resolved by documentation, not by uniformity.**
  `projects list`, `projects search`, `initiatives list` and `initiatives search`
  were converted to the `{results, meta}` envelope, which was a breaking output
  change, and the fourth envelope died with the `integrations` stubs. Two shapes
  remain distinct **by design**: the bare resolver object emitted by
  `projects resolve` and `initiatives resolve`, and the `{event: ...}` mutate
  envelope emitted by writes and dry runs. Both are now stated as contracts under
  the README's Output contract heading. A future goal that wants one shape
  everywhere is a new decision, not an unfinished one.
- **GAP-025 (deprecated leaves)** was closed by marking `roadmaps` and
  `roadmap-to-projects` deprecated in cobra rather than deleting them. They still
  run. Deleting them is a breaking change to hold for a major version.

### Residue inside closed rows

These rows are closed on their stated capability, but named operations inside
them are still unwrapped. Listed so a later sweep does not have to rediscover
them.

| Row | Still unwrapped | Note |
|---|---|---|
| GAP-007 search | `issueVcsBranchSearch` | Branch-name lookup, not issue text search. `searchIssues` and `semanticSearch` both ship. The deprecated `issueSearch` was deliberately dropped. |
| GAP-028 projects write | `projectUnarchive`, `projectReassignStatus` | `projectDelete` trashes rather than erases, so unarchive is the missing restore verb. `projectReassignStatus` is a bulk status migration, adjacent to GAP-048. |
| GAP-032 notifications | `notificationSubscriptionCreate`, `notificationSubscriptionUpdate` | The inbox is fully wrapped. Subscribing to a new entity is a separate capability. `notificationSubscriptionDelete` is deprecated upstream in favour of setting `active` false. |
| GAP-045 attachments | 11 of 15: `attachmentLinkDiscord`, `attachmentLinkFront`, `attachmentLinkGitHubIssue`, `attachmentLinkGitHubPR`, `attachmentLinkGitLabMR`, `attachmentLinkIntercom`, `attachmentLinkJiraIssue`, `attachmentLinkSalesforce`, `attachmentLinkSlack`, `attachmentLinkZendesk`, `attachmentSyncToSlack` | Every one requires a connected third-party integration in the workspace. `attachmentLinkURL` covers the generic case and lets Linear unfurl whichever provider it recognises. Properly part of GAP-063, the admin integrations family. |

## Ranked backlog

Rank and effort are carried verbatim from `gap-matrix.json`. Effort is `s`
(hours), `m` (a day), `l` (multi-day). Rows are in rank order, so the partial
remainders sort into their original positions.

Reason vocabulary:

- **rank below cutoff**: nothing wrong with it, the session ran out of ranks
  above it. These are the ones to take first.
- **admin family**: workspace administration, not issue workflow. An agent
  driving issues never connects a Slack workspace or verifies a domain.
- **needs OAuth actor**: not reachable with the personal API key this CLI
  authenticates with.
- **needs streaming transport**: needs a websocket or webhook receiver, which a
  request-and-response CLI process does not have.
- **deprecated upstream**: the operation exists but Linear is removing it.

| Gap | Rank | Capability | Reason left | Effort |
|---|---|---|---|---|
| GAP-010 (remainder) | 10 | Relations are synced by an `issueRelations` crawl and by write-through from `relations list`, so `--data-source local` works. Subscription-based freshness, meaning a relation changed by someone else invalidating the local row without a resync, is not built. | needs streaming transport, blocked behind GAP-058 | m |
| GAP-021 (remainder) | 21 | `--include-archived` ships on `issues list`, `relations list`, `attachments for-url` and `notifications list`. It is absent from every other read: `issues get` and `issues search`, `projects` and `initiatives` list, get, search and resolve, `documents` list and get, `comments list`, `labels list`, `cycles list`, `workflow-states list`, `milestones list`, `templates` list and get, `custom-views` list and get, `reactions list`, `project-updates list`, `notifications get`, `teams`, `users`, `me`, and all 25 promoted generic leaves in `internal/cli/linear_graphql_promoted.go`, whose shared query builder emits no `includeArchived` argument at all. | rank below cutoff. The four that ship are the four where an archived row is load bearing. The generic leaves are one builder change plus one flag, so this is cheaper than the row count suggests. | s |
| GAP-038 (remainder) | 38 | 8 of the 35 bare `id`/`data`/`synced_at` shell tables now populate: `documents`, `templates`, `custom_views`, `favorites`, `project_milestones`, `project_statuses`, `initiatives` (the sync allowlist in `internal/store/extended_sync.go`) and `issue_relations` (its own typed writer in `internal/store/relations.go`). 27 still never receive a row, so `sql` against them silently returns an empty set: `attachments`, `audit_entry_types`, `auth_resolver_responses`, `authentication_session_responses`, `customer_statuses`, `customer_tiers`, `customers`, `email_intake_addresses`, `entity_external_links`, `initiative_to_projects`, `integration_templates`, `integrations`, `integrations_settingses`, `issue_priority_values`, `issue_to_releases`, `organization_invites`, `organization_metas`, `organizations`, `project_labels`, `project_relations`, `release_pipelines`, `release_stages`, `releases`, `roadmap_to_projects`, `roadmaps`, `team_memberships`, `user_settingses`. | Mixed. `auth_resolver_responses` and `integrations` back commands that were deleted this session and should be dropped from the migration rather than filled. `roadmaps` and `roadmap_to_projects` are deprecated upstream. The rest are the storage half of backlog families below (releases GAP-059, customers GAP-060, project taxonomy GAP-048, admin GAP-063 to GAP-069), so each fills when its family is wrapped, not before. | m |
| GAP-041 (remainder) | 41 | Initiative CRUD, archive, unarchive, delete and project links ship. 16 mutations remain: `initiativeAddLabel`, `initiativeRemoveLabel`, `initiativeLeadTeamUpdate`, `initiativeUpdateCreate`, `initiativeUpdateUpdate`, `initiativeUpdateArchive`, `initiativeUpdateUnarchive`, `initiativeRelationCreate`, `initiativeRelationUpdate`, `initiativeRelationDelete`, `initiativeToProjectUpdate`, `initiativeLabelCreate`, `initiativeLabelUpdate`, `initiativeLabelDelete`, `initiativeLabelRetire`, `initiativeLabelRestore`. | rank below cutoff. The portfolio layer is writable, which was the blocking half. Initiative labels mirror the shipped `labels` family exactly and initiative updates mirror the shipped `project-updates` family exactly, so this is pattern-copy work. | m |
| GAP-048 | 48 | Project statuses, labels and relations are read-only: 12 mutations across `projectStatus*`, `projectLabel*` and `projectRelation*`. | rank below cutoff. Note `projectLabelCreateInput.teamId` is live-only against the vendored schema, so GAP-062 first. | m |
| GAP-050 | 50 | Team and team-membership management: 9 mutations, `teamCreate` through `teamMembershipDelete`. | admin family. Also carries a trap: `TeamCreateInput.markedAsDuplicateWorkflowStateId` is deprecated, returns `success: true` and does nothing, so it must never become a flag. | m |
| GAP-051 | 51 | User and user-settings writes: `userUpdate`, `userSettingsUpdate`, `userSettingsFlagsReset`, `userFlagUpdate`. | admin family. `userSettingsUpdateInput.unsubscribedFrom` is deprecated and ignored, so it must not be exposed. | s |
| GAP-052 | 52 | Cursor, ordering and sort control across the read surface: 65 connection types and 102 filter and comparator inputs reachable through a handful of flags. | rank below cutoff, and partly eroded already. Every read leaf added this session takes `--after`, and `notifications list` takes `--order-by` and `--max-pages`. The pre-existing core reads (`issues list`, `projects list`, `initiatives list`, `labels list`, `workflow-states list`, `teams`, `users`) still expose only `--limit`, so a large read cannot be resumed or ordered. | m |
| GAP-053 | 53 | Triage responsibility: `triageResponsibilityCreate`, `Update`, `Delete` plus the two reads. | rank below cutoff. Relevant later because `triage` is now a first-class state type in the groups vocabulary and `Team.triageIssueState` exists. | s |
| GAP-054 | 54 | Entity external links: `entityExternalLinkCreate`, `Update`, `Delete`, and the read is reachable only by id. | rank below cutoff. This is the reconcile primitive for when the right answer is neither a relation nor a comment, so it is the strongest candidate in this band. | s |
| GAP-055 | 55 | View preferences: `viewPreferencesCreate`, `Update`, `Delete`. | rank below cutoff. UI state for a human client, of little use to an agent. `ViewPreferencesCreateInput.cycleId` is deprecated and `initiativeLabelId` is live-only drift, so GAP-062 first. | s |
| GAP-056 | 56 | Issue sharing and reminders: `issueShare`, `issueUnshare`, `issueReminder`. | rank below cutoff. `issueShare` and `issueUnshare` are two of the 22 mutations present live but missing from the vendored schema, so GAP-062 first. | s |
| GAP-057 | 57 | Emoji: `emojiCreate`, `emojiDelete`, and the emoji reads. | rank below cutoff. Custom emoji upload for a workspace. Recorded only to complete the family sweep. | s |
| GAP-058 | 58 | All 82 GraphQL subscription fields. Nothing in the CLI can receive a change event. | needs streaming transport. The `tail` command that pretended to do this was deleted, so the CLI no longer claims a capability it lacks. Blocks the GAP-010 remainder. Consider GAP-067 webhooks as the cheaper receiver. | l |
| GAP-059 | 59 | Releases family is read-only behind four read leaves: 23 mutations across `release*`, `releaseNote*`, `releasePipeline*`, `releaseStage*` and `issueToRelease*`. | rank below cutoff, and blocked on GAP-062: six `Release*` input types drifted live-only by four fields each, making this the family most likely to generate wrong code from the stale local schema. | m |
| GAP-060 | 60 | Customer and customer-need family: 18 mutations and 6 queries, entirely absent. | rank below cutoff. This is Linear's CRM layer, orthogonal to issue workflow. Contains one item worth stealing later: the query `issueTitleSuggestionFromCustomerRequest`, the only server-side title synthesis helper in the schema, which reconcile could use. | m |
| GAP-061 | 61 | Linear's own agent protocol: 12 `agentSession`, `agentActivity` and `agentSkill` mutations plus 6 queries. | needs OAuth actor. Agent sessions are authored by an app actor, not by a personal API key. Also blocked on GAP-062: five of the mutations and the four `hasActiveAgentSessions`-style `IssueFilter` fields are live-only drift. | m |
| GAP-062 | 62 | Regenerate the vendored `schema.graphql` with `includeDeprecated: true`. The local copy is a strict subset: 22 live-only mutations, 14 live-only queries, 73 additively drifted input types, 15 fields deprecated live but unmarked locally, 29 live-deprecated input fields absent entirely. | rank below cutoff on its own, but it is the **prerequisite** for GAP-048, GAP-055, GAP-056, GAP-059, GAP-061 and GAP-066. Do this first in any next goal that touches those. Cheap. | s |
| GAP-063 | 63 | Third-party integrations: 58 mutations covering connect, configure, post-target and delete for every provider. | admin family, and needs OAuth actor for most providers. Absorbs the 11 `attachmentLink*` residue from GAP-045. Three of the 58 are deprecated upstream. | l |
| GAP-064 | 64 | Organization, domain, invite, trial and security settings: 13 mutations plus the org read surface. | admin family. Six `OrganizationUpdateInput` fields are deprecated and silently ignored and must never surface as writable flags. `organizationStartTrial` is deprecated in favour of `organizationStartTrialForPlan`. | m |
| GAP-065 | 65 | Authentication, SSO and session mutations: email, SAML, Google and passkey auth, logout variants, session revocation. | needs OAuth actor, and partly hostile to a CLI since these are browser flows. The CLI authenticates with a personal API key in the auth header with no Bearer prefix, and its persisted OAuth fields are never read. | m |
| GAP-066 | 66 | OAuth application management: 5 mutations and 2 queries. | needs OAuth actor and admin family. Every one is flagged live-only against the vendored schema, so it cannot even be described locally until GAP-062. | m |
| GAP-067 | 67 | Webhook management: `webhookCreate`, `Update`, `Delete`, `RotateSecret` plus the reads. | admin family, but the highest-leverage row in the admin band: webhooks are the other way to receive change events, so this plus a small receiver is the cheap alternative to GAP-058. | s |
| GAP-068 | 68 | Issue import and CSV export: 9 import mutations, 3 import check queries, `createCsvExportReport`. | admin family. Note the deleted `import` and `export` commands were broken REST stubs and were never wrappers over these, so there is no regression here to repair. | m |
| GAP-069 | 69 | Remaining administrative families, batched: audit entries, time schedules, git automation states and target branches, email intake addresses, push subscriptions, SLA configurations. | admin family. `gitAutomationStateCreateInput.branchPattern` and `PushSubscriptionCreateInput.userId` are deprecated. | m |

## Suggested order for the next goal

1. GAP-062, regenerate the schema. Everything below it that touches drifted input types depends on it, and it costs hours.
2. GAP-021 remainder, one builder change plus a flag across the promoted read leaves.
3. GAP-054 external links and GAP-053 triage, the two small rows an agent workflow actually reaches for.
4. GAP-041 remainder and GAP-048, both pattern-copies of families that already ship.
5. GAP-067 webhooks plus a receiver, which retires the GAP-010 freshness remainder far more cheaply than GAP-058 does.

Everything after that is the admin band, which no issue-workflow agent needs.
