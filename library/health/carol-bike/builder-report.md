# Builder report: CAROL all-family ride regeneration

Date: 2026-08-28 UTC
Printing Press: `4.31.1`

## Repair outcome

The canonical spec declares four concrete, GET-only, zero-based paginated ride-family endpoints: `REHIT`, `FAT_BURN`, `FREE_AND_ZONES_AND_CUSTOM`, and `FITNESS_TESTS`. The CLI exposes all four list commands, and `sync --resources ride` walks the same closed family allowlist while retaining one public resource and one local ride store.

The reviewed contribution's MCP registration exposed only the pre-existing typed `ride_list-rehit` tool. The repair adds typed read-only MCP tools for the other three list families, preserving the same pagination schema and API-version default as the established REHIT tool. Context metadata now lists every ride endpoint and reports the corresponding typed API tool count.

All account-derived workout-history facts have been removed from this public report and patch metadata. The retained evidence is structural or fixture-derived: every configured family is covered, deterministic family fixtures are non-empty, each family starts at page zero, stable IDs deduplicate across families, repeat sync is idempotent, and explicit API-version overrides retain precedence.

## Files changed by the contribution

- `spec.yaml`: added the three ride-family endpoints alongside `list-rehit`.
- `internal/cli/ride_list-fat-burn.go`: generated endpoint command.
- `internal/cli/ride_list-free-custom-zones.go`: generated endpoint command.
- `internal/cli/ride_list-fitness-tests.go`: generated endpoint command.
- `internal/cli/ride.go`: registered the three generated commands.
- `internal/cli/sync.go`: all-family fan-out, family checkpointing, shared ride-ID deduplication, response-path mapping, and the required API-version default.
- `internal/cli/carol_pagination_test.go`: fixture-derived family coverage, independent zero-page pagination, stable-ID deduplication, repeat idempotence, closed targets, and API-version override coverage.
- `internal/cli/root_test.go`: command-tree reachability and exact read-only endpoint annotations.
- `internal/mcp/tools.go`: typed tools for all ride-list families plus updated context metadata.
- `internal/mcp/tools_test.go`: typed ride-tool registration and safety-annotation coverage.
- `README.md` and `SKILL.md`: all-family command/sync documentation and explicit REHIT-only trend scope.
- `.printing-press.json`: source/run metadata and typed API tool count.
- `.printing-press-patches/carol-all-family-ride-sync.json`: durable reprint guard for all-family sync and MCP coverage.
- `.printing-press-patches/carol-zero-based-pagination.json`: privacy-safe, fixture-derived zero-page acceptance language.

`CHANGELOG.md`, `.printing-press-release.json`, the runtime version, auth hardening, platform credential code, module path, MCP intents, and unrelated patch records remain unchanged.

## Verification scope

Verification covers:

- focused CLI family pagination/default/override tests;
- focused MCP typed-tool registration tests;
- formatting, module tidiness, build, vet, and the full Go test suite;
- `govulncheck`;
- Printing Press dogfood, strict skill verification, strict PII audit, tools audit, and shipcheck;
- a built MCP JSON-RPC `tools/list` request proving all four typed list families plus `ride_get-latest` are present;
- generation validation, diff checks, and explicit secret/private-fact scans.

No live CAROL request or account data is required or permitted for this repair verification.

## Side effects and release boundary

This contribution remains local to the feature branch. It does not push, open or merge a pull request, publish a library release, use credentials, contact CAROL, or read or mutate live ride data.
