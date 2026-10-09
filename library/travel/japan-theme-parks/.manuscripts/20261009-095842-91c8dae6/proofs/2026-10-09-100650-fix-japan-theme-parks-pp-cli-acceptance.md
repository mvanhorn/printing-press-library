Acceptance Report: japan-theme-parks
  Level: Full Dogfood (live, source pacing kept)
  Tests: 95/95 mandatory passed (80 more skipped by the runner by design:
    no positional example, mutating command without --allow-destructive,
    non-id positional, blocked fixture)
  Passed by kind: help 32, happy_path 21, json_fidelity 21, dry_run_json 13, error_path 8
  Failures: none (final run)
  Fix loops: 2
    - Loop 1: `source parks --dry-run --json` gave the provenance envelope
      without a top-level dry_run:true (json_fidelity "missing dry_run:true").
    - Loop 2: `source parks` / `source queue-times` dry-run JSON had no
      `action` ("empty dry-run action").
  Fixes applied: 1 (final form)
    - CLI fix (generated handlers internal/cli/source_parks.go and
      internal/cli/source_queue-times.go): when the client returns the
      dry-run sentinel, call writeDryRun with action "GET <path>", so JSON
      is {"dry_run":true,"action":"GET /parks.json","would":...}. The
      interim wrapWithProvenance change was reverted.
  Printing Press issues: 1
    - Generated GET handlers wrap the client dry-run sentinel in the
      results/meta provenance envelope. The live dogfood dry-run contract
      needs top-level dry_run:true and a non-empty action. The generator
      template should short-circuit the sentinel through writeDryRun.
  Other checks: go vet clean; go test ./... all packages ok.
  Gate: PASS

Re-run after Phase 19 polish edits (source fingerprint refresh): Full live
dogfood 95/95 mandatory passed, 80 skipped by design, status pass.
