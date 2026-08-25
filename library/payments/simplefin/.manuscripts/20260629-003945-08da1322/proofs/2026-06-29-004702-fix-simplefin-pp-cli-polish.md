# SimpleFIN CLI Polish

Verdict: ship. further_polish_recommended: no.

| Metric | Before | After |
|---|---|---|
| Scorecard | 85 | 85 (Grade A) |
| Verify | 100% | 100% |
| go vet | 0 | 0 |
| gosec (hand-authored) | 16 | 0 |
| tools-audit | 0 pending | 0 pending |
| Live matrix | exercised | exercised |

Fixes: cleared 15 gosec G104 unhandled-error findings in hand code via explicit `_ =` on cleanup Close() calls; added narrow `#nosec G304` rationale on loadRulesFile (user-supplied --rules path).

Output review (Phase 4.85): PASS, no findings.

Retro candidates (generated-file, not polish-ownable): gosec G119 redirect-header + G201 SQL formatting in client.go/store.go; dead helpers in helpers.go; generic vs hand-authored sync/export command duplication; info->get naming in promoted_info.go. All routed to /printing-press-retro.
