# Polish Report: open-meteo

## Before → After
- Scorecard: 77/100 (unchanged; 2 structural dims documented)
- Verify: 96.9% (WARN, 1 critical) → 100% (PASS, 30/30)
- Live matrix (dogfood --live full): FAIL 28/72 → PASS 83/83, exercised
- verify-skill: FAIL (canonical-sections) → PASS
- publish validate: FAIL → PASS (all 11 checks)
- tools-audit: 1 finding → 0
- gosec hand-authored: 3 findings → 0 (31 remaining all in generator-emitted files → retro candidates)
- pii-audit: 0 findings
- go vet: 0 issues

## Fixes applied
1. 7x `// pp:data-source` annotations on novel commands (forecast diff, compare, is-good-for, normals, panel, accuracy, weather-mix)
2. Description drift: spec.yaml cli_description + root.go Short/Long + manifest.json + .printing-press.json → narrative headline
3. SKILL.md install section restored to canonical text (verify-skill canonical-sections)
4. .printing-press.json schema_version 1 → 2 (publish contract); installed gh + authenticated
5. Removed `import` command (read-only API; no writable resources; generator strips it in 4.30.1)
6. version command Short enriched (tools-audit thin-short)
7. snapshot.go gosec: MkdirAll 0700, WriteFile 0600, #nosec G304 (hashed cache keys)
8. Live dogfood fixes: realistic required-flag Examples on 11 endpoint commands; Examples added to doctor/feedback/profile; sync JSON envelope routed to stdout (machine mode); is-good-for pp:happy-args `<activity>=surfing;--place=Seattle` (v4.30.1 requires angle-bracket positional labels)
9. phase5 acceptance marker written (live full matrix, 83/83)

## Skipped findings (structural)
- dogfood WARN `defaultSyncResources empty`: Open-Meteo has zero syncable resources (every endpoint requires coordinates); novel commands are computed/live per data-source annotations. Sync is a no-op by design; the WARN is honest, not a defect.
- scorecard `path_validity 0/10`: multi-host API — generated commands emit absolute URLs (https://api.open-meteo.com/...); scorer matches segment-wise against relative spec paths. Generator limitation, present in 4.30.1 fresh trees too.
- scorecard `mcp_token_efficiency 4/10`: scorer slices the last NewTool() to EOF, swallowing shared handler code (context tool chunk = 6106 chars of which mostly makeAPIHandler). Approximation artifact, not description bloat.
- scorecard `live_api_verification N/A`: dim keys off verify mock-mode; CLI is live-verified via dogfood --live (83 tests).
- gosec 31 findings: all in generator-emitted files (internal/store, config, client, cache, mcp, cliutil) → Printing Press retro candidates.

## Ship recommendation: ship
All hard gates pass: verify 100%, scorecard 77, verify-skill 0, workflow-pass, publish-validate passed, gosec 0 hand-authored, tools-audit 0, pii-audit 0, live matrix exercised 83/83.
