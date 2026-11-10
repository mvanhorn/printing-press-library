# Unknown-caller prompt (board #35): calls link, calls count-unknown-callers, contacts create --call-id, dialer get-history --unmatched/--line (2026-10-15)

App main f132ed6f0 (live 2026-10-15 9:45 PM PT). The public OpenAPI at that commit is the contract; only this feature's operations were taken.

- `spec.yaml`: new `POST /calls/{id}/link` (linkCallToContact) and `GET /calls/unknown-callers` (countUnknownCallers), copied verbatim from the public
  OpenAPI; `POST /contacts` request body is `allOf` ContactCreate + `callId` (uuid) with the public description and the 409/422 refusals; `GET /dialer/history`
  gains only the `unmatched` and `line` query params (its other public params, q/recorded_only/etc., were NOT added: out of scope for this lane).
- `upstream/openapi.yaml` (the drift gate's snapshot of the public API, taken at the 829-operation reprint): only the two new path blocks were added so
  `TestSpecMatchesUpstreamPublicAPI` stays honest for this feature. The snapshot was NOT refreshed wholesale: the public API at f132ed6f0 has 82 more
  operations than this spec (chat, help, Mailchimp, Zoom, WordPress forms, pool filter sets, email signature, and more); refreshing it fails the gate until
  a full reprint is done, which is a separate lane.
- Generation: `printing-press` 4.32.1 on the old and the new spec.yaml into scratch dirs (gen-old, gen-new), then the difference applied. The tree is still the
  4.28.0 print, whose generated files differ in shape from 4.32.1 output (no HeaderParams/Mutating in code_orch, `classifyAPIError(err, flags)`, `Changed || body != ""`
  became `body != ""`, no wrapPlatformStructuredOutput, `printOutputWithFlagsMeta` without the trailing nil). So `calls_count-unknown-callers.go`, `calls_link.go`,
  `calls_link_call-to-contact.go` are the 4.32.1 files translated to the 4.28.0 helpers; the hunks for `contacts_create.go`, `dialer_get-history.go`, `calls.go`,
  `code_orch.go`, `tools.go` and `tools-manifest.json` were applied by hand in the 4.28.0 style. A reprint under the current Press must produce the same commands.
- Not applied from the 4.32.1 delta: the `sync.go`, `store.go`, `learn_init.go` and store test additions for a synced "link" table (the generator treated
  `/calls/{id}/link` as a syncable sub-resource; a POST acknowledgement is not data to sync), and its "Use since for incremental fetches" query tip in
  `tools.go` (a false positive: `since` here is the counter's window start, not a modification-time filter).
- Command shapes as generated: `calls count-unknown-callers --line --since`, `calls link call-to-contact <id> --contact-id [--attach-number] [--dry-run]`,
  `contacts create --call-id <uuid>`, `dialer get-history --unmatched --line <n>`. On `calls link call-to-contact`, the local `--dry-run` flag shadows the root
  `--dry-run`, so it sends the API's body `dryRun: true` (the unknown-caller preview) and does not trigger the Press's request-printing dry run; pinned by a test.
  `calls` stays a hidden group, as in the 4.28.0 print; README.md and SKILL.md list `calls link call-to-contact` like the other calls commands.
- `tools-manifest.json` 726 -> 728 tools (BOTH new operations were added: the link tool and the counter); `mcp.tool_count` 726 -> 728 in `internal/mcp/tools.go` plus the calls group lists.
- Tests: `internal/cli/unknown_caller_test.go`.

## Round 2 (Codex parity r1 findings, 2026-10-15) - hand-written, a reprint must keep these

- `contacts create --call-id` is REFUSED with `--source`, `--assigned-to`, or `source` / `assignedTo` keys in `--stdin` JSON (the server derives both from the call
  and keeps an explicit `assignedTo`). The check lives in `internal/cli/unknown_caller_guards.go` (`checkCallDerivedFields`), called from the hand-edited
  `contacts_create.go` after the body is built (flags or stdin). The `--source` / `--assigned-to` flag texts and the `contacts_create` manifest entry say so.
  `spec.yaml` is NOT changed: the public OpenAPI example still supplies both fields (an app-repo item), so a plain reprint would regenerate the old texts.
- `dialer get-history --unmatched` is a boolean switch (`BoolVar`) that sends `unmatched=1` when set; the bare `--unmatched --line <n>` is tested.
- `calls count-unknown-callers --since` is validated with `time.Parse(time.RFC3339Nano)` (`checkRFC3339Instant`): Z or an explicit offset, a real date; offset-less
  and impossible dates (2026-02-31) are refused before any request. The 31-day window stays the API's rule.
- Tests: `internal/cli/unknown_caller_test.go` (refusals for flags and stdin, bare `--unmatched`, `since` cases).

## Round 3 (Codex parity r2 findings, 2026-10-15) - hand-written, a reprint must keep these

- The guards moved from `internal/cli/unknown_caller_guards.go` to `internal/cliutil/unknown_caller_guards.go` (exported `CheckCallDerivedFields`,
  `CheckRFC3339Instant`, `ErrCallDerivedFields`) so the companion MCP server shares them. The CLI callers (`contacts_create.go`,
  `calls_count-unknown-callers.go`) import `cliutil`.
- MCPB: `handleCodeOrchExecute` in `internal/mcp/code_orch.go` has a hand-written block before `newMCPClient()`: for `contacts.create` it refuses `callId` with
  `source` or `assignedTo`; for `calls.count-unknown-callers` it refuses a `since` that is not a string or not strict RFC 3339 (Z or offset, real date).
  Both refuse with a tool error and no request. A reprint regenerates `code_orch.go` without this block and MUST re-apply it.
- Tests: `internal/mcp/unknown_caller_guards_test.go` (callId+source, callId+assignedTo refused with zero HTTP hits; callId alone posts; offset-less,
  2026-02-31, leading-space and non-date `since` refused with zero hits; valid `since` sent).
- The Round 2 note above that places the guard in `internal/cli/unknown_caller_guards.go` is superseded by the cliutil location.

## Reprint 637 (2026-10-30)

GET /dialer/history is paginated in the public document (page, per_page, q, recorded_only, ...). `--unmatched` stays the hand-written
bool that sends `unmatched=1` (the print emits a string enum flag); the bare read now sends page=1&per_page=50 (test updated).

Round 3 (reprint 637): the local `--dry-run` on `calls link call-to-contact` is now `--server-dry-run` (body `dryRun: true`, request sent);
the global `--dry-run` prints and sends nothing there too. See reprint-637-server-dry-run.md.
