# NAVITIME CLI maintenance

This is a Printing Press CLI. Keep API integrations limited to NAVITIME and preserve its generated provenance. Shared agent/tool configuration is outside this project.

Read [docs/data-contract.md](docs/data-contract.md) before changing time, fare, pass or route-normalization logic. Read [docs/access.md](docs/access.md) when changing source access or transport. Read [SKILL.md](SKILL.md) for the agent workflow; use command help for current flags.

Preserve leading-zero IDs, explicit nulls, Japan-local dates, fare groups, source caveats and freshness. Validate source effective parameters. Challenge pages must fail, and displayed pass-filtered fares must not become inferred pass-holder costs.

Keep pure parsing/comparison tests deterministic and content-asserting. Use read-only real journeys for provider acceptance. Check the relevant package first, then `go test -count=1 ./...`, `go vet ./...` and `go build ./...` before acceptance. Keep live requests and output bounded; measurement scripts use isolated cache directories.

Novel Cobra commands keep the generated dry-run guard before file/network access and input validation. Use generated projection helpers, `boundCtx`, read-only annotations and explicit `pp:data-source` strategies. Sibling HTTP clients use `cliutil.AdaptiveLimiter` and typed rate-limit errors. Keep diagnostics on stderr and JSON on stdout.

Record durable code customizations in `.printing-press-patches/` as schema_version 2 records naming behavior, reason and affected files. Preserve release-ledger versions; the public library assigns releases after merge. Root owns orchestration and acceptance; implementation/test delegation follows the user's model instructions.

The optional MCP main uses `RegisterFocusedTools` to exclude unsupported sync tools and replace generic context. Preserve this wrapper and its main wiring when regenerating; its registration test defines the supported tool surface.
