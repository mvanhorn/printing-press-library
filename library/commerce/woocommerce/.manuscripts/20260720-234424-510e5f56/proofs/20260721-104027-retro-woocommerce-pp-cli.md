# Printing Press Retro: WooCommerce

## Session Stats
- API: woocommerce (wc/v3 admin + wc/store/v1 public; spec authored from the live WP-REST route index — no official OpenAPI exists, best community spec covered 67/166 endpoints)
- Spec source: live route index
- Scorecard: 95/100 (A) · Verify: 100% · Shipcheck: 7/7 PASS
- Fix loops: 2 (duplicate switch case at generate; broken-file resurrection at `--force` regen)
- Manual edits to generated files: 6 (learnings.go race, auth.go dead cmd + hint fixes, sync.go cursorType, config.go credential precedence, walker.go MCP pruning via polish, catalog_watch meta.source)
- Features built from scratch: 8 novel commands + storeapi sibling client + snapshot schema (expected workflow)
- Live verification: two production WooCommerce stores (client identities withheld), 537-order and 1,329-order mirrors

## Findings

### F1. MCP cobratree walker prunes hidden subtrees — novel commands vanish from MCP (Bug) → FILE NEW, P1
- **What happened:** 5 of 8 novel commands (`orders triage`, `customers ltv`, `catalog audit|watch|diff`) were absent from the MCP surface while advertised in root-help Highlights, README, SKILL, and `.printing-press.json`. 33 tools instead of 38.
- **Scorer correct?** Blind spot, not wrong: dogfood's novel-features check verifies CLI invocability; MCP surface parity passed because endpoint mirrors were correctly hidden. Neither checks MCP reach of novel leaves.
- **Root cause:** `internal/mcp/cobratree/walker.go` `continue`s on `child.Hidden`, pruning the whole subtree. The generator itself (a) marks endpoint-mirror parents Hidden under the >50-endpoint Cloudflare pattern and (b) wires novel scaffolds into those same parents (`orders.go`, `catalog.go`, `customers.go` AddCommand). The machine creates both halves of the collision.
- **Cross-API check:** every large-API print (>50 endpoints ⇒ hidden parents) whose novel features attach to resource parents — the wiring the generator emits by default. The 3 novel commands that survived hung off hand-authored visible parents (`stock`, `revenue`, top-level `refund-rate`), which is why the gap was invisible.
- **Frequency:** subclass: Cloudflare-pattern CLIs with resource-scoped novels — the standard shape for big APIs. Named with evidence: WooCommerce (this run, 5 missing), and the mechanism is structural (a tree-walk bug in a shared template), not API-specific — any hidden parent with a visible child loses the child from MCP.
- **Fallback:** agent must know cobra `Hidden` ≠ `mcp:hidden` and hand-patch a DO-NOT-EDIT file that regen/mcp-sync clobbers. Near-zero fallback reliability.
- **Durable fix:** walker treats cobra `Hidden` as CLI-help curation only: skip *registering* a hidden group but still descend into it. `mcp:hidden` stays the real opt-out. Verified in-place: 33→38 tools, no endpoint-mirror leakage, root help still curated at 30 entries, `go test ./internal/mcp/...` green.
- **Test:** positive — hidden parent with novel child ⇒ child registered, parent not; negative — `mcp:hidden` parent ⇒ subtree still pruned; endpoint-mirror leaves still classify `commandEndpoint` and skip.
- **Case against filing (Step G):** "polish already fixed it." Fails: the fix lives in a generated DO-NOT-EDIT file and dies on next regen; every future large print re-ships the gap silently.
- **Related prior retros:** #3573 `extends` (cobratree, different bug); #3445 `extends` (novel-command MCP annotations).

### F2. Pagination: `cursorType: offset` paired with `cursorParam: page` → silent single-page sync (Bug) → COMMENT on #3538
- **What happened:** `sync` never advanced past page 1. Mirror held 10/40 products; with `per_page=100`, exactly 100/537 orders — offset arithmetic (`0+limit`) sent as a 1-based page number (`?page=10` instead of `?page=2`). No warning; every analytics command computed on a fraction of the store as if complete.
- **Scorer correct?** Gap: dogfood/live matrix can't see under-fetch — sync "completes" successfully.
- **Root cause:** spec authored `pagination.type: offset` (skill-side error), and the generator accepted the self-contradictory pair `cursorParam: page` + `cursorType: offset` without validation, emitting arithmetic wrong by construction.
- **Cross-API check:** `page`/`per_page` is the dominant REST idiom (all WP-REST APIs, GitHub-style pagination); any offset-typed spec against a page-numbered param reproduces exact silent data loss.
- **Durable fix:** spec validation/profiler — `cursorParam` named `page`/`page_number` with `cursorType: offset` is a hard error or auto-correct to `page`. This is the inverse variant of #3538 (cursorParam WITHOUT cursorType): same family — profiler emits self-contradictory pagination config, sync silently stores one page.
- **Case against:** "spec author (skill) wrote offset; garbage in, garbage out." Fails: the machine had both names in hand and emitted arithmetic that cannot be right for `page`; a one-line validation converts silent data corruption into a loud error.
- **Related prior retros:** #3538 `aligned` — comment there, don't file new.

