# Jalan CLI maintenance

The accommodation surface is `stay`; inspect its help for the current command contract. Read [SKILL.md](SKILL.md) when using the CLI for trip discovery, and [docs/verification.md](docs/verification.md) when changing source parsing or claims of coverage.

Preserve these invariants:

- Property, room and plan IDs are distinct strings with leading zeroes.
- Japanese source text and canonical URLs ground extracted facts. Unknown is explicit; absence is not false.
- Room bath facts stay separate from property facilities. Private use, reservability, outdoor placement and hot-spring water are independent.
- Quote units, party, nights, child categories, fees, conditional discounts and points remain distinct. Failures never become zero prices or sold-out claims.
- Inventory freshness uses observation timestamps. Cache reuse is explicit. Requests, pages, retries, comparison width and command time are bounded.
- Accommodation stdout is compact JSON; diagnostics use stderr. Keep shared freshness/coverage metadata under field selection.

`internal/jalan` owns parsing and bounded source access. `internal/cli/stay*.go` owns command behavior. Parser fixtures are focused public excerpts; live E2E evidence must distinguish source-verified behavior from fixtures. Run affected tests during edits, then `go test -count=1 ./...`, `go vet ./...` and the live verification runner for source-contract changes.

Printing Press generated the surrounding framework. Treat `internal/cliutil` and `internal/mcp/cobratree` as generated framework. Prefer domain-layer fixes; a correctness repair needed for publication may patch the emitted copy when protected by a durable customization record and regression tests. Keep shared generator code unchanged. Preserve run receipts, source evidence and custom behavior when reprinting. Use the Printing Press publish workflow for library publication. Shared configuration changes require explicit authorization.
