# Acceptance Report: splitwise (reprint v4.19.0)
Level: Full Dogfood — READ-ONLY scope (user constraint)
Gate: PASS  (14/14)

## Live read-only matrix (real account)
- sync --full (read-only GET endpoints): 54 friends, 35 groups, 100 expenses, 100 notifications, 7 categories, current-user. get-currencies 153 returned / 0 stored (no extractable ID — code-keyed; live unaffected). Expense store capped ~100/page (account has 638; raise --max-pages for full history).
- balances --json: real net position (net + by-currency object). PASS
- debts --aged --json: 9 relationships, 2 with populated age (7 null — oldest-expense lookup misses beyond the 100-expense page; honest null, same as prior print). PASS
- spend --group-by category --json: 12 category buckets. PASS
- ledger <group-id> --json: {expenses, group_id, group_name, running_balances} — running balances present. PASS
- recurring --json (NEW): scanned 85 expenses, surfaced 5 recurring groups. PASS
- search "the"/"a"/"dinner": 20/7/15 FTS hits over synced expenses. PASS

## Write-path safety (no real mutations)
- settle-up <group-id> (no --record): computed 5-transfer plan, printed "plan only — re-run with --record". No POST. PASS
- split <group-id> --amount 30 (no --record, NEW): equal mode, 13 shares summing EXACTLY to 30.00, preview only. No POST. PASS
- split <group-id> --amount 30 --record under PRINTING_PRESS_VERIFY=1 (NEW): "would create expense (verify mode)" — write gate fires, no POST. PASS

## Silent-empty regression check
NONE. Every store-reading novel command returns real data; the dual-form listResourceRows correctly reads get-friends/get-groups/get-expenses keys under the v4.19.0 store.

## Fixes applied: 0 (all green on first live read pass)
## Printing Press issues for retro: 2
- get-currencies (code-keyed, no numeric id) drops all 153 rows on sync (all_items_failed_id_extraction) — the #2327 class, still unfixed.
- v4.19.0 novel-stub emission (newNovel*Cmd files) collides with hand-authored impls on reprint.
