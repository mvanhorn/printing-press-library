# amazon-pp-cli Shipcheck

Manual shipcheck — `printing-press shipcheck` was not run because the generator was not used (no spec source; hand-built per the instacart-pattern decision).

## Build gate
- `go build ./...` — PASS
- `go vet ./...` — PASS (no findings)
- `gofmt -l .` — PASS (no diffs after `-w`)
- `go test ./...` — PASS, 19 tests across `store`, `auth`, `amazon`, `history`

## Behavioral dogfood

Test harness: isolated `AMAZON_PP_CLI_HOME=/tmp/amazon-pp-dogfood-QpEc61`, two profiles created from scratch (`personal`, `work`), JSONL fixtures loaded into each profile's per-profile DB.

| Scenario | Expected | Got | Verdict |
|---|---|---|---|
| `amazon-pp-cli --version` | print v0.1.0 | "amazon-pp-cli version 0.1.0" | PASS |
| `amazon-pp-cli --help` | tree of commands | every command listed | PASS |
| `profiles list` (empty) | "no profiles configured" hint | exactly that | PASS |
| `profiles add personal --label "Personal account"` | profile written, becomes active | yes, persisted | PASS |
| `profiles add work --label "Work account"` | second profile, doesn't change active | yes | PASS |
| `profiles list` | table with `*` next to active | renders correctly | PASS |
| `profiles list --json` | structured payload | valid JSON | PASS |
| `--profile personal history import <jsonl>` | imports orders + items | "imported 3 orders, 6 items (0 skipped)" | PASS |
| `--profile personal history stats` | counts + last-purchased | orders=3, items=4, last=2026-04-28 | PASS |
| `history list` | most-purchased rollup, COUNT/LAST/ASIN/TITLE | renders sorted by purchase_count DESC | PASS |
| `history list --json` | structured list | valid JSON | PASS |
| `history search 'charmin'` | one match | B07AAA0001 | PASS |
| `history search 'bath tissue'` | no match (no "bath" or "tissue" token in titles) | "no matches" | PASS (honest — FTS doesn't pretend) |
| `history search 'tide pods'` | one match | B07CCC0003 | PASS |
| `add --dry-run 'charmin'` | preview without writing to cart | "would add B07AAA0001 (Charmin Ultra Strong...) — purchase_count=2, last=2026-04-02" | PASS |
| `add --dry-run --json 'charmin'` | structured preview | full JSON with asin, title, purchase_count, last_purchased_at, dry_run=true, added=false | PASS |
| `add 'totally-new-thing' --dry-run` | exit 4 with helpful refusal | exit=4, "no match in history (this CLI is repurchase-only; run `history search` to see what's loaded)" | PASS — safety rail enforced |
| `reorder-last --dry-run` | enumerate last order's items | "would re-add 2 items from order 112-0000003-0000003 (placed 2026-04-28)" + 2 lines | PASS |
| `reorder-last --dry-run --json` | structured plan | full payload, items[], added_count=0 | PASS |
| `checkout` (no --yes) | exit 10 with "requires --yes" | exit=10, exact message | PASS — confirmation gate enforced |
| `checkout --dry-run` | walk flow without session | "would GET checkout page and POST place-order" (does NOT require session in dry-run) | PASS |
| `checkout --yes --dry-run --json` | structured dry-run | JSON with `confirmed:false, status_note:"dry-run..."` | PASS |
| `--profile work history import <work jsonl>` | imports into work DB | "imported 2 orders, 2 items" | PASS |
| `--profile work history list` | shows ONLY work items | Logitech mouse + Anker hub, no personal items | PASS — cross-profile isolation verified |
| `--profile personal history list` | still shows ONLY personal items | Charmin/Tide/Batteries/Stickies, no work items | PASS |
| `auth paste --header "session-id=...; at-main=...; ubid-main=..."` | persist 3 cookies | "saved 3 cookies to .../cookies.json" | PASS |
| `auth status` | report loaded + has_marker | loaded=true, has_marker=true, count=3 | PASS |
| `doctor --skip-live` | offline diagnostic | cookies_loaded=true, history_orders=3, no live ping | PASS |
| `auth logout` | delete cookies.json | "removed ..." | PASS |
| `auth status` (after logout) | report cleared | loaded=false, count=0 | PASS |

## Things not tested in this dogfood

- Live amazon.com reachability (`doctor` without `--skip-live`) — requires real cookies. Will be exercised in Phase E.
- Real `add` / `cart show` / `checkout` against amazon.com — same.
- kooky `auth login` flow — depends on the user's Chrome state and Keychain perms.

## Verdict

`ship`. The CLI compiles, every unit test passes, every offline dogfood scenario passes including the two safety-critical gates (repurchase-only refusal, checkout confirmation gate) and the cross-profile isolation invariant. The live-fire smoke test (Phase E) requires the user's real session cookies.

Scorecard estimate: I did not run `printing-press scorecard` because that tool walks a generated CLI's spec-derived surfaces (`agent-context`, `tools-manifest.json`, etc.) that this hand-built CLI does not emit. Subjective score would be ~70/100 — high on safety, behavioral correctness, and agent-friendliness; low on the generator-shape conformance dimensions the scorecard expects (no MCP, no `agent-context` command, no spec).
