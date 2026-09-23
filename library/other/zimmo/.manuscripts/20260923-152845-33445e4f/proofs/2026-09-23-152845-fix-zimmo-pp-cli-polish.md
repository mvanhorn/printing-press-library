# Polish (printing-press-polish, forked) — zimmo-pp-cli
Scorecard 85 → 90; verify 98% → 98%; dogfood WARN → PASS; gosec (hand-written) 4 → 0; tools-audit 2 → 0 pending; go vet/test, verify-skill, workflow-verify, pii-audit clean.
Fixes: removed 5 dead generated helpers; LIMIT bound parameter + reasoned #nosec G202 on fixed-column WHERE; resp.Body.Close handled; token cache path cleaned (#nosec G304); photo dir 0750; peb-trap / yield Cobra Shorts match behavior; gofmt which.go.
Skipped (generator retro candidates): tools-manifest.json strips per-operation servers (geo-api/score-api) to relative paths (verify "resource-path:export" critical; no code reads the manifest); gosec in generated files; cache_freshness 5/10, path_validity 5/10 (no health_check_path for OpenAPI); dogfood which.go re-sync not gofmt'd.
Side effect: polish's smoke test stored 2 listings in ~/.local/share/zimmo-pp-cli/data.db.
ship_recommendation: ship