### F3. Config credential precedence: global credentials file silently overrides per-config credentials → cross-tenant key transmission (Bug, security) → FILE NEW, P1
- **What happened:** `config.Load` unconditionally cleared any credentials the selected config file carried and applied the single global `credentials.toml`. Running against store B with store A's keys in the global file sent **store A's credentials to store B's server** (confirmed live: 401 from the other tenant's origin, `credentials_location: credentials file` while `base_url` pointed at B). Per-store credentials were unreachable by config at all.
- **Scorer correct?** N/A — no scorer covers multi-config credential routing.
- **Root cause:** generator config template — `LoadCredentials()` result wins over file-config credential fields.
- **Cross-API check:** any per-tenant API where one operator runs multiple accounts: WooCommerce (per-store, this run), Shopify (per-shop, in library), WP Engine (multi-account, in library) — the agency use case the commerce CLIs explicitly target.
- **Frequency:** every multi-tenant CLI; latent single-tenant.
- **Counter-check (Step C):** would config-first precedence hurt a single-tenant CLI? No — a CLI with only the global file and no config-file credentials falls through to the same global path. The only behavior it changes is the leak.
- **Durable fix:** config-file credentials take precedence; shared credentials file is fallback only (verified both directions). Better still: scope the credentials store by config path or base_url. Interaction: the existing loose-permissions gate strips file credentials *before* precedence, so per-store configs must be 0600 — correct behavior, worth surfacing in the auth template output.
- **Case against:** "multi-store is an edge case; `--home` already isolates." Fails: `--home` only works if the operator knows it; the natural path (`--config`/`WOOCOMMERCE_CONFIG`) actively transmits the wrong tenant's keys — a leak, not a UX preference.
- **Related prior retros:** #3438 `extends` (credential env-var handling, different failure).

### F4. `auth set-token` emitted and doc-referenced but unregistered for pair-credential specs (Bug) → COMMENT on #3550
- **What happened:** dogfood flagged `1 unregistered commands: set-token`. WooCommerce uses a consumer key + secret PAIR (HTTP Basic), so the generator correctly registered `auth set-credentials` but still emitted `newAuthSetTokenCmd` (never wired) plus three user-facing hints pointing at the nonexistent command (`doctor.go` ×2, `client.go` ×1).
- **Root cause:** generator emits the single-token auth command unconditionally, even when the resolved auth shape is a credential pair.
- **Durable fix:** skip emitting `newAuthSetTokenCmd` and its doc hints when auth is a credential pair; register only `auth set-credentials`. This is exactly open issue #3550 (`auth set-token implemented and doc-referenced but unregistered`) with a pair-credential reproduction.
- **Case against:** "already fixed by removing dead code in this CLI." Fails: printed-CLI removal doesn't stop the next pair-credential print from re-emitting it.
- **Related prior retros:** #3550 `aligned` — comment there, don't file new.

