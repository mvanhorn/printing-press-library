# Sumble CLI — Build Log

## Built
- Generated full v6 surface from hand-authored internal spec (sumble-v6-spec.yaml):
  organizations (find/enrich/match/intelligence-brief), people (find/find-related-people/enrich),
  postings (find/get/find-related-people), technologies (find), organization-lists, contact-lists.
- 7 hand-authored credit-economy novel commands (internal/cli/): cost-estimate, balance, budget
  (set/show/clear), spend, stale, stack-diff, reconcile. Shared helpers in sumble_credit.go
  (credit cost table, ledger, budget store, org-tech cache).
- MCP transport [stdio, http].

## Notes / deferred
- Resource `jobs` renamed to `postings` (jobs collides with the built-in async-jobs command).
- Nested request bodies (filters, organization) emit as opaque JSON-string flags
  (--filters '{...}', --organization '{...}') rather than dot-flattened leaves. Correct wire
  body confirmed via dry-run. Clean-flag UX lives in the hand-authored novel commands.
- organizations/enrich requires a `filters` object even when empty (API 422 otherwise);
  stack-diff sends filters:{}. Documented in troubleshooting.

## Generator/environment limitations found (retro candidates)
- govulncheck v1.3.0 (latest) cannot analyze go1.26 modules — it forces the go1.25.10 toolchain
  and errors on go1.26 stdlib. The generator emits `go 1.26.3` in go.mod, so the govulncheck
  quality gate fails for environmental reasons on every CLI generated on this machine. go build,
  go vet, go mod tidy all pass. Not a code defect.
- `cli-printing-press generate --force` deletes hand-authored files in emitted dirs
  (internal/cli/*.go) — wiped all 7 novel command files on a re-run to refresh the README from
  corrected research.json. regen-merge is the documented safe-refresh path; --force should warn
  or preserve non-emitted hand-authored files.
