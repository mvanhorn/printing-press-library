# Local source review

Review path: root agent directly, as explicitly required by the user; correctness, security, maintainability, data contract and performance reviewed together. Generator-reserved packages were left unchanged.

The stay command seam applies `boundCtx` before all live sibling-client requests. The client adds a 60s cap, 20s request timeout, two-slot limit, source rate limiter, bounded retries, same-origin HTTPS redirects and response limits. Inputs validate IDs, date horizon, per-room occupancy and applicable filters. Search paging maps into native 30-row pages; offers expose observed tuple counts separately from source plan counts. Cache reuse is explicit and timestamp preserving. Exact plan identity and source query echoes prevent substituting generic/reference prices. Comparison failures survive in typed error/partial envelopes; unknown prices never become zero.

Root reviewed source extraction, cache/output boundaries and the consequence-focused tests. Earlier review fixes are in the build/shipcheck logs and current tests (identity, whole-stay quote scope, source echo, logical paging, explicit failures, cache home, Japanese aliases and encoding). No additional source defect was established in this pass. Live source-variant diagnosis remains in acceptance evidence, not an inferred successful observation.

Autofix summary: no new code edits in this review round; earlier integrated fixes were rechecked. Findings cleared at round 1.

## Template retro candidates (recorded locally)

- Installed Printing Press workflow runner ignores the separate workflow-step `args` field and checks expected fields only at the top-level. The local manifest uses inline flags and envelope assertions; the independent E2E runner verifies nested facts. No generator source was modified.
- Dogfood renders an unsupported exclusivity superlative and repeated rationales in Unique Features/Capabilities. Final docs remove that boilerplate after the last sync. Local build instructions live outside the overwritten Quick Start.
- Verify-skill requires the canonical registry installer block even for an unpublished local delivery. The block is clearly scoped to a future published release; local build instructions are authoritative.

No external issue or PR was filed, and no shared configuration was changed.
