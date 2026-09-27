# Agent guide

This is a read-only Rakuten Travel CLI built with Printing Press. The focused commands live in `internal/cli`; HTTP, query validation and source parsing live in `internal/travel`.

## Work

Read [docs/data-contract.md](docs/data-contract.md) before changing identity, occupancy, pricing, freshness or failure interpretation. Check command help for flags rather than copying an independent flag inventory into docs.

Keep the domain/client seam testable with injected HTTP and time. Source calls have request, byte, timeout and retry bounds; new call paths must share them. A failed source request stays an error through comparison. Browser discovery is a maintenance tool; runtime requests use HTTP.

Use `go test -count=1 ./internal/travel ./internal/cli` for changes to the focused behavior, followed by the full build/test/vet checks before acceptance. Live tests must use current dates, bounded requests and semantic assertions; source prices are not fixed expectations. Keep captured account credentials/cookies out of fixtures and evidence.

## Generated code

Preserve narrow hand-authored command files. Each novel command declares `// pp:data-source live` or its actual strategy and matching Cobra annotations; reject incompatible source modes. Keep dry-run before I/O, and pass a bounded command context into the domain client.

Record durable code customizations under `.printing-press-patches/` using schema version 2, with the generator version/run ID from `.printing-press.json`. Describe the behavioral contract a reprint must preserve. Documentation edits need no patch record. Keep generator-reserved `internal/cliutil` and `internal/mcp/cobratree` unchanged; report findings there to the orchestrator.

Preserve creator attribution and release-ledger files. Keep shared agent configuration unchanged. Publishing uses the Printing Press publish workflow; release versions and catalog mirrors are owned by the library automation.

Use [SKILL.md](SKILL.md) to operate the CLI and [README.md](README.md) to build it.
