---
title: "fix: Retry instacart add on notFoundBasketProduct and surface actionable guidance"
type: fix
status: completed
date: 2026-04-19
---

# fix: Retry instacart add on notFoundBasketProduct and surface actionable guidance

## Overview

`instacart add <retailer> <query>` sometimes fails with `Instacart rejected the add: notFoundBasketProduct` even when the same retailer search returns valid items. A follow-up `add --item-id <id>` against the same retailer succeeds. The top pick from the live three-call chain is occasionally not addable at the active cart's shop, and we give up after the first try.

This plan narrows the gap: when `UpdateCartItemsMutation` returns `notFoundBasketProduct`, walk the next ranked candidates before surfacing an error, and replace the terse error message with actionable guidance. Adds the first automated tests for the live `add` mutation surface.

## Problem Frame

Observed today (2026-04-19) at Costco with query `strawberries`:

1. First `instacart-pp-cli add costco "strawberries"` returned exit code 5, `notFoundBasketProduct`.
2. `instacart-pp-cli search strawberries --store costco --json` returned six valid items, top hit `items_1576-18876359` "Strawberries, 2 lbs".
3. `instacart-pp-cli add --item-id items_1576-18876359 costco` succeeded against the same cart (id `757109404`).
4. A second `add costco "strawberries" --no-history --dry-run --json` then chose the same `items_1576-18876359` that just worked.

Root causes, from tracing `library/commerce/instacart/internal/cli/add.go` and `library/commerce/instacart/internal/gql/search.go`:

- Autosuggest nondeterminism. `gql.ResolveProducts` generates a fresh `autosuggestionSessionId` UUID per call (`search.go:63`). The server can reorder suggestions between calls, and `rerank` can still place an item at results[0] that is not addable at the active cart's shop.
- Shop / location drift. `ensureInventoryToken` caches the retailer inventory token for six hours (`search.go:29`) tied to the first shop that served a bootstrap. The active cart may have been created against a different Costco warehouse. Evidence: the observed cart contained `items_18148-25502282` (Dino Nuggets) alongside five `items_1576-...` items, so the cart has historically accepted items from multiple location prefixes but not every `1576` product is stocked at every shop the cart routes through.
- Transient out-of-stock. The `Items` lookup returns a name and id without a reliable "addable right now at this shop" flag, so stock-outs surface only at mutation time.

Today's behavior: single attempt, opaque error. The user has no idea that `search` + `--item-id` would have worked, or that the next candidate in the autosuggest list is likely addable.

## Requirements Trace

- R1. When `UpdateCartItemsMutation` returns `notFoundBasketProduct`, the CLI retries with the next ranked candidate(s) from the already-fetched result set before returning an error.
- R2. Retry count, attempted item ids, and the resolver path are visible in `--json` output so agents can see which candidate landed.
- R3. When all candidates exhaust, the CLI surfaces guidance that names the concrete next step (`search` then `add --item-id`, or `--no-history`).
- R4. The retry path is covered by unit tests that exercise the mutation loop without a live network.
- R5. The history-first resolver path still writes back on successful adds, regardless of which candidate succeeded.
- R6. Change ships as a PR to `main` (never a direct push), with release notes in the PR description.

## Scope Boundaries

- Not changing `gql.ResolveProducts` ordering, rerank weights, or the autosuggest sanitization.
- Not filtering candidates by active-cart location prefix. The cart has been observed mixing prefixes, so prefix filtering would drop valid adds; retry is cheaper and covers more cases.
- Not broadening the retryable error set beyond `notFoundBasketProduct` in this change. Other `ErrorType` values (for example `soldOut`, `unavailable`) are out of scope here and noted as a follow-up.
- Not adding live integration tests against Instacart's API. Unit tests around a mockable mutation seam only.
- Not changing the `--item-id` direct path. When a user supplies an id, we still make exactly one attempt.
- Not restructuring the `add` command's argument parsing or the history-first resolver.

## Context & Research

### Relevant Code and Patterns

