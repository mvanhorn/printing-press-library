# Local review findings and convergence

Nine semantic, provider, metadata/help and fare-scope findings were corrected in place across three review rounds, with four additional tool-description refinements. This is a new unpublished local workspace with no Git commits; the current source/tests and the three independent reviewer reports are the authoritative record.

Convergence: findings cleared at round 3. Final source/runtime confirmation: reviewer-round3.md and reviewer-round3-consistency.json. Review path: exactly one fresh-context gpt-6.1-sol MAX direct reviewer combining skill, document, output, correctness, security, API-contract, reliability, maintainability and resource checks. No additional reviewer or codex exec used.

## Template and reserved-scope candidates

Gosec reports39 framework/reserved findings and zero hand-authored provider findings. These are not changes introduced by the read-only provider adapter. Generator-owned/reserved packages were retained according to the Press template escape hatch; the source scanner report and triage JSON retain exact evidence. Template paths below are actual matching emit files when identifiable; unmapped paths are explicitly labeled. No user tradeoff or supported-feature reduction is pending.

| Source location | Rule / severity | Generator template candidate | Disposition |
| --- | --- | --- | --- |
| internal/client/chrome.go:149 | G402 / HIGH | chrome.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/client.go:438 | G119 / HIGH | client.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/platform/gate.go:22 | G101 / HIGH | platform_gate.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/cli/teach.go:210 | G703 / HIGH | teach.go.tmpl, learn/teach.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/store/store.go:3450-3453 | G201 / MEDIUM | store.go.tmpl, learn_lookups/store.go.tmpl, learn_patterns/store.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/store/store.go:3147-3150 | G201 / MEDIUM | store.go.tmpl, learn_lookups/store.go.tmpl, learn_patterns/store.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/store/store.go:1191 | G201 / MEDIUM | store.go.tmpl, learn_lookups/store.go.tmpl, learn_patterns/store.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/store/store.go:1167 | G201 / MEDIUM | store.go.tmpl, learn_lookups/store.go.tmpl, learn_patterns/store.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/store/learnings.go:609 | G202 / MEDIUM | learnings.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/platform/migration.go:156 | G202 / MEDIUM | platform_migration.go.tmpl | Generator-provided platform migration; SQLite literal escapes apostrophes before VACUUM INTO, so the concatenation detector is a false positive. |
| internal/store/store.go:253 | G304 / MEDIUM | store.go.tmpl, learn_lookups/store.go.tmpl, learn_patterns/store.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/platform/ratelimit.go:148 | G304 / MEDIUM | platform_ratelimit.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/platform/profile.go:510 | G304 / MEDIUM | platform_profile.go.tmpl, profile.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/learn/teach_log.go:112 | G304 / MEDIUM | learn/teach_log.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/learn/playbooks.go:73 | G304 / MEDIUM | learn/playbooks.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/config/config.go:109 | G304 / MEDIUM | config.go.tmpl, learn_entities/config.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/cli/teach_playbook.go:376 | G304 / MEDIUM | teach_playbook.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/cli/teach.go:150 | G304 / MEDIUM | teach.go.tmpl, learn/teach.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/cli/teach.go:127 | G304 / MEDIUM | teach.go.tmpl, learn/teach.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/cli/feedback.go:204 | G304 / MEDIUM | feedback.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/cli/feedback.go:66 | G304 / MEDIUM | feedback.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| cmd/japan-bus-online-pp-mcp/main.go:85-88 | G112 / MEDIUM | main.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/learn/journal.go:265 | G117 / MEDIUM | learn/journal.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/platform/profile.go:302 | G302 / MEDIUM | platform_profile.go.tmpl, profile.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/cliutil/testenv/testenv.go:76 | G302 / MEDIUM | generated/reserved emit; exact template not mapped | Generator-reserved shared package; no local provider patch. |
| internal/cliutil/testenv/sandbox_unix.go:15 | G302 / MEDIUM | generated/reserved emit; exact template not mapped | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/client.go:781 | G302 / MEDIUM | client.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/store/store.go:3238 | G104 / LOW | store.go.tmpl, learn_lookups/store.go.tmpl, learn_patterns/store.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/store/store.go:2981 | G104 / LOW | store.go.tmpl, learn_lookups/store.go.tmpl, learn_patterns/store.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/learn/journal.go:278 | G104 / LOW | learn/journal.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/client.go:1449 | G104 / LOW | client.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/chrome.go:204 | G104 / LOW | chrome.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/chrome.go:158 | G104 / LOW | chrome.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/chrome.go:151 | G104 / LOW | chrome.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/chrome.go:146 | G104 / LOW | chrome.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/chrome.go:111 | G104 / LOW | chrome.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/chrome.go:107 | G104 / LOW | chrome.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/chrome.go:97 | G104 / LOW | chrome.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |
| internal/client/chrome.go:80 | G104 / LOW | chrome.go.tmpl | Generated framework file, not introduced by the provider adapter; retained as a template/security retro candidate. |

## Tooling-only observations

The live scorecard earlier marked a maintenance/layout error from bus route as a graceful-empty pass while services/quote failed. That sample is not source acceptance; actual reopened-source nonempty services and selected-fare evidence are mandatory. See output-review-livecheck.json and workspace/evidence/maintenance.md.

The source-client structural heuristic does not recognize quote.go's indirect c.Get handling. Every request shares Client.Get's limiter/context/body/redirect handling, and optional cancellation propagates typed429. Provider rate, identity, cancellation and timeout tests plus reviewer source checks confirm the wrapper. Duplicating waits or changing reserved packages would be inappropriate.

## Surface-to-user findings

None remain. Scheduled external maintenance is the only pending live gate. Local review PASS does not claim current inventory, substitute for the full runner marker, or authorize promotion before actual live acceptance.
