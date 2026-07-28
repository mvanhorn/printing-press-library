# Printing Press Retro: WooCommerce

## Session Stats
- API: woocommerce (wc/v3 admin + wc/store/v1 public, spec authored from the live WP-REST route index)
- Spec source: live route index (no official OpenAPI exists; best community spec covers 67/166 endpoints)
- Press version: 4.28.0 → 4.29.0 during run
- Scorecard: 95/100 (A) · Verify: 100% · Shipcheck: 7/7 PASS (4 consecutive clean sweeps)
- Fix loops: 2 (duplicate switch case at generate; broken-file resurrection at --force regen)
- Manual code edits to generated files: 6 (learnings.go race, auth.go dead cmd, doctor/client hints, sync.go cursorType, config.go credential precedence, walker.go MCP pruning — last one via polish)
- Features built from scratch: 8 novel commands + storeapi sibling client + snapshot schema (expected workflow)
- Live verification: two production WooCommerce stores (client identities withheld), 537-order and 1,329-order mirrors

## Findings

### F1. MCP cobratree walker prunes hidden subtrees — novel commands vanish from MCP (Bug)
- **What happened:** 5 of 8 novel commands (`orders triage`, `customers ltv`, `catalog audit|watch|diff`) were absent from the MCP surface while advertised in root-help Highlights, README, SKILL, and `.printing-press.json`. 33 tools instead of 38.
- **Scorer correct?** Scorer blind spot, not wrong: dogfood's novel-features check verifies CLI invocability, MCP surface parity passed because endpoint mirrors were correctly hidden. Neither checks MCP reach of novel leaves.
- **Root cause:** `internal/mcp/cobratree/walker.go` `continue`s on `child.Hidden`, pruning the whole subtree. The generator itself (a) marks endpoint-mirror parents Hidden under the >50-endpoint Cloudflare pattern and (b) wires novel scaffolds into those same parents (`orders.go`, `catalog.go`, `customers.go` AddCommand). The machine creates both halves of the collision.
- **Cross-API check:** every large-API print (>50 endpoints ⇒ hidden parents) whose novel features attach to resource parents — the wiring the generator emits by default. The 3 novel commands that survived hung off hand-authored visible parents (`stock`, `revenue`, top-level `refund-rate`), which is why the gap was invisible.
- **Frequency:** subclass: Cloudflare-pattern CLIs with resource-scoped novels — the standard shape for big APIs.
- **Fallback:** agent must know cobra `Hidden` ≠ `mcp:hidden` and hand-patch a DO-NOT-EDIT file that regen/mcp-sync clobbers. Near-zero fallback reliability.
- **Durable fix:** walker treats cobra `Hidden` as CLI-help curation only: skip *registering* a hidden group but still descend. `mcp:hidden` stays the real opt-out. Verified in-place on this CLI: 33→38 tools, no endpoint-mirror leakage, root help still curated.
- **Test:** positive — hidden parent with novel child ⇒ child registered, parent not; negative — `mcp:hidden` parent ⇒ subtree still pruned; endpoint-mirror leaves still classify `commandEndpoint` and skip.
- **Case against filing (Step G):** "polish already fixed it." Fails: the fix lives in a generated DO-NOT-EDIT file and dies on next regen; every future large print re-ships the gap silently.
- **Related prior retros:** #3573 `extends` (cobratree, different bug); #3445 `extends` (novel-command MCP annotations).

### F2. Pagination: `cursorType: offset` paired with `cursorParam: page` → silent single-page sync (Bug)
- **What happened:** `sync` never advanced past page 1. Mirror held 10/40 products; with `per_page=100`, exactly 100/537 orders — offset arithmetic (`0+limit`) sent as a 1-based page number (`?page=10` instead of `?page=2`). No warning; every analytics command computed on a fraction of the store as if complete.
- **Scorer correct?** Gap: dogfood/live matrix can't see under-fetch — sync "completes" successfully.
- **Root cause:** spec authored `pagination.type: offset` (skill-side error), and the generator accepted the self-contradictory pair `cursorParam: page` + `cursorType: offset` without validation, emitting arithmetic that's wrong by construction.
- **Cross-API check:** `page`/`per_page` is the dominant REST idiom (all WP-REST APIs, GitHub-style pagination); any offset-typed spec against a page-numbered param reproduces exact silent data loss.
- **Frequency:** subclass: page-numbered APIs with offset-typed specs. Guard: true offset APIs (`?offset=`) keep current behavior.
- **Durable fix:** spec validation/profiler: `cursorParam` named `page`/`page_number` with `cursorType: offset` is a hard error or auto-correct to `page`. This is the inverse variant of open issue #3538 (cursorParam without cursorType) — same family: profiler emits self-contradictory pagination config, sync silently stores one page.
- **Case against:** "the spec author (skill) wrote offset; garbage in, garbage out." Fails: the machine had both names in hand and emitted arithmetic that cannot be right for `page`; a one-line validation converts silent data corruption into a loud error.
- **Related prior retros:** #3538 `aligned` (comment there, don't file new).

### F3. Config credential precedence: global credentials file silently overrides per-config credentials → cross-tenant key transmission (Bug, security)
- **What happened:** `config.Load` unconditionally cleared any credentials the selected config file carried and applied the single global `credentials.toml`. Running against store B with store A's keys in the global file sent **store A's credentials to store B's server** (confirmed live: 401 from the other tenant's origin, `credentials_location: credentials file` while `base_url` pointed at B). Per-store credentials were unreachable by config at all.
- **Scorer correct?** N/A (no scorer covers multi-config credential routing).
- **Root cause:** generator config template: `LoadCredentials()` result wins over file-config credential fields.
- **Cross-API check:** any per-tenant API where one operator runs multiple accounts: WooCommerce (per-store), Shopify (per-shop), WP Engine (multi-account), ht-ml (multi-site) — the agency use case the commerce CLIs explicitly target.
- **Frequency:** every multi-tenant CLI; latent single-tenant.
- **Durable fix:** config-file credentials take precedence; shared credentials file is fallback only (verified both directions on this CLI). Better still: scope the credentials store by config path or base_url. Note interaction: the existing loose-permissions gate strips file credentials *before* precedence, so per-store configs must be 0600 — correct behavior, worth documenting in the auth template output.
- **Case against:** "multi-store is an edge case; `--home` already isolates." Fails: `--home` only works if the operator knows; the natural path (`--config`/`WOOCOMMERCE_CONFIG`) actively transmits the wrong tenant's keys — that's a leak, not a UX preference.
- **Related prior retros:** #3438 `extends` (credential env-var handling, different failure).

### F4. Pair-credential auth: single-token assumptions across auth command set and