- `library/commerce/instacart/internal/cli/add.go`, the single file that owns the live `add` flow. `newAddCmd` at line 25, live resolver branch at lines 94-109, mutation call at lines 156-178, `resolveActiveCartID` at line 337.
- `library/commerce/instacart/internal/gql/search.go`, `ResolveProducts` (line 40). Already returns up to `limit` ranked results; today only `results[0]` is used in `add.go:108`.
- `library/commerce/instacart/internal/instacart/` (types package). `UpdateCartItemsResponse`, `UpdateCartItemsVars`, `CartItemUpdate` live here and already expose `ErrorType`.
- `library/commerce/instacart/internal/gql/client.go`, the `*gql.Client.Mutation` method. Currently called inline from `add.go`.
- `library/commerce/instacart/internal/cli/add_history_test.go`, the established test pattern. Uses `newTestApp(t)` with a per-test SQLite store and no network.
- `library/commerce/instacart/internal/cli/history_hints_test.go`, pattern for asserting formatted-string hints.

### Institutional Learnings

- `docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md` already documents that Instacart's web API does not expose a clean order-history op and backfill must come from the DOM. Same theme applies here: Instacart's server-side shape is authoritative and we adapt to observed responses rather than enforce client-side invariants.

### External References

None gathered. The relevant surfaces are all in-repo and the error string is Instacart-specific.

## Key Technical Decisions

- Retry with the existing `ResolveProducts` result slice rather than re-querying. The autosuggest chain already returns up to five ranked items; re-querying would get a fresh session and reshuffle. Reusing the slice preserves relevance ordering.
- Retry ceiling of three total attempts (initial + two retries) when the original candidate set is large enough. Rationale: the first real miss is usually the top pick; a second or third candidate almost always wins. More than three attempts would pile latency without improving success rates and could look like hammering if Instacart tightens basket validation later. Configurable via a package constant, not a user flag.
- Only `notFoundBasketProduct` triggers retry in this change. Other `ErrorType` values surface unchanged. Rationale: scope discipline and a real observed symptom to validate against. Future error codes can be added to a small allowlist.
- Extract a small helper, `tryAddCandidates(app, retailer, cartID, candidates, qty)`, so the mutation loop is unit-testable via an injectable mutation invoker. The cobra `RunE` stays thin.
- Guidance on exhaustion names the exact next commands the user should run, including their own query string. Example copy: `all 3 candidates rejected as notFoundBasketProduct. Try: instacart search "<query>" --store <retailer> then instacart add --item-id <id> <retailer>. Or retry with --no-history.`
- JSON output adds `attempts: [{item_id, name, error_type}]` when retries happened. When a retry succeeds, `resolved_via` stays as its original value (`live` or `history`); an additional `retry_count` integer reports how many candidates were tried before success.
- Exit code stays 5 (`ExitConflict`) on exhaustion, preserving the current contract for agents grepping exit codes.
- Location-prefix filtering rejected. The observed cart mixes prefixes, so filtering would drop valid adds. Retry covers the failure mode at less cost.

## Open Questions

### Resolved During Planning

- Should retry re-run the full autosuggest chain? No. Reuse the already-ranked slice; fresh queries risk reshuffling and add latency.
- Should we widen the retryable error set now? No. Limit to the observed symptom; widen when we have a second confirmed symptom.
- Should the history-first resolver path also retry? Yes, but via a different mechanism. When the history hit fails with `notFoundBasketProduct`, fall back to a live search and then run the same retry loop on those candidates. Write-back still only fires on the candidate that actually succeeded.
- Is the retry ceiling a flag? No for now. A package constant keeps the surface tiny; promoting to a flag is a one-line change later.

### Deferred to Implementation

- Exact helper signature and return shape. The test scenarios below describe behavior; the implementer chooses the cleanest interface.
- Whether `resolveActiveCartID` needs to be called again on retry. Expected answer: no, the cart id is stable for the duration of the command. Verify against the types in `instacart/types.go` during implementation.
- Whether to log a one-line notice to stderr on retry when not in `--json` mode. Lean toward yes, but leave the wording to implementation.

## High-Level Technical Design

> This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.

Today's flow, one attempt:

```
ResolveProducts -> results[0] -> UpdateCartItemsMutation
                                   |
                                   ErrorType != "" -> return error
```

After change, up to three attempts:

