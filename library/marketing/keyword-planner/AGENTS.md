# Keyword Planner source contract

This source implements the approved Google Keyword Planner evidence collector. Preserve the v25 Discovery input, generator manifest, authorship, LICENSE, NOTICE, and documented patches. The durable source target is library/marketing/keyword-planner/; the public catalogue command is keyword-planner-pp-cli.

## Invariants

- Remote collection is limited to generateKeywordIdeas, generateKeywordHistoricalMetrics, and small read-only account/targeting helpers. CLI and MCP entry points must share the curated request and evidence policy. Do not restore a raw customer endpoint or generic authentication bypass during regeneration.
- Load approved GOOGLE_ADS bindings selectively from ~/.env. Access tokens stay in memory. Never print, journal, persist, or copy credential values into this source, the portfolio, manuscripts, or a generic generated credential store.
- The evidence portfolio is ~/.local/share/keyword-planner/snapshots.db and is separate from generated learning/profile state. Commit exact response bytes before interpretation. Preserve errors and incomplete collections; every repeated collection gets a distinct snapshot.
- Preserve signed int64 precision, NULL versus zero, original inputs, variants, targeting, verified currency, and receipt linkage. Open/future returned months stay raw-only. Do not infer missing values, suppression causes, per-seed attribution, separate-country demand from combined geos, or YouTube revenue.
- Portfolio reads and default doctor are offline. Display limits cannot silently limit collection. Authentication and quota failures must not appear as empty success.
- Keep new command implementations in preserved files, use registration hooks, and record durable behavior under .printing-press-patches/. Keep source annotations and runtime metadata consistent. Each novel command declares pp:data-source and an honest MCP read-only annotation.

## Verification

Run focused tests inside this module, then go build ./..., go vet ./..., and go test -count=1 ./... for completed slices. Never run the Printing Press generator repository's full tests. Per-feature tests must assert content, empty cases, mismatching filters, and failure behavior. Keep fixtures deterministic; root owns live credentials, call budget, and delivered-path verification.

The Printing Press phase ledger owns workflow order. Runner-written acceptance is required for delivery; never hand-author a passing marker. Technical checks do not establish maintainer acceptance. Publication, warehouse mutations, service changes, and global skill installation are separate actions.

Read README.md for commands and DATA-CONTRACT.md for semantics. Runtime command truth comes from keyword-planner-pp-cli --help and keyword-planner-pp-cli agent-context --pretty. Local learning/profile commands are optional helpers and must never replace fresh evidence or mix their tables into snapshots.db.
