# Bookclicker CLI — Absorb Manifest

## Ecosystem scan result: EMPTY

| Source | Query | Result |
|---|---|---|
| GitHub repos | `bookclicker` | 0 |
| GitHub code | `bookclicker.com/api` | 0 |
| npm | `bookclicker` | 0 |
| Web (MCP servers, Claude skills, CLIs, scripts) | multiple | 0 |

There is no third-party tool for Bookclicker. **Layer 1 has no competitor to absorb.** The incumbent is the Bookclicker web UI itself, so absorbed rows below are drawn from the UI's own capability surface (FAQ-documented workflows + verified endpoints). Nearly all differentiation comes from Layer 2.

## Absorbed (match or beat the web UI)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Browse marketplace newsletters | Launch Center list browser | `bookclicker-pp-cli lists search` | Offline FTS over ~4,000 lists; UI paginates 25 at a time across 161 pages |
| 2 | Filter lists by genre | Launch Center genre dropdown | `(behavior in bookclicker-pp-cli lists search) --genre` | Composable with price, reach, and weekday filters in one query |
| 3 | Filter by promo type | Calendar promo selector | `(behavior in bookclicker-pp-cli lists search) --inv-type solo\|feature\|mention` | Server + local filtering; UI requires opening each calendar |
| 4 | See list reach and engagement | List card | `(behavior in bookclicker-pp-cli lists search) --min-members/--min-open-rate` | Numeric thresholds; UI shows values but cannot filter on them |
| 5 | See promo pricing | List card | `(behavior in bookclicker-pp-cli lists search) --max-price/--swap-only` | Cross-list price comparison in one table |
| 6 | View a single list | List detail | `bookclicker-pp-cli lists get` | `--json` + `--select`, agent-consumable |
| 7 | List campaign history | (found in JS bundle) | `bookclicker-pp-cli lists campaigns` | Not surfaced anywhere in the UI navigation |
| 8 | View my own lists | My Lists page | `bookclicker-pp-cli my-lists` | Includes pricing/swap-only config in one view |
| 9 | Calendar availability for a list | Booking calendar | `bookclicker-pp-cli calendar availability` | N months in one call, machine-readable |
| 10 | Per-date inventory | Calendar day cell | `bookclicker-pp-cli inventory get` | Direct date query, no click path |
| 11 | Set per-date inventory | Calendar day editor | `bookclicker-pp-cli inventory set` | Scriptable; `--dry-run` |
| 12 | List pen names | Pen Names page | `bookclicker-pp-cli pen-names list` | Includes requests + groups |
| 13 | Create/update/delete pen name | Pen Names editor | `(generated endpoint) pen_names create/update/delete` | Typed flags, `--dry-run` |
| 14 | Buyer-eligible pen names | Launch booking modal | `bookclicker-pp-cli pen-names for-buyer` | — |
| 15 | List my books | Pen Name book list | `bookclicker-pp-cli books list` | FTS by title |
| 16 | Get/create/update/delete book | Book editor | `(generated endpoint) my_books get/create/update/delete` | Typed flags, `--dry-run` |
| 17 | Accept a reservation | Offer inbox | `bookclicker-pp-cli reservations accept` | `--reply-message`, batchable |
| 18 | Decline a reservation | Offer inbox | `bookclicker-pp-cli reservations decline` | Same |
| 19 | Cancel reservation (buyer/seller) | Reservation row | `bookclicker-pp-cli reservations cancel --side` | Single command for both sides |
| 20 | Refund request / issue refund | Reservation row | `bookclicker-pp-cli reservations refund` | `--dry-run` mandatory before send |
| 21 | Bulk cancel | Cancel-all button | `bookclicker-pp-cli reservations cancel-all --side` | Guarded by `--yes` |
| 22 | Request confirmation | Reservation row | `bookclicker-pp-cli reservations request-confirmation` | — |
| 23 | Dismiss reservation notice | Feed item | `(generated endpoint) reservations dismiss` | — |
| 24 | Confirm-promo options | Confirm Promos panel | `bookclicker-pp-cli promos options` | — |
| 25 | Confirm a sent promo | Confirm Promos panel | `bookclicker-pp-cli promos confirm` | The recurring chore, scriptable |
| 26 | Start a conversation | Message composer | `bookclicker-pp-cli conversations start` | — |
| 27 | Integration health | Integrations page | `bookclicker-pp-cli integrations list` | **Redacts `api_key.key` unconditionally** |
| 28 | Account snapshot | Dashboard | `bookclicker-pp-cli account` | One call returns user+lists+books+pen names |
| 29 | External reservations | Off-platform tracker | `(generated endpoint) external_reservations create/update` | — |

## Transcendence (only possible with a local mirror)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Cross-list launch planner | `plan` | hand-code | Requires all ~4,000 lists + their weekday inventory patterns joined locally and ranked. The UI opens one calendar at a time across 161 pages. | Use this to fill a launch window. Do NOT use it to inspect a single list; use 'lists get'. |
| 2 | Offline marketplace search | `search` | hand-code | FTS5 over the synced corpus; the API offers no full-text search across all pages | none |
| 3 | Reach-per-dollar ranking | `(behavior in plan) --rank value` | hand-code | Requires `active_member_count × open_rate ÷ price` computed across every list at once | none |
| 4 | Swap reciprocity ledger | `swap-balance` | hand-code | Requires reservation history joined by counterparty across both directions. Nothing in the UI shows who owes whom. | Use this to find partners who take swaps without reciprocating. |
| 5 | Partner performance / ROI | `partner-roi` | hand-code | Requires historical reservations joined to list metrics at time of booking | Use this to decide who to rebook. Not a live marketplace query. |
| 6 | Confirmations due queue | `confirm-due` | hand-code | Requires local reservation state to know what awaits confirmation, in one list rather than a home-page widget | none |
| 7 | Launch coverage health | `launch health` | hand-code | Requires joining a book's launch window against booked reservations to expose uncovered dates | none |
| 8 | List quality drift | `drift` | hand-code | Requires two or more syncs; open/click rate change over time exists nowhere in the product | Use this to catch decaying lists before rebooking. |
| 9 | Remaining promo capacity | `capacity` | hand-code | Encodes Solo=1, Feature=1, Mention=9 caps per newsletter per date against booked inventory | none |
| 10 | Stale outbound offers | `stale` | hand-code | Requires local pending-offer history with age; the UI shows status but not age | none |

**Scoring:** all ten score ≥5/10 on user value. #1, #4, and #8 are the strongest — each answers a question the product structurally cannot.

## Shipping-scope summary

- **Absorbed rows:** 29 (24 clean command paths, 5 generated-endpoint rows)
- **Transcendence rows:** 10, all `hand-code`
- **Stubs:** none proposed
- **Excluded deliberately:** payment-method mutation (`/api/payment_infos/*`) and `user/destroy_member`. Read-only payment listing is included; card mutation and account deletion are out of scope for an agent-driven CLI.

## Hard constraints carried into generation

1. `api_key.key` from `/api/integrations/{id}` must be redacted in every output path and never logged.
2. Auth is cookie + CSRF, not bearer. The stored session cookie is a credential: never written to a manuscript, HAR, or README.
3. Mutating commands must support `--dry-run`; `cancel-all` and `refund` additionally require `--yes`.
