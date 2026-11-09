# Duplicate proposals look (board #95): list paging, decide revisions, import rows; the two contact commands renamed (2026-10-16)

Contract: the app's public OpenAPI at branch tip f2d638db6, frozen in the Cerebro session-tools folder
(`frozen-contracts/f2d638db6/openapi.yaml`). Only three path blocks were taken, VERBATIM:
`/contacts/{id}/duplicate-proposals` (GET, now curated: operationId `listContactDuplicateProposals`, query cursor / limit /
held_cursor / held_limit, typed answer), `/contacts/{id}/duplicate-proposals/{proposalId}` (POST, now curated:
`decideContactDuplicateProposal`, typed `decision` body, answer with previousMembershipRevision / membershipRevision / total) and
the new `/imports/{id}/duplicate-proposals` (GET, still a generated placeholder `getImportsIdDuplicateProposals`, page / per_page).
Written into both `spec.yaml` and `upstream/openapi.yaml` (the drift gate stays honest); no wholesale refresh.

Generation: `printing-press` 4.32.1 on the old and the new spec.yaml into scratch dirs, delta applied by hand in the 4.28.0 print's
style (`classifyAPIError(err, flags)`, `compactFields(x)`, no `wrapPlatformStructuredOutput`, `printOutputWithFlagsMeta` without the
trailing nil, the 4.28 `resolvePaginatedReadWithStrategy` signature without responsePath, no `collectionItemsForOutput`).

- The curated operationIds RENAME the two contact commands: `get-contacts-id` -> `list-contact`, `post-contacts-id-proposal-id` ->
  `decide-contact` (files renamed with git mv; MCPB endpoint ids `contacts.duplicate-proposals.list-contact` / `.decide-contact`;
  `tools.go` group endpoints; `tools-manifest.json` names `contacts_duplicate-proposals_list-contact` / `_decide-contact`).
- New: `imports_duplicate-proposals.go`, `imports_duplicate-proposals_get-imports-id.go`, the `imports.go` registration, one
  `code_orch.go` endpoint, one `tools.go` group, `tool_count` 730 -> 731, one `tools-manifest.json` tool.
- `internal/types/types.go`: the two generated response structs (`DuplicateProposalsListContactResponse`,
  `DuplicateProposalsDecideContactResponse`).
- Not applied (as in the earlier lanes): `sync.go` / `store.go` / `learn_init.go` / store test additions for synthetic
  `contacts_duplicate_proposals` and `imports_duplicate_proposals` sync tables, the `root_test.go` command-list changes,
  `.printing-press.json` `mcp_tool_count`.

## Hand-written, a reprint must keep these

- The old command names stay cobra aliases (`get-contacts-id` on list-contact, `post-contacts-id-proposal-id` on decide-contact), so
  scripts written against the identity v2 release keep working. Test: `TestDuplicateProposalsOldNamesStayAliases`.
- No `--all` on list-contact or get-imports-id. The generator emits it, but the 4.28 paginator merges a top-level list, and these
  answers are objects (two separately paged lists plus revisions for the contact; `{ rows, meta }` for the import), so `--all` would
  print a wrong or empty result. Callers follow nextCursor / page themselves.
- Only the paging flags the caller gave are sent (the generator always sends the int defaults); the bare request stays bare, as the
  identity v2 test requires.
- Local checks before any request (`internal/cliutil/paging_guards.go`: `CheckPageSize`, `CheckCursor`, `CheckPagingParams`):
  limit and held-limit 1-100, page 1-100000, per-page 1-100, a given but blank cursor refused; the import id is UUID-checked like the
  contact ids. The MCPB executor's hand-written `case` in `handleCodeOrchExecute` now covers the two renamed ids plus
  `imports.duplicate-proposals.get-imports-id` (UUID positionals) and applies `CheckPagingParams` to the list and the import rows.
- `code_orch.go` QueryParams: the generator leaves the pagination parameter out of the bindings (`cursor` on the list, `page` on the
  import rows); both were added by hand next to the generated ones.
- Kept from the identity v2 lane (see duplicate-proposals-identity-v2.md): the strict `--decision`, the UUID checks, no
  `writeMutationResponseToStore` in the POST. The `Long` help of both commands and the manifest descriptions document the paging
  fields and the revision rule.
- Tests: `internal/cli/proposals_look_test.go`, `internal/cliutil/paging_guards_test.go`, `internal/mcp/proposals_look_guards_test.go`;
  the identity v2 tests were updated to the new constructor names and endpoint ids.

## Parity round 2 (Codex r1 findings P1-P3, 2026-10-16)

- P1, legacy MCP ids: `handleCodeOrchExecute` maps `contacts.duplicate-proposals.get-contacts-id` to `.list-contact` and
  `.post-contacts-id-proposal-id` to `.decide-contact` (`codeOrchLegacyEndpointIDs` in `code_orch.go`) BEFORE the endpoint lookup, so
  the old ids pass through the same UUID, decision and paging guards. `tools-manifest.json` keeps two deprecated alias tools
  (`contacts_duplicate-proposals_get-contacts-id`, `_post-contacts-id-proposal-id`) next to the renamed ones. A reprint must keep both.
  Tests: `TestCodeOrchLegacyProposalEndpointIdsStillWork` (both ids executed end to end, guards still apply),
  `TestDuplicateProposalsHelpAndManifestAgreeOnBlankCursor` (aliases present).
- P2, provenance line: the generated `[]json.RawMessage` count printed `0 results (live)` for the object-shaped answers. Both commands
  now call `envelopeItemCount` (`internal/cli/proposals_look_output.go`): rows for the import answer, proposals + heldIdentifiers for
  the contact answer. Test: `TestDuplicateProposalsProvenanceCountsNestedArrays` (stdout is /dev/null, a character device, so the
  terminal path runs).
- P3, cursor wording: CLI flag help and both manifest cursor descriptions say "Omit it for the first page; an explicitly blank value
  is refused", matching `CheckCursor`.

## Reprint 637 (2026-10-30)

Kept: the cursor / page bindings in `code_orch.go`, the decide-contact Long and Example, the hand parameter texts and the two alias tools in
`tools-manifest.json`, the legacy endpoint ids.
