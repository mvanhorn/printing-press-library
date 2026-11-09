# Duplicate proposals (identity rules v2, board #69): contacts duplicate-proposals get / decide (2026-10-15)

Superseded names (2026-10-16, board #95): the curated OpenAPI renamed the commands to `list-contact` / `decide-contact` and the
MCPB ids to `contacts.duplicate-proposals.list-contact` / `.decide-contact`; the old names below stay aliases. See
duplicate-proposals-look-board-95.md.

App main 61bf61876. The public OpenAPI at that commit (`public/openapi.yaml`) is the contract; only the two new operations were taken.

- `spec.yaml` and `upstream/openapi.yaml`: the two path blocks `/contacts/{id}/duplicate-proposals` (GET) and
  `/contacts/{id}/duplicate-proposals/{proposalId}` (POST), copied VERBATIM from the public OpenAPI. They are the app's generic placeholders
  (operationId `getContactsIdDuplicateProposals` / `postContactsIdDuplicateProposalsProposalId`, summaries "Get contacts id duplicate-proposals" /
  "Create / invoke ...", the POST body a free object with no `decision` property), the same shape as the existing `contacts collaborators` blocks.
  The drift gate (`TestSpecMatchesUpstreamPublicAPI`) stays honest for these two paths. The snapshot was NOT refreshed wholesale.
- Generation: `printing-press` 4.32.1 on the old and the new spec.yaml into scratch dirs (gen-old, gen-new; each ended on the generator's own
  govulncheck gate, files complete as in the previous lane), then the difference applied by hand in the 4.28.0 print's style (`classifyAPIError(err, flags)`,
  no `wrapPlatformStructuredOutput`, no `Mutating`/`HeaderParams` in code_orch, `printOutputWithFlagsMeta` without the trailing nil, `compactFields(x)`).
  Applied: `contacts_duplicate-proposals.go`, `contacts_duplicate-proposals_get-contacts-id.go`, `contacts_duplicate-proposals_post-contacts-id-proposal-id.go`,
  the `contacts.go` registration, two `code_orch.go` endpoints, the `tools.go` group and `tool_count` 728 -> 730, two `tools-manifest.json` tools.
  The reads use the "live" strategy.
- Not applied from the 4.32.1 delta (as in the unknown-caller lane): `sync.go`, `store.go`, `learn_init.go` and store test additions for a synthetic
  `duplicate_proposals` sync table (a per-contact read and a POST acknowledgement are not data to sync); `root_test.go` command-list additions.
  `.printing-press.json` `mcp_tool_count` (726) was not touched, as in the previous lane.
- Command shapes as generated: `contacts duplicate-proposals get-contacts-id <id>` and
  `contacts duplicate-proposals post-contacts-id-proposal-id <id> <proposalId>`. The names are ugly because the public summaries are placeholders;
  curating the app's OpenAPI (operationId, summary, a typed `decision` body) is an app follow-up, after which a reprint will rename them.

## Hand-written, a reprint must keep these

- The POST has a `--decision` flag (not generated: the public body is a free object). Required unless `--stdin`; refused together with `--stdin`;
  exactly `not_sure`, `different` or `merge` (lower case); `--stdin` JSON must be exactly `{ "decision": <one of those> }`. Both ids are UUID-checked
  (the GET checks its id) before any request. The meaning of each value, "merge only records an ALREADY merged pair, the command never merges", and the
  refusals (`not_merged`, `already_decided`, `other_record_not_editable`) are in the `Long` help and the `tools-manifest.json` description.
- Guards in `internal/cliutil/duplicate_proposal_guards.go` (`ProposalDecisions`, `CheckUUID`, `CheckProposalDecision`, `CheckProposalBody`), shared with
  the MCPB executor: `handleCodeOrchExecute` in `internal/mcp/code_orch.go` has a hand-written `case` for both endpoint ids that refuses a non-UUID id,
  a missing, non-string or unknown decision, and any extra body key, with a tool error and no request. A reprint regenerates `code_orch.go` without it and MUST re-apply it.
- Tests: `internal/cli/duplicate_proposals_test.go`, `internal/cliutil/duplicate_proposal_guards_test.go`, and the two tests appended to
  `internal/mcp/unknown_caller_guards_test.go` (shared `execGuard` harness).
- The POST command does NOT call `writeMutationResponseToStore` (the generator emits it for every POST). The `{ id, status, decidedAt }` acknowledgement is
  an incomplete record (no rule, no other record), and terminal `different` / `merged` pairs disappear from the live GET (which selects only `open` and
  `not_sure`), so persisting it would create a misleading searchable `duplicate-proposals` resource. A reprint MUST drop that call again. Test: `TestDuplicateProposalsDecideLeavesTheLocalStoreUnchanged`
  (Codex parity r1 P2).
