# Phase 4.95 — Local code review — immovlan-pp-cli (run 20260922-135147-d48976cc)

**Review path chosen:** direct subagent dispatch via the Agent tool — three always-on reviewers (correctness, security, maintainability), re-run in full every round; no conditional reviewers (no API contract, migrations or async paths in the hand-written code).

**Scope:** internal/immovlan/*, internal/store/immovlan_store.go, the 13 hand-written files in internal/cli/ and their tests. Generated files were read-only context.

## Autofix summary

Round 1: 10 correctness (1 P1, 3 P2), 9 security (3 P2), 22 maintainability findings; all in-scope items autofixed in place except the two deferred below. Highlights: `enrich` no longer erases type/price/postcode/private (COALESCE upsert + dataLayer identity), pagination end read from `page=` links instead of a short page (false "gone" in `watch`), enrich resumable (`--retry-after`, oldest reads first), agencies `--epc` semantics, relisted sameHome gate, csvSafe on every CSV path, validated `vlan_code`, `photos` hidden from MCP, no-overwrite photo downloads, symmetric rounding, order-insensitive saved-search change detection, shared helpers and 8 new test functions.
Round 2: 7 correctness (1 P2), 4 security (1 medium), 15 maintainability findings; all fixed except the deferred names below. Highlights: greedy multi-chain relisted grouping with deterministic order, `--plain`/`--quiet` sanitised, EPC/agency-id/postcode/date validated at parse time, `resolveCommune` shared, `matchSameAs`/`applyFindFilters` extracted and tested, envelopes for saved/shortlist/locations, `ListingFilter.MaxPrice`.
Live finding while smoke-testing between rounds: `PhotoHash` hashed the URL's last segment, which on Immovlan is the size (`/Large`), so every listing shared one hash and `relisted` grouped 26 unrelated listings; fixed (file-name segment) with a stored-hash migration (`vlan_meta.photo_hash_version`).
Round 3: 3 correctness (1 medium: the round-2 house-number suffix let street ordinals such as "1er Mai" become the house number; 2 low: photo sub-chain duplicating an address group, sale→rent pairs grouped), 5 security (all low: map keys not sanitised, partial photo left on close error, migration stamped after a partial scan, punctuation-only commune matching empty localities, CDN error bodies echoed), 3 maintainability (dead `csvSanitize`, extraction scaffolding in `relistGroupFrom`, page size literal in two help strings). All fixed in place with regression tests; build, vet and the three package suites pass.

**Convergence outcome:** stopped at the round-3 cap with 0 findings outstanding (every round-3 finding was fixed and covered by a test; no fourth reviewer pass per the cap).

**Post-fix simplification:** `/simplify` skipped — the working tree is not a git repository (no diff to scope it to); the maintainability reviewer ran three full rounds on the same in-scope files instead.

## Deliberately deferred (not defects)

- JSON field names `old_price` / `new_price` / `change_pct` (watch) and `cut_pct` / `cut_eur` (drops) kept for parity with immoweb-pp-cli, which the Hermes job already consumes.
- `printPebTrap` / `printSplit` / `printAgencies` kept as explicit per-command printers (reviewer marked optional).
- Advertiser phone numbers stay in the store (SEC-9 round 1, informational): Arnaud's job needs them to call sellers; they never leave the machine.

## Template-shape retro candidates (generated code, not patched)

- `internal/cli/helpers.go:405-411` — 401 hint says "check your API credentials" and 404 hint says "run the 'list' command" on a no-auth CLI without a `list` command (generator template text).
- `internal/cli/listings_get.go:18` — generated Short truncated mid-sentence ("Returns the schema.").
- `internal/cli/helpers.go` — HTTP 403 mapped to `authErr` (exit 4) for the generic `listings` commands; a public-site CLI has no credentials, exit 5 would be truthful (documented in README/SKILL exit-code tables instead).
- `printPlain` (generated) prints cell values verbatim; hand-written views sanitise before handing over, but generated resource commands do not.
- dogfood flagged 5 dead generated helpers and one generated dead flag (`maxAge`) — generator emissions.

## Out-of-scope retro candidates

None in `internal/cliutil/` or `internal/mcp/cobratree/`.

## Surface-to-user findings

None: no finding required a scope change or a real trade-off.
