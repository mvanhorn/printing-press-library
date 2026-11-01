Polish Results for google-voice-pp-cli

Recommendation: **ship to the private local library after the parent refreshes live acceptance**. This is a mid-pipeline quality verdict, not public publication or installation approval. No public publish check, PR, remote HTTP MCP, source mutation, scheduler, or relationship-memory integration was added or run.

| Diagnostic | Before | After | Delta |
| --- | ---: | ---: | ---: |
| Scorecard | 80/100 | 80/100 | 0 |
| Verify | 100% (29/29) | 100% (29/29) | 0 |
| Unresolved hand-authored gosec findings | 18 | 0 | -18 |
| Raw gosec findings, including generated code | 108 | 90 | -18 |
| Go vet findings | 0 | 0 | 0 |
| Tools-audit pending findings | 0 | 0 | 0 |
| PII-audit pending findings | 0 | 0 | 0 |
| Actionable MCP catalog tools | 31 | 27 | -4 help-only groups |
| Meaningful live-source scorecard samples | not exercised | not exercised | unchanged |

Dogfood PASS (mock only; live matrix not exercised — run the live gate before publish). The binary's actual diagnostic verdict is **WARN**, exit 0, before and after: seven unused generated helpers remain. The structured PASS below means no failed dogfood gate; it does not conceal that non-blocking WARN. MCP Surface and all five novel-feature checks pass.

The scorecard reports five passing sampled commands, but they use an empty sandbox mirror. No meaningful message samples were assessed, so this report explicitly records `not_exercised`. The required forked output review found no framing or plausibility defects in those five empty samples; semantic relevance and ordering of nonempty source results remain unassessed by polish. The parent explicitly owns fresh final live/content/MCP checks against this source, so this limitation does not independently force hold.

Both binaries build, gofmt is clean, and the full existing Go suite passes: 14 tested packages plus 6 packages without test files. Final go vet is clean. Final verify-skill exits 0 with zero errors and 27 warnings marked likely false positives: its static parser treats dynamically registered child names as parent positional arguments. Actual child help and runtime registration exist. Workflow verification returns workflow-pass with an explicit no-manifest skip; no manifest or workflow was fabricated to improve the score.

**Fixes applied**

- Compare filesystem owner IDs in int64 without narrowing the current user's ID to uint32.
- Preserve primary failures together with close failures using errors.Join in session publication, mirror-open failure cleanup, schema inspection, and coverage inspection. Deferred rollback remains best effort; sql.ErrTxDone after successful commit cannot turn success into failure.
- Add narrow, statement-local security explanations for required Google SAPISIDHASH SHA-1, owner-only directory traversal modes, validated private mirror paths, fixed sibling-MCP execution without a shell, constant SQL fragments with bound caller values, and expected deferred rollback behavior.
- Mark account, threads, history and mirror as help-only parent groups. Runtime tools/list now contains 27 tools; all expected Voice leaves remain, read tools retain read-only hints, and mirror_sync remains a write. Improve the history and mirror help summaries.
- Ground account_get, threads_list and threads_get MCP descriptions in the bundled spec and implementation. Document the existing account, required thread ID, count bounds, and same-thread continuation. Apply the supported mcp-descriptions.json sidecar with a successful Printing Press mcp-sync; preserve stdio-only transport.
- Record the security and tool-discovery customizations in two schema-v2 patch files. Existing customization records and hooks remain intact. No release ledger or version stamp was edited.

**Tool and PII completion**

Both required playbooks were read and applied. Pass 2 reviewed the complete runtime catalog, including the learning reads/writes, SQL/context/workflow framework helpers and Voice leaves, beyond the mechanical audit. Tools-audit reports `no findings`; PII-audit, including the run manuscripts scope, reports `no findings`. Both local ledgers contain zero findings and no incomplete gates. No bulk accepts, synthetic findings, or acceptance fields were invented. There were no PII source values to replace or PII accepts requiring pre-decision fields.

**Security triage**

The pinned gosec v2.26.1 initially failed under host Go 1.27.3 with `package regexp without types`. Running that same pinned scanner with GOTOOLCHAIN=go1.26.10 produced complete before and after reports with no package-loading errors. Final gosec exits nonzero because it still reports the 90 generator-owned findings below; all custom Voice findings are cleared. The raw reports remain only in the private session directory.

The eight narrowly accepted custom-code warnings have exact local rationales:

| Rule / location | Rationale |
| --- | --- |
| G505, internal/voice/remote.go:6 | Google SAPISIDHASH requires SHA-1; this is protocol interoperability, not password storage. |
| G401, internal/voice/remote.go:57 | The fixed-origin timestamp/cookie SHA-1 signature must match Google's protocol; substituting a hash invalidates authentication. |
| G204, internal/cli/voice_core.go:558 | Process path is the fixed companion basename beside the running executable; no shell or caller-controlled arguments are used. |
| G302, internal/voice/session.go:130 | chmod targets a directory: 0700 provides owner traversal with no group/other permissions. |
| G302, internal/voice/mirror.go:88 | The private directory requires owner traversal; 0700 grants no group/other access. |
| G304, internal/voice/mirror.go:91 | This is an explicit operator-selected private mirror path; regular-file, owner/ACL and trusted-ancestor checks run before SQLite data access. |
| G202, internal/voice/mirror.go:427 | Only code-owned SQL fragments/order clauses are concatenated; every caller-supplied filter and limit uses a bound placeholder. |
| G104, internal/voice/mirror.go:177 | Deferred rollback is cleanup after returned primary/commit errors; ErrTxDone after commit is expected and must not be promoted to a failure. |

The remaining 90 findings are **generated-code retro candidates**, not claimed false positives or repaired locally. Generated source and the reserved cliutil/cobratree packages were not hand-edited. Exact rule and line groups:

| Generated file | Rule: lines |
| --- | --- |
| internal/cli/auth.go | G104: 819; G124: 1140, 1098; G204: 1428, 1426, 1424, 1422, 1420, 1418, 1416, 1414, 1412, 1410, 1408, 1406, 1404, 1402, 1400, 1398, 1253, 1227, 1179, 655, 1047; G304: 1065, 712, 704, 619 |
| internal/cli/deliver.go | G703: 155, 140, 126 |
| internal/cli/deliver_spacefast.go | G304: 120; G703: 120 |
| internal/cli/feedback.go | G304: 209, 67 |
| internal/cli/helpers.go | G304: 4238, 4199 |
| internal/cli/store_adopt_darwin.go | G103: 212, 197, 179; G115: 212, 167, 155; G304: 100 |
| internal/cli/teach.go | G304: 151, 128; G703: 211 |
| internal/cli/teach_playbook.go | G304: 380 |
| internal/client/client.go | G115: 3946, 3944, 4079, 3940, 3938; G302: 1716; G304: 317 |
| internal/client/cookiejar.go | G124: 336, 145; G304: 268 |
| internal/cliutil/credentials.go | G104: 688, 684, 620 |
| internal/cliutil/creds_perms_unix.go | G115: 71, 53 |
| internal/cliutil/darwin_acl.go | G115: 230 |
| internal/cliutil/darwin_acl_darwin.go | G115: 143 |
| internal/cliutil/filelock_unix.go | G115: 76 |
| internal/cliutil/testenv/sandbox_unix.go | G302: 16 |
| internal/cliutil/testenv/testenv.go | G302: 79 |
| internal/cliutil/text.go | G115: 115 |
| internal/config/config.go | G304: 172 |
| internal/learn/journal.go | G104: 279; G117: 266 |
| internal/learn/match.go | G101: 78 |
| internal/learn/playbooks.go | G304: 74 |
| internal/learn/teach_log.go | G304: 113 |
| internal/mcp/cobratree/shellout.go | G101: 518 |
| internal/platform/gate.go | G101: 23 |
| internal/platform/migration.go | G202: 158 |
| internal/platform/profile.go | G302: 303; G304: 511 |
| internal/platform/ratelimit.go | G304: 149 |
| internal/store/learnings.go | G202: 610 |
| internal/store/store.go | G104: 3930, 3564; G201: 4266-4269, 3839-3842, 1630, 1606; G202: 1155; G304: 311 |

**Other skipped findings and structural score limits**

