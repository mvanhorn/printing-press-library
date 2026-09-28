Local verifier repairs — root reviewed

Base Printing Press4.32.5; workspace binary4.32.5+local.verifier-fixes. Original installed executable and shared tool configuration remain unchanged. Same run, receipts, spec and artifacts retained.

1. Parse workflow JSON only from stdout; retain stderr separately and use both streams for failure classification.
2. Resolve skill paths/flags/positionals against the built local command tree; retain static fallback and reject bogus flags/paths.
3. Explicit PRINTING_PRESS_LOCAL_DELIVERY=1 requires exact truthful local build/version/path prerequisites instead of unpublished registry installation. Default public contract unchanged.
4. Empty tail registry only qualifies with strict AST proof of literal empty declarations; missing/dynamic/nonempty contracts and export/import remain enforced.

Tests:27Python resolver tests; targeted Go workflow/canonical/bundle tests;9resource-contracttests+19subtests. Root reviewed source and negative cases before using this binary for acceptance.

Changed files:
- scripts/verify-skill/test_runtime_command_tree.py
- scripts/verify-skill/verify_skill.py
- internal/pipeline/workflow_manifest.go
- internal/pipeline/workflow_verify.go
- internal/pipeline/runtime_resource_paths_test.go
- internal/pipeline/runtime_resource_paths.go
- internal/pipeline/workflow_verify_test.go
- internal/cli/verify_skill_bundled.py
- internal/cli/verify_skill_local_delivery_test.go
- internal/cli/verify_skill.go
- internal/cli/verify_skill_local_delivery.go

Binary SHA256: 005d0b49daf8a3dd2e3e2e021b601a343bab325df800ad2a107df6c8e7d9ab7f
