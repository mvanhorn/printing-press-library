# Phase 4.95 — Local Code Review Findings (google-play-pp-cli)

Review path: two parallel general-purpose reviewer subagents (correctness+security on hand-authored Go; README/SKILL/AGENTS correctness audit covering Phase 4.8 + 4.9). No working-dir review skill matched; subagent dispatch is the universal fallback.

## Autofixed in-place
- **compare.go: rate-limit swallowed on all-failed path** (MEDIUM). When every fetch failed, returned a generic apiErr (exit 5), discarding a typed `*cliutil.RateLimitError`. Fixed to prefer surfacing the rate-limit (exit 7) via classifyGplayErr, else the first typed error.
- **similar.go: dead code** (LOW). Removed unused `findClusterApps` + `minInt` (superseded by `parseClusterGridApp` + inline path probing).
- **app.go: redundant nil-guard** (LOW). `ContainsAds` simplified to `root.path(48).bool()` (bool() already returns false for a nil node).
- **README.md/SKILL.md/research.json: troubleshoot recipe** (ERROR). Changed `sync --resources charts` (invalid — only `categories` is a sync resource) to `top --collection TOP_FREE --category GAME` twice, which is what actually writes chart snapshots.
- **README.md/SKILL.md/research.json: compare --select paths** (ERROR). `--select appId,score,...` → `items.appId,items.score,...` to match compare's nested `items` envelope. Verified the corrected paths resolve against live output.
- **README.md/SKILL.md/research.json: rate-limit prose** (WARNING). "about one request per second" → "about two requests per second (set with --rate-limit)" to match the actual `--rate-limit` default of 2.

## Verified clean (no action)
- SQL: all 11 queries parameterized; no concatenation; NULL columns read via sql.Null*; rows.Err() checked; tx rollback/commit + stmt.Close deferred.
- compare.go fan-out: buffered results channel, concurrency cap 4, dedicated close goroutine, all results consumed by index. No deadlock/leak.
- SSRF/URL building: url.Values.Set for HTML endpoints; `%q` Go-quoting for batchexecute inner payloads; baseURL is a fixed constant. Injection-safe.
- Unbounded reads: io.LimitReader on both transports (12 MiB HTML, 16 MiB batchexecute).
- Rate limiting: typed RateLimitError on exhausted 429/503 + PlayGatewayError-in-200 sentinel; all single-fetch commands route through classifyGplayErr (exit 7).
- Resource leaks: resp.Body.Close on all paths; store.Close deferred after error-checked open.
- Context propagation: live commands wrap cmd.Context() with boundCtx before sibling-client calls; local-only commands correctly use cmd.Context() directly.

## Retro candidates (template-shape — NOT patched per Phase 4.95 escape hatch; surfaced to user)
- **Generated `import` command + `--idempotent` persistent flag on a read-only no-auth CLI** (ERROR per audit). These are generator framework scaffolding emitted for every printed CLI; `import` issues POST/create calls which a public-store scraper has no endpoint for. Patching the printed CLI would hide the machine issue and diverge from the rest of the library. Filed as a retro candidate: the generator should suppress write-oriented scaffolding (import, --idempotent) when the spec is read-only / auth:none. Not advertised in README/SKILL, so low user-facing risk.

## Convergence
In-scope findings cleared in 1 round. No `/simplify` pass needed (autofix output was minimal and localized).