- Seven dead helpers in generated internal/cli/helpers.go are retained for upstream review: booleanNoValueFlags, booleanSpaceFormPositionalErr, endpointArgsUsageErr, endpointPositionalArgs, endpointUsageLine, retainCLIQueryParams and retainExplicitQueryParams. Several are members of the same otherwise-unused generated helper chain; deleting generated utilities locally would not be durable.
- Cookie/SAPISIDHASH auth is not a bot/bearer/basic prefix, so the auth-protocol heuristic scores it poorly despite fixed-host read authentication. Source auth correctness belongs to parent live acceptance.
- Insight/workflow scores reflect this deliberately bounded evidence reader. Relationship judgments, source writes, remote HTTP MCP and new workflow features remain outside approved scope. No scaffolding was added for scoring.
- The tools manifest has an empty typed-tools array while runtime registration contains the three typed read tools plus command-mirror tools. Runtime tools/list was inspected directly; the manifest/scorer description dimensions remain unscored. The sanctioned override fixes the actual runtime descriptions, rather than inventing manifest entries.
- No local public-library clone was found; the working private build was treated as canonical. There was no public copy to synchronize.
- The lock-status gate confirmed no held lock. A required heartbeat attempt correctly failed because the parent intentionally released the lock for foreground polish; polish did not acquire a competing lock.
- Publish validation and Publish Offer were skipped because STANDALONE_MODE=false. Parent promotion, archive and any subsequent publication gates remain parent-owned.

**Changed paths**

CLI root: `<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli`.

- [.printing-press-patches/google-voice-agent-tool-discovery.json](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/.printing-press-patches/google-voice-agent-tool-discovery.json)
- [.printing-press-patches/google-voice-polish-security-invariants.json](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/.printing-press-patches/google-voice-polish-security-invariants.json)
- [.printing-press-pii-polish.json](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/.printing-press-pii-polish.json)
- [.printing-press-tools-polish.json](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/.printing-press-tools-polish.json)
- [internal/cli/history.go](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/internal/cli/history.go)
- [internal/cli/mirror.go](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/internal/cli/mirror.go)
- [internal/cli/voice_core.go](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/internal/cli/voice_core.go)
- [internal/mcp/tools.go](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/internal/mcp/tools.go)
- [internal/voice/mirror.go](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/internal/voice/mirror.go)
- [internal/voice/privacy_unix.go](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/internal/voice/privacy_unix.go)
- [internal/voice/remote.go](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/internal/voice/remote.go)
- [internal/voice/session.go](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/internal/voice/session.go)
- [mcp-descriptions.json](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/mcp-descriptions.json)
- [tools-manifest.json](<runstate>/chief-of-staff-9adf62f4/runs/20261031-175451-910ae94f/working/google-voice-pp-cli/tools-manifest.json)

The changed Go sources are internal/cli/history.go, internal/cli/mirror.go, internal/cli/voice_core.go, internal/mcp/tools.go, internal/voice/mirror.go, internal/voice/privacy_unix.go, internal/voice/remote.go and internal/voice/session.go. The generated MCP source changed through mcp-sync only. Rebuilt artifacts are the root CLI and both staged binaries. Parent must refresh the owned full live matrix and MCP calls before promotion; the parent has made no concurrent target edits.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---

Scope: five eligible empty sandbox samples; no meaningful source content assessed.

---POLISH-RESULT---
scorecard_before: 80
scorecard_after: 80
verify_before: 100
verify_after: 100
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: not_exercised
dogfood_live_matrix_after: not_exercised
govet_before: 0
govet_after: 0
gosec_before: 18
gosec_after: 0
tools_audit_before: 0 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Compare private-directory owner IDs without narrowing.
- Preserve primary errors together with cleanup-close errors; keep expected deferred rollback non-failing.
- Explain eight safe or protocol-required security findings with narrow local nosec rationales.
- Remove four help-only groups from actionable MCP discovery while preserving every Voice leaf and honest read/write hints.
- Improve three typed read-tool descriptions through the supported override sidecar and successful mcp-sync.
- Add two durable schema-v2 customization records.
skipped_findings:
- Dogfood returns non-blocking WARN exit 0 for seven unused generated helpers; PASS above denotes no failed dogfood gate.
- Ninety gosec findings in generated files are upstream retro candidates; exact rule/path/line groups appear above and are excluded from the hand-authored counts.
- Twenty-seven verify-skill warnings are marked likely false positives for dynamically registered child commands; exit 0 and zero errors.
- Workflow verification has no manifest and explicitly skips workflow execution; parent owns actual live workflows.
- Scorecard auth, insight, workflow and remote-transport limits reflect the bounded private cookie-auth reader; no out-of-scope features or scoring scaffolding added.
- Typed-tools manifest/scorer dimensions remain unscored; runtime tools/list was reviewed directly.
remaining_issues:
- Polish sandbox live matrix was not exercised on meaningful source content; parent explicitly owns refreshed final live/content and MCP acceptance before promotion.
ship_recommendation: ship
further_polish_recommended: no
further_polish_reasoning: Custom-code diagnostics and tool/PII gates are clear; another polish pass would repeat structural/generated warnings while the parent must run the final live gate.
---END-POLISH-RESULT---