```
ResolveProducts -> results[0..N]
        |
        v
  tryAddCandidates loops:
    attempt = UpdateCartItemsMutation(results[i])
    if attempt.ErrorType == "notFoundBasketProduct" and more candidates and under cap:
        record {item_id, name, error_type} and advance i
        continue
    if attempt.ErrorType != "":
        return error with attempts log
    return success with retry_count, picked candidate
```

History-first path rejoins the live loop on failure:

```
history hit -> single attempt
     |
     ErrorType == "notFoundBasketProduct" -> fall through to ResolveProducts
                                              then tryAddCandidates as above
```

## Implementation Units

- [ ] Unit 1: Extract mutation loop into a testable helper

Goal: Introduce `tryAddCandidates` (or equivalent) that encapsulates the mutation call and retry policy. Keep cobra `RunE` thin.

Requirements: R1, R2, R4.

Dependencies: None.

Files:
- Modify: `library/commerce/instacart/internal/cli/add.go`
- Test: `library/commerce/instacart/internal/cli/add_test.go` (new)

Approach:
- Move the existing `client.Mutation("UpdateCartItemsMutation", ...)` call plus response parsing into a helper that accepts the already-resolved candidate slice, quantity, cart id, and a mutation invoker function.
- Iterate candidates up to the retry ceiling. On `notFoundBasketProduct`, record the attempt and continue. On any other `ErrorType`, return immediately. On a clean response, return the winning candidate and the attempt log.
- The mutation invoker is a function value so the unit test can substitute a fake without touching `gql.Client`.
- Preserve the existing write-back call to `writeBackPurchasedItem` when the helper reports success, using the winning candidate rather than the original top pick.

Patterns to follow:
- `AppContext` / `newAppContext` plumbing already used throughout `internal/cli/*.go`.
- Test setup shape from `add_history_test.go` (`newTestApp(t)` with a tmpdir store).
- Error wrapping via `coded(ExitConflict, ...)` as already used at `add.go:177`.

Test scenarios:
- Happy path: first candidate succeeds; helper returns that candidate, `retry_count == 0`, and no attempt log entries.
- Retry-then-succeed: first candidate returns `notFoundBasketProduct`, second succeeds; helper returns the second candidate, `retry_count == 1`, and one entry in the attempt log with the first item id and error type.
- Retry-cap: three straight `notFoundBasketProduct` responses with three candidates; helper returns a conflict error, attempt log has three entries, no candidate reported as picked.
- Exhaust candidates below cap: two candidates supplied, both return `notFoundBasketProduct`; helper returns a conflict error after two attempts, does not fabricate a third.
- Non-retryable error: first candidate returns a different `ErrorType` (for example `"soldOut"`); helper returns immediately with a conflict error containing that error type, no further candidates attempted.
- Transport error: the mutation invoker returns a Go error; helper propagates it without consuming additional candidates.
- Empty candidates slice: helper returns a conflict error without calling the invoker.

Verification:
- `go test ./library/commerce/instacart/internal/cli/...` passes with the new tests.
- `go vet ./library/commerce/instacart/...` clean.
- Manual `instacart-pp-cli add <retailer> "<query>" --dry-run --json` unchanged (dry-run path short-circuits before the helper).

- [ ] Unit 2: Wire the helper into the live `add` command

Goal: Replace the inline mutation call in `newAddCmd` with `tryAddCandidates`, pass the full `ResolveProducts` result slice, and enrich JSON output with `retry_count` and `attempts`.

Requirements: R1, R2, R3.

Dependencies: Unit 1.

Files:
- Modify: `library/commerce/instacart/internal/cli/add.go`

Approach:
- After `ResolveProducts` returns, pass the full slice (up to the ceiling) to the helper rather than slicing to `results[0]`.
- When `itemID` was supplied directly, pass a single-element candidate slice and set retry ceiling to 1 so `--item-id` behavior is unchanged.
- For the history-first hit, attempt the single item once. On `notFoundBasketProduct`, fall through to `ResolveProducts` and the retry loop. Mark `resolved_via` as `"history->live"` when that fallback fires, so agents can tell the two paths apart.
- On helper success, extend the JSON envelope: add `retry_count` (int) and `attempts` (array of `{item_id, name, error_type}`, omitted or empty when no retries happened).
- On helper exhaustion, surface a human message that names the concrete next step: run `search`, pick an id, then `add --item-id`, or retry with `--no-history`. Include the query and retailer verbatim so a copy-paste works.
- Preserve `ExitConflict` exit code on exhaustion.

