# Dub CLI — Absorb Manifest

## Ecosystem audit (refreshed for 2026-05)

| Tool | Type | Lang | Surface | Status |
|------|------|------|---------|--------|
| dubinc/dub-ts | Official SDK | TS | All 53 ops (Speakeasy-generated) | Active, source of truth |
| dubinc/dub-go | Official SDK | Go | All 53 ops | Active |
| dubinc/dub-python | Official SDK | Python | All 53 ops | Active |
| dubinc/dub-php | Official SDK | PHP | All 53 ops | Active |
| dubinc/dub-ruby | Official SDK | Ruby | All 53 ops | Active |
| **dub-cli** (npm, official) | **Official CLI** — NEW | TS | **6 commands** (`login`, `config`, `domains`, `shorten`, `links`, `help`) — covers ~3-4 of 53 operations | Active (v0.0.12, MVP) |
| gitmaxd/dubco-mcp-server-npm | MCP server | JS | 4 ops (`POST /links`, `PATCH /links/{id}`, `DELETE /links/{id}`, `GET /domains`) | Active |
| sujjeee/dubco | Community CLI | TS | Link shortening only (~3 ops) | **Stale ~24 mo** |
| Bitly CLI / Short.io / Rebrandly | Competitors | various | Link CRUD only | Active |

**Key insight:** Official `dub-cli` shipped after the prior print closed issue #506 — but it is an MVP covering **3-4 of 53** operations. No analytics, no bulk, no partner program, no bounties, no conversion tracking, no commissions, no payouts, no customers, no QR. The single MCP wraps **4 of 53**. Our coverage is ~10× the official CLI's; transcendence is uncontested.

---

## Absorbed (match the spec exactly — generator-emitted)

The generator absorbs all 53 spec operations as typed commands. This manifest groups them by resource for review; the generator enumerates the spec faithfully.

| # | Resource | Operation | Method+Path | Command | Notes |
|---|----------|-----------|-------------|---------|-------|
| 1 | links | list | GET /links | `dub-pp-cli links list` | --json, --select, --csv, FTS via local store |
| 2 | links | create | POST /links | `dub-pp-cli links create` | --dry-run, --tag, --folder |
| 3 | links | retrieve | GET /links/info | `dub-pp-cli links get` | local-store fast path |
| 4 | links | update | PATCH /links/{linkId} | `dub-pp-cli links update` | --dry-run, diff preview |
| 5 | links | delete | DELETE /links/{linkId} | `dub-pp-cli links delete` | --yes |
| 6 | links | upsert | PUT /links/upsert | `dub-pp-cli links upsert` | idempotent CI flow |
| 7 | links | count | GET /links/count | `dub-pp-cli links count` | local fast path |
| 8 | links | bulk-create | POST /links/bulk | `dub-pp-cli links bulk create` | --stdin-jsonl, partial-success report |
| 9 | links | bulk-update | PATCH /links/bulk | `dub-pp-cli links bulk update` | diff preview, --yes >25 |
| 10 | links | bulk-delete | DELETE /links/bulk | `dub-pp-cli links bulk delete` | --yes, audit log |
| 11 | analytics | retrieve | GET /analytics | `dub-pp-cli analytics get` | per-second rate-limit aware |
| 12 | events | list | GET /events | `dub-pp-cli events list` | `--follow` polling absorbs prior `tail` |
| 13 | tags | list | GET /tags | `dub-pp-cli tags list` | local store |
| 14 | tags | create | POST /tags | `dub-pp-cli tags create` | idempotent on name |
| 15 | tags | update | PATCH /tags/{id} | `dub-pp-cli tags update` | (NEW vs prior) |
| 16 | tags | delete | DELETE /tags/{id} | `dub-pp-cli tags delete` | --yes |
| 17 | folders | list | GET /folders | `dub-pp-cli folders list` | |
| 18 | folders | create | POST /folders | `dub-pp-cli folders create` | |
| 19 | folders | update | PATCH /folders/{id} | `dub-pp-cli folders update` | (NEW vs prior) |
| 20 | folders | delete | DELETE /folders/{id} | `dub-pp-cli folders delete` | --yes |
| 21 | domains | list | GET /domains | `dub-pp-cli domains list` | |
| 22 | domains | create | POST /domains | `dub-pp-cli domains create` | |
| 23 | domains | update | PATCH /domains/{slug} | `dub-pp-cli domains update` | |
| 24 | domains | delete | DELETE /domains/{slug} | `dub-pp-cli domains delete` | --yes |
| 25 | domains | register | POST /domains/register | `dub-pp-cli domains register` | status-poll wrapper |
| 26 | domains | status | GET /domains/status | `dub-pp-cli domains status` | pretty verification state |
| 27 | qr | get | GET /qr | `dub-pp-cli qr get` | binary response — `--out file.png` or `--base64` |
| 28 | track | lead | POST /track/lead | `dub-pp-cli track lead` | --dry-run |
| 29 | track | sale | POST /track/sale | `dub-pp-cli track sale` | --dry-run, idempotency key |
| 30 | track | open | POST /track/open | `dub-pp-cli track open` | |
| 31 | customers | list | GET /customers | `dub-pp-cli customers list` | |
| 32 | customers | get | GET /customers/{id} | `dub-pp-cli customers get` | |
| 33 | customers | update | PATCH /customers/{id} | `dub-pp-cli customers update` | (NEW vs prior — was read-only) |
| 34 | customers | delete | DELETE /customers/{id} | `dub-pp-cli customers delete` | --yes (NEW vs prior) |
| 35 | partners | list | GET /partners | `dub-pp-cli partners list` | |
| 36 | partners | create | POST /partners | `dub-pp-cli partners create` | |
| 37 | partners | analytics | GET /partners/analytics | `dub-pp-cli partners analytics` | (NEW vs prior) |
| 38 | partners | applications | GET /partners/applications | `dub-pp-cli partners applications list` | |
| 39 | partners | approve-app | POST /partners/applications/approve | `dub-pp-cli partners applications approve` | --yes (path simplified) |
| 40 | partners | reject-app | POST /partners/applications/reject | `dub-pp-cli partners applications reject` | --yes (path simplified) |
| 41 | partners | ban | POST /partners/ban | `dub-pp-cli partners ban` | --yes |
| 42 | partners | deactivate | POST /partners/deactivate | `dub-pp-cli partners deactivate` | --yes (NEW vs prior) |
| 43 | partners | links list | GET /partners/links | `dub-pp-cli partners links list` | |
| 44 | partners | links create | POST /partners/links | `dub-pp-cli partners links create` | |
| 45 | partners | links upsert | PUT /partners/links/upsert | `dub-pp-cli partners links upsert` | |
| 46 | commissions | list | GET /commissions | `dub-pp-cli commissions list` | local-store join |
| 47 | commissions | update | PATCH /commissions/{id} | `dub-pp-cli commissions update` | |
| 48 | commissions | bulk update | PATCH /commissions/bulk | `dub-pp-cli commissions bulk update` | diff preview |
| 49 | bounties | submissions list | GET /bounties/{bountyId}/submissions | `dub-pp-cli bounties submissions list` | (NEW vs prior) |
| 50 | bounties | approve sub | POST /bounties/{bountyId}/submissions/{id}/approve | `dub-pp-cli bounties submissions approve` | --yes (NEW vs prior) |
| 51 | bounties | reject sub | POST /bounties/{bountyId}/submissions/{id}/reject | `dub-pp-cli bounties submissions reject` | --yes (NEW vs prior) |
| 52 | payouts | list | GET /payouts | `dub-pp-cli payouts list` | |
| 53 | tokens | embed referrals | POST /tokens/embed/referrals | `dub-pp-cli tokens embed-referrals` | (path simplified vs prior) |

