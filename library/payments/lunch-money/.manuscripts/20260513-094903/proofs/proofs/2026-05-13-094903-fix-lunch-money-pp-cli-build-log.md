# Lunch Money CLI Build Log

## What was built

### Phase 2 (generator)
- 59 spec-derived commands across 16 resource tags (transactions, categories, tags, manual-accounts, plaid-accounts, recurring-items, budgets, balance-history, crypto, cryptocurrencies, summary, me)
- Framework commands: agent-context, analytics, api, auth (set-token/status/logout/setup), completion, doctor, export, feedback, help, import, profile, search, sync, tail, version, which, workflow
- Local SQLite store with FTS5, sync, rate-limit-aware client
- MCP server binary (lunch-money-pp-mcp) with stdio+http transports + code orchestration (per x-mcp Cloudflare-pattern enrichment)
- Auth env-var aliases all 5 ecosystem names: `LUNCHMONEY_ACCESS_TOKEN` (canonical), `LUNCHMONEY_API_KEY`, `LUNCHMONEY_TOKEN`, `LUNCHMONEY_API_TOKEN`, `LUNCH_MONEY_API_KEY`
- `recurring` alias for `recurring-items`

### Phase 3 (Codex-delegated novel features — 2 batches)

**Batch 1 (7 read-only local-store commands):**
1. `triage` — unreviewed transactions with suggested category from per-payee history
2. `transactions subscriptions` — cadence detection (weekly/monthly/yearly) over local txns
3. `transactions duplicates` — near-dup self-join with union-find clustering
4. `accounts stale` — UNION across manual_accounts + crypto_manual + balance_history
5. `net-worth on <date>` — point-in-time across balance_history with carry-forward
6. `recurring missing --month YYYY-MM` — recurring_items left-join transactions
7. `budgets burn --period YYYY-MM` — per-category linear-rate projection

Plus shared `internal/cli/transcendence_helpers.go` (+ tests) for common store query/aggregation helpers. Codex used 240,710 tokens.

**Batch 2 (1 mutating bulk command):**
8. `transactions retag --match REGEX --add-tag X --remove-tag Y --category-id N --dry-run` — local FTS5 regex match → bulk PUT /transactions. Local tag-name → tag-id resolution; auto-create tag via POST /tags when used in --add-tag with unknown name. Codex used 158,928 tokens.

### Phase 3 narrative fixes (Claude)
- `auth set-token YOUR_TOKEN_HERE` (added required positional in narrative; the command itself was correct)
- `summary --include-occurrences` (was `--with-occurrences`, doesn't exist)
- `plaid-accounts trigger-fetch` for Plaid resync (was `accounts plaid sync`, doesn't exist)

## What was intentionally deferred / not built

- **No write endpoint for /recurring_items** in v2 spec — Subscription Detective's "create recurring item" suggestion is informational, surfaced as recommended UI follow-up rather than a CLI mutation.
- **No FX rate normalization** in net-worth / crypto holdings — base currency from account JSON used as-is.
- **No alias top-level `accounts manual` / `accounts plaid`** — kept generator defaults `manual-accounts` and `plaid-accounts`. The new top-level `accounts` parent has only the novel `stale` subcommand.
- **No DeepWiki query** during Phase 1.5a.6 — skipped within the 2-min budget; the OpenAPI spec is exhaustive and competitor analysis filled the gap.

## Skipped body fields / generator-limitation residue

- `transactions update` (bulk PUT) command's body shape: the spec uses `{transactions: [...]}` array under `transactions` key — Codex hand-wrote the retag command using this same shape, verified PASS.
- Some spec-derived commands ship with operation-id-derived names (`get-all`, `delete-by-id`, `update-id`) that are not the friendliest English. Some have `get-all` → `list` alias auto-added by generator. Polishing the rest is a Phase 5.5 / future-polish task.

## Generator limitations observed

- The generator emitted `recurring-items` as the parent slug from the spec tag. Codex added `recurring` as a Cobra alias so `recurring missing` works.
- The generator did NOT emit a top-level `accounts` parent; Codex created it specifically to host `accounts stale`. The existing `manual-accounts` / `plaid-accounts` remain authoritative for spec endpoints.

## Verification (Phase 3 exit)

- `go build ./...` PASS
- `go vet ./...` PASS
- `go test ./...` PASS
- All 8 novel-feature commands respond to `--help`
- `validate-narrative --strict --full-examples`: 11/11 narrative commands resolved + dry-run PASS
- `public-param-audit --strict`: 0 pending findings
- Spec-derived auth wiring inspected in `internal/config/config.go`: 5 env-var aliases present, canonical first
- Live API reachable (HTTP 200 on /me); mock server reachable (HTTP 200 with placeholder bearer)