Patterns to follow:
- Existing JSON envelope shape at `add.go:189-204`.
- Error construction via `coded(...)`.
- Guidance-hint pattern from `historyIsEmpty` + `backfillHint` already in this file.

Test scenarios:
- Happy path, JSON mode, first candidate wins: envelope includes `added`, `cart_id`, `resolved_via: "live"`, `retry_count: 0`, no `attempts` key (or empty array, depending on chosen shape).
- Retry-then-succeed, JSON mode: envelope includes `retry_count: 1`, `attempts` has one entry with the first item id, and `added` names the second candidate.
- History fallback: seed a history hit whose mutation returns `notFoundBasketProduct`, then the live retry succeeds. `resolved_via == "history->live"`, write-back fires for the winning live candidate, attempt log includes the history item id.
- Exhaustion, text mode: stderr message contains the query, retailer, and both suggested commands; stdout is empty; exit code 5.
- Exhaustion, JSON mode: envelope is an error object with `error: "notFoundBasketProduct"`, `attempts` listing all tried candidates, `hint` field naming the next-step commands, exit code 5.
- `--item-id` direct path unchanged: single attempt, no retry, failure surfaces the underlying `ErrorType` unchanged aside from the new guidance hint.
- `--dry-run` path unchanged: no mutation fires, no retry envelope fields appear.

Verification:
- `go build ./...` succeeds.
- `instacart-pp-cli add costco "strawberries" --json` succeeds and the JSON envelope contains `retry_count` (0 when the first candidate wins, greater than 0 otherwise).
- Forcing a `notFoundBasketProduct` via a crafted item id yields the new guidance message with the user's actual query and retailer.

- [ ] Unit 3: Update SKILL.md and plugin mirror for the new JSON fields

Goal: Document `retry_count`, `attempts`, and the new exhaustion hint so downstream agents can rely on them.

Requirements: R2, R3.

Dependencies: Unit 2.

Files:
- Modify: `library/commerce/instacart/SKILL.md`
- Modify: `plugin/skills/pp-instacart/SKILL.md` (regenerated)
- Modify: `plugin/.claude-plugin/plugin.json` (patch version bump)

Approach:
- Under the `add` recipe section, add a short note: "Retries automatically up to 3 candidates on `notFoundBasketProduct`. JSON output includes `retry_count` and, when retries happened, an `attempts` array."
- Under exit codes, leave `5` unchanged but add a one-line note about the new guidance hint in stderr.
- Run the skill regenerator (see AGENTS.md: `go run ./tools/generate-skills/main.go`).
- Bump `plugin/.claude-plugin/plugin.json` patch version manually.

Patterns to follow:
- `AGENTS.md` section "Keeping plugin/skills in sync" is the authoritative procedure.

Test scenarios:
- Happy path: running the skill verifier (`.github/scripts/verify-skill/verify_skill.py`) against the instacart CLI passes.

Verification:
- `diff library/commerce/instacart/SKILL.md plugin/skills/pp-instacart/SKILL.md` shows no drift after regenerate.
- Plugin version in `plugin/.claude-plugin/plugin.json` incremented by one patch.

- [ ] Unit 4: Capture the failure mode in a solutions doc

Goal: Record the root-cause analysis so the next agent that hits this does not have to re-derive it.

Requirements: R1, R5.

Dependencies: None (can run in parallel with Unit 1).

Files:
- Create: `docs/solutions/best-practices/instacart-not-found-basket-product.md`

Approach:
- Short prose doc in the shape of the existing `instacart-orders-no-clean-graphql-op.md`. Cover: observed symptom, three root causes (autosuggest nondeterminism, shop / location drift, transient stock-out), why location-prefix filtering was rejected, why retry is the right fix, retry ceiling choice, and a link to this plan plus the PR.

Test scenarios: None (documentation).

Verification:
- File renders cleanly, linked from the PR description, referenced from the plan.

- [ ] Unit 5: Open the PR

