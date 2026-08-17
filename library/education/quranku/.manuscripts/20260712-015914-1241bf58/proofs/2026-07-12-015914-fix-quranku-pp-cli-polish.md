# QuranKu CLI — Polish (Phase 5.5)

ship_recommendation: ship | further_polish_recommended: no
Scorecard 77 (unchanged) | Verify 100% | Dogfood PASS | go vet 0 | gosec (authored) 2->0 | tools-audit 2->0 pending

Fixes:
- gosec G404 random.go: narrow #nosec (non-crypto verse selection, math/rand correct)
- gosec G301 quranku_corpus.go: data-dir perms 0o755 -> 0o700
- MCP Short rewrite: bookmark list, bookmark rm (agent-grade, scope/param aware)

Remaining: none actionable (generated-file gosec = generator retro candidates; scorecard sub-metrics structural for a small fixed corpus).