Every absorbed command supports: `--json`, `--select <field,...>`, typed exit codes (0/2/3/4/5/7/10), `--dry-run` for mutations, agent-native examples in `--help`.

---

## Reprint Reconciliation — prior 14 novel features → 12 in this print

Prior CLI (v2.3.9) shipped 14 transcendence features. Re-scoring each against current personas (marketer / partner-program operator / growth engineer) and the refreshed spec:

| # | Prior feature | Verdict | Justification |
|---|---------------|---------|---------------|
| 1 | `links stale` | **KEEP** | Still requires local store join. Highest user-impact in the prior print. |
| 2 | `links drift` | **KEEP** | Sequential snapshots are still local-only. Kills "campaign died and nobody noticed" failure mode. |
| 3 | `links duplicates` | **KEEP** | Pure SQL self-join, cheap to maintain, surfaces real consolidation candidates. |
| 4 | `links lint` | **KEEP** | Pure-data audit, no API call. Catches brand-conflict slugs before they ship. |
| 5 | `links rewrite` | **KEEP** | Highest-leverage feature for power users. Diff-with-dry-run protects mass mistakes. |
| 6 | `campaigns` (tag dashboard) | **REFRAME → `links rollup`** | Tag-grouped dashboard plus folder-grouped. Joins analytics × tags × folders × links locally. Renamed because "campaigns" overloaded with `track` events terminology. |
| 7 | `funnel` (click→lead→sale) | **REFRAME → keep** | Tied more concretely to `/track/lead` + `/track/sale` event flow. Still local-store only — the API can't compute conversion ratios. |
| 8 | `customers journey` | **KEEP** | With customer mutations now exposed, the timeline is more useful. |
| 9 | `partners leaderboard` | **REFRAME** | Now reads from `/partners/analytics` (NEW endpoint) joined with local commissions × payouts for accurate per-partner ROI. |
| 10 | `partners audit-commissions` | **KEEP** | Cross-resource reconciliation strengthened by `/partners/analytics`. |
| 11 | `domains report` | **DROP** | Lower-impact than the rest; consolidate the useful parts (unverified-domain check) into `health`. Too narrow to stand alone. |
| 12 | `health` | **KEEP** | Cross-resource report. Now also surfaces bounty submissions awaiting review and `/partners/analytics` headroom. |
| 13 | `since` | **KEEP** | Universal change feed. Powers agent "what's new" flows. |
| 14 | `tail` | **DROP** | Polling loop is just an ergonomic on `events list`. Consolidate to `events list --follow`. Removes a redundant top-level command. |

