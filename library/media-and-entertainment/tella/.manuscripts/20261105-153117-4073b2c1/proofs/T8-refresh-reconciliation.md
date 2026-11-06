# T8 refreshed generator reconciliation

Completed local integration against restore point `a62d61665dde5620e095052547a72c1ccfab64a0`. The final product diff preserves the customized Tella workflows and public command signatures. No Press generation, rebuild, install, Git write, outbound message, credential lookup, or external API request was performed in T8.

## Inputs and provenance

## Applied behavior

`tail --dry-run` now returns intent before client creation. `tail --follow=false --json` emits a single filtered API response envelope rather than one event per item; NDJSON uses Cobra output so captured output and MCP shellout receive it. Existing unknown-resource checks, interval validation, no-cache behavior and forced single poll for harness/MCP remain.

When learning is disabled, `teach`, `teach-playbook` and `playbook amend` now emit `disabled:true` in JSON/agent mode while human and explicit quiet modes remain silent. Dry-run takes precedence and does not open the learning store. The new focused test exercises all three commands across human, JSON, agent, explicit quiet and dry-run output.

Three stale MCP command descriptions now agree with the corrected research. `tools-manifest.json` gains the refreshed 21-entry novel_features metadata without changing any prior endpoint, tool, auth, transport or confirmation data. Current README/SKILL examples and root/which descriptions already contained the corrected research and are byte-exact to the restore point.

## Every meaningful Go delta

| Refreshed file | Decision | Reason |
| --- | --- | --- |
| `internal/mcp/tools.go` | applied | Update three stale command-mirror descriptions from corrected research; preserve registrations, endpoint safety and input schemas. |
| `internal/cli/transcripts.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/platform_client.go` | retain_stronger_current_safety | Fresh delta removes source/profile validation, pre-resolver dry-run previews and mutation no-effects regression. Keep current safety code and tests; no genuine new implementation to import. |
| `internal/cli/exports_wait.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/tail.go` | applied | Add live data-source annotation, no-HTTP dry-run, single-poll filtered JSON response envelope and Cobra writer for NDJSON. Preserve interval validation and harness/MCP single-poll guards. |
| `internal/cli/teach_playbook.go` | applied | Use shared disabled-learning output for teach-playbook and playbook amend after dry-run; keep human/explicit quiet silence and no state creation. |
| `internal/cli/videos_clips_clean.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/clips_captions.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/exports_wait_test.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/workspace_stats.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/clips_transcript_diff.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/teach.go` | applied | Add writeLearningDisabled using existing quiet/agent output policy; honor dry-run before disabled learning and emit disabled:true for machine output. |
| `internal/cli/transcripts_search.go` | no_live_counterpart | Fresh change updates description in unused TODO novel search scaffold; current registered search is the implemented constructor in transcripts.go with corrected description. Do not copy scaffold. |
| `internal/cli/videos_viewed.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/webhooks_tail.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/clips_transcript_diff_test.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/clips_edit_pass_test.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/clips_edit_pass.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/videos_viewed_test.go` | retain_implementation | Fresh output replaces the actual novel implementation or substantive tests with TODO scaffolds/smoke tests. Keep current implementations, behavior tests, paths and signatures. |
| `internal/cli/which.go` | already_integrated | Current descriptions already match refreshed research for cached transcript search, public edit-pass and cut-only cleanup. Preserve runtime root version and all registrations. |
| `internal/cli/root.go` | already_integrated | Current descriptions already match refreshed research for cached transcript search, public edit-pass and cut-only cleanup. Preserve runtime root version and all registrations. |
| `internal/cli/platform_cli_test.go` | retain_stronger_current_safety | Fresh delta removes source/profile validation, pre-resolver dry-run previews and mutation no-effects regression. Keep current safety code and tests; no genuine new implementation to import. |
| `internal/client/client.go` | retain_trust_validation | Fresh delta removes explicit CertificateInvalidError refusal and Darwin untyped certificate-not-trusted chain classification. Keep current tested strict TLS trust behavior and logical-request idempotency. |
| `cmd/tella-pp-mcp/main.go` | already_versioned | Fresh var version=0.0.0-dev replaces a literal development string and reformats whitespace. Current runtime is already versioned at 1.0.0. Preserve exact current entrypoint bytes. |

## Other deltas

- **.printing-press.json**: Use fresh version 4.35.2 and generated_at; preserve original creator, contributor, spec URL/checksum, run ID, all 21 novels and existing validation evidence. Exact binary and source pin recorded in existing framework patch record.
- **tools-manifest.json**: New fresh adds novel_features metadata only; append identical 21 manifest contracts while retaining all existing endpoint tools, confirmation and transport metadata.
- **README.md, SKILL.md**: Current docs already contain corrected research descriptions and all three explicit novel examples, including --video=ID disambiguation. Keep expanded lesson docs and original attribution.
- **.goreleaser.yaml, LICENSE, NOTICE, Go copyright headers**: Owner-only new generation lost Greg full name and Course creator contributor; retain existing Greg Ceccarelli attribution and three contributor surfaces.
- **boolean endpoint examples**: Full old/new fresh scan found no other normalized Go changes beyond the 28 reviewed files. Boolean-example generator fix emits no additional changed endpoint example in this spec; current explicitly assigned --video=ID docs stay intact.

## Compatibility decision discovered by runtime verification

## Preservation evidence

- `T8-preservation.json` compares exact bytes with HEAD for both release ledgers, root.go, MCP main, thin registry, MCP walker, both link constructors, paid chat constructor, client TLS/idempotency implementation, profile dry-run code/tests, README, NOTICE and LICENSE.
- All 12 patch records remain. Eleven are byte-exact; the existing framework patch retains every prior contract and gains one refresh object plus file entries. No duplicate patch record.
- 21 novel contracts and all seven added workflow groups remain reachable. Current runtime discovery has the same 288 paths, Use strings, flags, types and defaults as T7.
- Cut-only rollback, strict recipe/trust validation, logical-request idempotency and explicit paid consent remain in unchanged implementation files and passed the full suite.
- Local rebuilt MCP fixtures exercise tella_read refusal, execute preview, explicit-confirm POST for each of the two link endpoints, read-only refusal and paid-chat refusal. Nine checks pass; exactly two POSTs hit loopback fixtures. No external service request or paid chat was made.
- Generic read-only POST search code and its tests remain unchanged and passed the full suite.

## Final validation

| Gate | Exit | Log |
| --- | --- | --- |
| focused | 0 | `<local-evidence>` |
| test | 0 | `<local-evidence>` |
| vet | 0 | `<local-evidence>` |
| build | 0 | `<local-evidence>` |
| tella-pp-cli-build | 0 | `<local-evidence>` |
| tella-pp-mcp-build | 0 | `<local-evidence>` |

Runtime logs: `<local-evidence>`; discovery proof `<local-evidence>`; loopback behavior `<local-evidence>`; rebuilt CLI JSON/version smoke `<local-evidence>`. Tool enumeration walks cursors to an absent terminal nextCursor and rejects duplicate tool names.

## Remaining work and boundaries