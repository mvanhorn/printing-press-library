# Japan Guide CLI — targeted post-acceptance fix verification

**Result: PASS — no new application finding.** This verifies the small dry-run/metadata changes recorded in `evidence/review-fixes.json.post_acceptance_fixes` after phase-17 convergence. It is not an additional source-parser review round. The same sole MAX reviewer performed the check without source edits or additional agents.

All eleven current hashes match the stable review-fixes snapshot. Compared `internal/cli/guide.go` against the isolated copy of the round-3 source: the change is limited to preserving/wrapping the hidden foundation command, using the flat dry-run helper for inspection, and declaring inspection's external method GET. Source parsing, cache-save error typing, fallback exclusion and comparison limits are unchanged.

## Checks

- **Foundation annotations preserved.** The generated source command has a non-nil annotation map and callable RunE. The hook now adds mcp:hidden rather than replacing its pp:endpoint, pp:method GET, pp:path and existing metadata. The saved original handler remains the non-dry-run path; no generated file was patched.
- **Source and inspection dry-runs are valid simulations.** Independent rebuilt-binary calls return exit 0 and exactly one flat JSON object with dry_run:true, action and would. Inspection was called with cache, an isolated cache-dir and agent mode; its snapshot directory was not created. Source dry-run used JSON mode. Neither emitted stderr or an additional JSON document.
- **No source request.** Independently ran only the relevant changed CLI tests: TestGuideDryRunJSONDoesNotRequestSource and TestGuideDryRunsAndUsageAreNetworkFree. They passed, with the mock transport observing zero requests on source and inspection dry-run paths. No broad test suite was repeated.
- **MCP exposure/classification unchanged.** The actual rebuilt tools/list response contains 23 public tools, no source_directory, seven guide tools, guide_inspect.readOnlyHint:false and true for the other six guide tools. Adding pp:method GET therefore did not restore an incorrect read-only hint or expose the foundation endpoint.
- **Metadata is truthful.** GET describes external Japan Guide acquisition; the optional local snapshot write still has conservative MCP may-write semantics. The dry-run branch precedes acquisition and cache setup.
- **Documentation matches the exception.** README and SKILL explicitly describe inspection's machine-format dry-run simulation object rather than fetched facts. Their normal agent envelope guidance and source-fact contracts remain applicable to live/offline fact results.
- **Fixture invocation mistake separated from source behavior.** The full-runner NO_LEARN setting suppressed the generated teaching-fixture output by design. Unsetting it for those local fixtures is a runner invocation correction, not an external operation or a change to source extraction. Reviewer catalog/dry-run probes deliberately disabled learning; they did not invoke teaching fixtures.

Independent concise proof is saved in `evidence/reviewer-acceptance-fixes-probes.json`. It contains catalog hints, the two simulation objects and snapshot-directory absence only. No raw HTML, credentials, source requests, accounts, bookings, messages or publication actions were involved.

The prior phase-17 application convergence PASS remains valid. Full phase-18 runner completion is the builder's separate acceptance responsibility; this report does not claim the still-running matrix has completed.