**Add since prior print (driven by new spec endpoints):**
- **`bounties triage`** — Group bounty submissions by status × age × bounty type. Surfaces what's awaiting review and what's been pending too long. Direct value from `/bounties/{bountyId}/submissions` endpoints (NEW). Persona: partner-program operator.
- **`bounties payout-projection`** — Project upcoming payouts from approved-but-unpaid submissions × current commission rates. Marketing/finance ops question that takes 4 dashboard tabs today. Persona: partner-program operator + finance.

**Net:** keep 8, reframe 3, drop 2, add 2 → **12 transcendence features**.

---

## Transcendence (only possible with local store + cross-API joins)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Dead-link detection | `links stale --days 90` | 9/10 | Local store of links + analytics joined by `link_id`. Zero clicks in N days, archived links with traffic, expired links with active referrers. No API endpoint shaped this way. |
| 2 | Performance drift detection | `links drift --window 7d --threshold 30` | 8/10 | Sequential analytics snapshots in SQLite enable week-over-week deltas. `/analytics` returns point-in-time only. |
| 3 | Duplicate destination finder | `links duplicates` | 6/10 | Pure SQL self-join over local link store. Surfaces consolidation candidates and accidental dupes from bulk-create overruns. |
| 4 | Slug-collision lint | `links lint` | 6/10 | Pure-data audit: lookalikes (`/launch` vs `/launches`), reserved-word violations, brand-conflict hazards. No API to ask. |
| 5 | Bulk URL/UTM rewrite with diff | `links rewrite --match 'utm_source=old' --replace 'utm_source=new'` | 9/10 | Show every link that would change and the exact patch BEFORE sending. Saves campaign-wide blast radius mistakes. |
| 6 | Tag/folder-grouped rollup | `links rollup --by clicks --groupBy tag --interval 30d` | 7/10 | Joins `analytics_buckets` × `links` × `tags` × `folders` for arbitrary slice-and-dice the API doesn't expose together. |
| 7 | Conversion funnel | `funnel --link <key> --interval 30d` | 7/10 | Click→lead→sale ratios per link or campaign. Local-store join across `events` × `track/lead` × `track/sale`. |
| 8 | Customer journey timeline | `customers journey <id>` | 6/10 | Every link a customer clicked + when they became lead + when they purchased, in one timeline. |
| 9 | Partner leaderboard | `partners leaderboard --by commission` | 7/10 | Joins `/partners/analytics` (NEW) × local `commissions` × `payouts` for accurate per-partner ROI. |
| 10 | Commission audit reconciliation | `partners audit-commissions` | 8/10 | Cross-resource reconcile to flag stale rates, missing payouts, expired bounties still earning. |
| 11 | Workspace health doctor | `health` | 9/10 | Monday-morning report: rate-limit headroom, expired-but-active links, dead destinations, unverified domains, dormant tags, **bounty submissions awaiting review** (NEW). |
| 12 | Time-windowed change feed | `since 24h` | 7/10 | "What happened in the last N hours?" Local timestamps power created/updated/deleted links + recent partner approvals + new bounty submissions. |
| 13 | **Bounty submission triage queue** | `bounties triage` | 8/10 | **NEW.** Groups partner-submitted proof by status × age × bounty type. Surfaces awaiting-review backlog. Direct ops value from new bounty endpoints. |
| 14 | **Bounty payout projection** | `bounties payout-projection --window 30d` | 7/10 | **NEW.** Projects upcoming payouts from approved-but-unpaid submissions × current commission rates. Joins `bounties` × `commissions` × `payouts`. |

All 14 transcendence features score **>= 6/10**. Total: **14 transcendence + 53 absorbed = 67 commands.**

---

## Stub policy

**No stubs.** Every absorbed command and every transcendence command above is shipping-scope. If the generator emits a command body that doesn't compile, we fix it; we do not ship `// TODO: not yet wired` placeholders.

---

## Coverage comparison

| Surface | Operations covered | % of spec |
|---------|-------------------|-----------|
| Official `dub-cli` | ~3-4 | ~7% |
| `gitmaxd/dubco-mcp` | 4 | ~7% |
| `sujjeee/dubco` (stale) | ~3 | ~6% |
| **`dub-pp-cli` (this build)** | **53 absorbed + 14 novel** | **100% + transcendence** |

---

## Reachability + economics

- API confirmed live with user's key (HTTP 200 on `/links` against real workspace).
- Workspace-scoped key isolates blast radius per workspace; no cross-workspace contamination.
- Free tier 60/min covers Phase 5 dogfood comfortably; analytics endpoints get adaptive backoff in the generated client.
- Every test link in Phase 5 will be tagged `pp-test-<run-id>` and cleaned up after the run.
