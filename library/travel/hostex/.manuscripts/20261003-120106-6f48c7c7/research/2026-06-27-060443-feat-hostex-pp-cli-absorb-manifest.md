# Hostex CLI — Absorb Manifest

## Landscape
No third-party Hostex CLI, SDK wrapper, MCP server, or Claude plugin exists in the wild (web search + public printing-press registry: zero hits). The only first-party automated surfaces are the **REST API (86 ops)** and the **official Hostex MCP server (72 tools)** at `https://hostex.io/mcp`. The CLI absorbs the full REST surface as typed commands and auto-mirrors its Cobra tree to MCP (matching/exceeding the official MCP), then transcends with local-SQLite cross-entity commands no single API call can answer.

## Absorbed (match or beat everything that exists)
See `absorbed-table.md` — all 86 REST operations across 16 tags, each generated as a typed command with `--json/--select/--dry-run/--csv`, offline-friendly where syncable, and error_code-aware typed exit codes. Highlights by tag: Reservations (18), Property (13), Listing Calendar (10), Finance/Incomes & Expenses (8), Task (8), Knowledge Base (5), Messages (4+offers/preapproval), Reviews, Automation, Availability, Channels, Reservation Tags, Calendar Share Links, Webhooks, OAuth.

## Transcendence (only possible with our approach)
All rows are hand-code local-SQLite joins over the synced mirror (data-source `local`/`auto`), read-only, agent-native output. Source: novel-features subagent (customer model -> candidates -> adversarial cut; 7 survivors of 14 candidates, all >= 6/10).

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Operations gap finder | `ops-gaps --within 7d` | 8/10 | hand-code | Joins reservations × tasks × check-in-details to surface occupied/imminent stays missing a clean or check-in info; no endpoint returns this. | none |
| 2 | Inbox SLA triage | `inbox-sla --breach 6h` | 8/10 | hand-code | Derives per-thread age from latest-message ts + unread_count and ranks SLA breaches; query-conversations has no age clock. | Use this for unanswered-guest triage ranked by clock. Do NOT use it to read a thread's messages; use the generated `messages get-conversation-details`. |
| 3 | Cross-channel price parity | `price-parity --property <id> --days 30` | 7/10 | hand-code | Diffs per-channel listing price + min-stay for the same property-date locally; no single call diffs channels. | Use for cross-channel price/min-stay drift. Do NOT use it for availability/inventory mismatch; use `oversell-watch`. |
| 4 | Oversell / double-sell watch | `oversell-watch --days 30` | 7/10 | hand-code | Joins property master availability against per-channel listing inventory to flag double-sell risk. | Use for availability/inventory mismatch (double-sell risk). Do NOT use it for price/min-stay drift; use `price-parity`. |
| 5 | Revenue rollup | `revenue-rollup --by property --month 2026-06` | 7/10 | hand-code | Nets income−expense by property/month from the transactions ledger, resolving cached item/method dictionaries. | none |
| 6 | Stay brief dossier | `stay-brief <stay_code>` | 7/10 | hand-code | Fan-joins reservation + guest + thread state + tasks + transactions + review on stay_code into one agent-shaped object. | Use for a single stay's full picture. Do NOT use it to scan many stays for problems; use `ops-gaps`. |
| 7 | Automation preview | `automation-preview --day tomorrow` | 6/10 | hand-code | Joins pending automation plans to each thread's reply-state so queued bot actions on human-handled threads are flagged. | Use to vet queued automated actions before they fire. Do NOT use it to find slow human threads; use `inbox-sla`. |

## Killed candidates (audit trail)
channel-health (thin wrapper over query-channel-accounts), occupancy-report (lower cadence than revenue-rollup), checkin-today (subsumed by ops-gaps/stay-brief), review-gaps (query-reviews already filters replied/rating; folded into automation-preview), guest-history (occasional, not weekly), kb-coverage (speculative; real value needs LLM), rate-budget (needs runtime counters the mirror doesn't model).