### F5. Learn-loop synonym registration data race in generated code (Bug) → FILE NEW, P2
- **What happened:** the generated concurrency test `TestPlaybookInit_ConcurrentSafe` fails under `-race` with `fatal error: concurrent map iteration and map write`. `RegisterQuerySynonyms` writes the package-level `querySynonyms` map while `compileQuerySynonyms` iterates it.
- **Scorer correct?** Gap: no gate runs the generated `go test` suite (this is the concrete instance of open #3456), so the race ships green.
- **Root cause:** `internal/store/learnings.go` template — the synonym registration path mutates a package-global map with no lock while a reader iterates it. Ships in every CLI whose spec declares `learn.synonyms`, and the failing test is emitted for every CLI regardless.
- **Cross-API check:** the learn loop is default-on for every print; the racy code and its failing test are byte-identical across CLIs — not API-specific at all. Named with evidence: WooCommerce (this run, reproduced under `-race` with and without my fix to confirm it's the template).
- **Durable fix:** guard `querySynonyms` / `querySynonymRules` with a `sync.RWMutex` (write lock in `RegisterQuerySynonyms`, read lock at the `foldQueryTokens` read site). Verified: race clears, `-count=5 -race` green.
- **Case against:** "startup is single-threaded in practice." Fails: the generator itself ships a test that spawns concurrent registration and fails under `-race`; codegen must not emit a test its own output can't pass.
- **Related prior retros:** #3456 `extends` (no gate runs the generated test suite — this is a concrete instance).

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|----------------------|------------|--------|
| F1 | MCP walker prunes hidden subtrees; novel commands vanish from MCP | generator | subclass: Cloudflare-pattern + resource-scoped novels | near-zero (DO-NOT-EDIT file, clobbered by regen) | small | skip register on hidden, still descend; `mcp:hidden` still prunes |
| F3 | Global credentials override per-config credentials (cross-tenant leak) | generator | every multi-tenant CLI | low (natural path leaks) | small | config-first only when file carries credentials; else unchanged |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|----------------------|------------|--------|
| F5 | Learn-loop synonym registration data race in generated code | generator | every CLI with the learn loop | low (no gate runs generated tests) | small | RWMutex; no behavior change single-threaded |

### Comment on existing issues
| Finding | Title | Existing issue | Why |
|---------|-------|----------------|-----|
| F2 | Pagination offset/page silent single-page sync | #3538 | Same profiler family (self-contradictory pagination config → one-page sync); adds page-numbered-param reproduction |
| F4 | `auth set-token` unregistered for pair-credential specs | #3550 | Exactly that issue with a credential-pair reproduction |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| `example.com` base_url rejected before other validation | RFC2606 guard fired first with no other signal | working-as-designed; correct guard |
| `--force` merge resurrected a broken `sync.go` | AST merge preferred a non-compiling preserved file | printed-CLI iteration artifact; already recoverable by wiping output |
| duplicate `variations` switch case at generate | two syncable list endpoints under one resource | folded into F2's profiler family; per-CLI worked around by splitting resource |
| credential tests assume single token (`internal/cliutil`) | generated test template expects one credential | overlaps #3438 territory; folded into F4 comment as a note, not filed separately |

## Work Units

### WU-1: MCP cobratree walker must descend into hidden groups (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Hidden endpoint-mirror parents no longer prune novel child commands from the MCP surface.
- **Target:** `internal/mcp/cobratree/walker.go` (walk function's `child.Hidden` branch).
- **Acceptance criteria:**
  - positive: a hidden parent with a non-hidden novel child ⇒ child registered as an MCP tool, parent not registered.
  - negative: a parent annotated `mcp:hidden` ⇒ entire subtree still pruned; endpoint-mirror leaves still classify `commandEndpoint` and are skipped.
- **Scope boundary:** does not change root-help curation or `mcp:hidden` semantics.
- **Complexity:** small

### WU-2: Config-file credentials take precedence over the shared credentials file (from F3)
- **Priority:** P1
- **Component:** generator
- **Goal:** A config file that carries its own credentials is authoritative; the shared global file is fallback only — eliminating cross-tenant key transmission.
- **Target:** generator config template (`internal/config/config.go` `Load`).
- **Acceptance criteria:**
  - positive: 0600 config with its own `consumer_key`/`consumer_secret` ⇒ `credentials_location: config file`, and the config's keys are the ones sent.
  - negative: config without credentials ⇒ still falls back to the shared credentials file.
- **Scope boundary:** does not alter the loose-permissions gate (which still strips file credentials before precedence — document that per-store configs must be 0600).
- **Complexity:** small

### WU-3: Guard the learn-loop synonym maps against concurrent access (from F5)
- **Priority:** P2
- **Component:** generator
- **Goal:** The generated `learnings.go` no longer races under `-race`, so the emitted concurrency test passes.
- **Target:** generator learn template (`internal/store/learnings.go`).
- **Acceptance criteria:**
  - positive: `go test ./internal/store/ -run Concurrent -race -count=5` green on a fresh print with seeded `learn.synonyms`.
  - negative: single-threaded synonym folding behavior unchanged.
- **Scope boundary:** locking only; no change to synonym semantics.
- **Dependencies:** reinforces #3456 (run the generated test suite in a gate) but is independently shippable.
- **Complexity:** small

## Anti-patterns
- Advertising a capability (novel command) in help/README/SKILL/manifest while it is silently absent from the MCP surface — the parity check must cover reach, not just existence.
- Emitting a `go test` that the generator's own output cannot pass under `-race`.
- Letting `sync` report success while storing one page of a multi-page resource.

## What the Printing Press Got Right
- The dual-namespace WooCommerce shape (authenticated `wc/v3` + public `wc/store/v1`) fell straight out of per-endpoint `no_auth: true` — the zero-credential catalog commands worked on first generation.
- Basic-auth pair emission (`Basic {key}:{secret}` → base64) was correct end-to-end with no hand-editing.
- The novel-feature scaffolds (correct names, flags, `mcp:read-only` annotations, verify-friendly RunE) made the 8 hand-built commands a body-fill exercise, not a from-scratch build.
- The `catalog_snapshots` hand-authored migration in its own file survived `--force` regen exactly as the durability contract promises.