Goal: Ship the change via a PR to `main` with a description that covers the bug, the fix, evidence, and release notes.

Requirements: R6.

Dependencies: Units 1, 2, 3, 4.

Files: None added; all prior units already staged.

Approach:
- Create a feature branch, push, open a PR with `gh pr create` using a HEREDOC body.
- PR body includes: short bug summary (observed command + error), root cause bullet list, what changed, before / after JSON output snippets, test coverage summary, link to this plan, link to the solutions doc.
- Follow commit-style convention from `AGENTS.md`: `fix(cli): ...` for code changes, `chore(plugin): regenerate pp-instacart skill + bump to X.Y.Z` for the plugin mirror bump, `docs(cli): ...` for the solutions doc.
- Never merge directly to `main`. PR only. Merge happens after human approval.

Test scenarios: None (ship step).

Verification:
- PR URL displayed.
- CI green on the branch: verify-skills, go build, go test.
- PR description contains a visible bug reproduction snippet and the new JSON envelope excerpt.

## System-Wide Impact

- Interaction graph: The change is contained to `internal/cli/add.go` and the new helper. The `gql` package, types package, store, and config are untouched.
- Error propagation: Exit code 5 preserved. New JSON error shape adds fields but keeps prior keys where they exist, so agents that already parsed the old envelope still work.
- State lifecycle risks: Write-back to `purchased_items` now fires for the candidate that actually succeeded, not necessarily the original top pick. This is a correctness improvement: today's behavior could write-back an item that the user bought this session even when the live resolver would have picked differently on retry. Worth calling out explicitly in the solutions doc.
- API surface parity: No other CLI in the library uses this retry pattern today. Do not generalize prematurely. If a sibling CLI develops a similar need, consider factoring later.
- Integration coverage: Unit tests mock the mutation invoker. A manual smoke test against a live Instacart cart is the only end-to-end verification pre-merge; record the result in the PR.
- Unchanged invariants: `--item-id` direct path still makes exactly one attempt and surfaces the underlying error type unchanged. `--dry-run` still short-circuits before any mutation. The three-call chain in `gql.ResolveProducts` is unchanged. Exit code contract is unchanged.

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Retry masks a deeper problem (for example the user's address or cart is wrong). | Surface `attempts` in JSON so agents can inspect. Keep the retry ceiling low (3). Exhaustion message names the next diagnostic steps. |
| Adding retry changes perceived latency for successful adds that retried. | Three attempts at ~200-400ms each is acceptable for an interactive cart operation. Add a single-line stderr notice so users in text mode know a retry happened. |
| History-fallback to live resolver could surprise users who set `--no-history` expectations. | `--no-history` path never hits the history resolver in the first place, so no behavior change there. For default path, `resolved_via: "history->live"` makes the fallback visible. |
| New `attempts` key in JSON output could break strict schema consumers. | The field is additive and omitted (or empty) when `retry_count == 0`, preserving prior shape for the common case. Document in SKILL.md before release. |
| Skill regeneration drift between `library/` and `plugin/skills/`. | `AGENTS.md` procedure explicitly covered in Unit 3. Verifier runs in CI. |
| Instacart retires or renames `notFoundBasketProduct`. | Scoped allowlist. When the error string changes, add the new name. Not a blocker; graceful no-retry fallback for unknown error types. |

## Documentation / Operational Notes

- SKILL.md (library + plugin mirror) updated in Unit 3.
- Solutions doc added in Unit 4.
- PR description carries a before / after JSON snippet and a short rationale paragraph usable as release notes.
- No env var changes. No migration. No feature flag.
- No monitoring surface. This CLI does not emit telemetry today.

## Sources & References

- Live tracing of the failure today against cart id `757109404` at Costco (location `1576`, with a pre-existing cross-location item at `18148`).
- `library/commerce/instacart/internal/cli/add.go` (current mutation call at line 177).
- `library/commerce/instacart/internal/gql/search.go` (`ResolveProducts`, `ensureInventoryToken`).
- `library/commerce/instacart/internal/cli/add_history_test.go` (test pattern to mirror).
- `docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md` (prior institutional note in the same domain).
- `AGENTS.md` sections "Keeping plugin/skills in sync" and "Commit style".
