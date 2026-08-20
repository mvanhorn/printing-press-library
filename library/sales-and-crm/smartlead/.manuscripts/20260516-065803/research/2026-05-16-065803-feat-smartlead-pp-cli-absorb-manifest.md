# SmartLead CLI — Absorb Manifest

## Ecosystem scanned
- **LeadMagic smartlead-mcp-server** (TS, ~122 tools) — official-partner MCP. Tool
  ceiling, but ~half its tools wrap UI features not in the public REST API.
- **bcharleson/smartlead-cli** (TS, ~41 cmds) — closest existing model: JSON-first,
  agent-parseable, login-config. UX bar to match and beat.
- **smartlead-ai/API-Python-Library** (Python, 40 methods) — API-faithful reference.
  Every method maps to a real documented endpoint. Authoritative path source.
- **LeadMagic/cold-email-cli** (TS, archived) — multi-platform; naming reference only.
- **jonathan-politzki/smartlead-mcp-server**, **lkm1developer/...**, n8n node — minor.
- No standalone competing CLI for Instantly/Lemlist/Apollo exists. No real competitor.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List campaigns | Python `campaigns.all` | `campaigns list` | SQLite-mirrored, offline, FTS |
| 2 | Get campaign | Python `campaigns.get` | `campaigns get` | --json --select, cached |
| 3 | Create campaign | Python `campaigns.create` | `campaigns create` | --dry-run, typed exit codes |
| 4 | Delete campaign | Python `campaigns.delete` | `campaigns delete` | --dry-run guard |
| 5 | Start/pause/stop campaign | Python `campaigns.update_status` | `campaigns status` | typed START/PAUSED/STOPPED enum |
| 6 | Update campaign settings | Python `campaigns.update_settings` | `campaigns settings` | typed track/stop-lead enums |
| 7 | Update campaign schedule | docs `POST /schedule` | `campaigns schedule` | typed days/timezone flags |
| 8 | Get sequences | Python `campaigns.get_sequences` | `sequences get` | offline cache |
| 9 | Save sequences | Python `campaigns.save_sequences` | `sequences save` | --stdin JSON body (complex body) |
| 10 | List campaign email accounts | Python `get_all_email_accounts` | `campaigns email-accounts` | offline join |
| 11 | Attach email accounts | Python `add_email_accounts` | `campaigns attach-accounts` | --dry-run |
| 12 | Detach email accounts | Python `remove_email_accounts` | `campaigns detach-accounts` | --dry-run |
| 13 | List campaign leads | Python `get_leads` | `leads list` | paginated, mirrored, FTS |
| 14 | Add leads to campaign | Python `add_leads_to_campaign` | `leads add` | --stdin JSON list (complex body) |
| 15 | Delete lead from campaign | Python `delete_lead_from_campaign` | `leads remove` | --dry-run |
| 16 | Unsubscribe lead (campaign) | Python `unsubscribe_lead_from_campaign` | `leads unsubscribe` | --dry-run |
| 17 | Update lead category | Python `update_lead_category` | `leads category` | typed |
| 18 | Lead message history | Python `get_lead_message_history` | `leads history` | thread cached + searchable |
| 19 | Export campaign leads | Python `export_leads_data` | `leads export` | --csv passthrough |
| 20 | Campaign statistics | Python `get_statistics` | `analytics statistics` | email_status enum filter |
| 21 | Analytics by date range | Python `get_statistics_date_range` | `analytics by-date` | snapshot for drift |
| 22 | Top-level campaign analytics | Python `get_top_level_analytics` | `analytics campaign` | snapshotted |
| 23 | List campaign webhooks | Python `get_webhooks` | `webhooks list` | offline |
| 24 | Upsert campaign webhook | Python `update_webhook` | `webhooks upsert` | typed event/category enums |
| 25 | Delete campaign webhook | Python `delete_webhook` | `webhooks delete` | --dry-run |
| 26 | Get lead by email | Python `leads.get_by_email_address` | `leads find` | also hits local FTS |
| 27 | Lead's campaigns | Python `leads.get_campaigns` | `leads campaigns` | offline join |
| 28 | List lead categories | Python `leads.get_categories` | `leads list-categories` | cached |
| 29 | Unsubscribe lead globally | Python `unsubscribe_from_all_campaigns` | `leads unsubscribe-all` | --dry-run |
| 30 | Add domain block list | Python `add_to_block_list` | `leads block-domains` | --dry-run |
| 31 | Update lead | Python `leads.update` | `leads update` | --stdin JSON body (complex body) |
| 32 | List email accounts | Python `email_accounts.all` | `email-accounts list` | mirrored, FTS |
| 33 | Create email account | Python `email_accounts.create` | `email-accounts create` | --dry-run |
| 34 | Update email account | Python `email_accounts.update` | `email-accounts update` | --dry-run |
| 35 | Set warmup settings | Python `set_warmup_settings` | `email-accounts warmup` | typed |
| 36 | Get warmup stats | docs `GET /warmup-stats` | `email-accounts warmup-stats` | snapshotted for trend |
| 37 | Reconnect failed accounts | Python `reconnect_failed_email_accounts` | `email-accounts reconnect` | --dry-run |
| 38 | Create client | Python `clients.create` | `clients create` | typed permission enum |
| 39 | List clients | Python `clients.all` | `clients list` | mirrored |
| 40 | Local store sync | (no tool has this) | `sync` | full snapshot of all entities to SQLite |
| 41 | Offline search / SQL | (no tool has this) | `search`, `sql` | FTS5 + raw SELECT over the mirror |

Notes:
- bcharleson `inbox reply` / `message-history` overlap with #18; covered.
- LeadMagic Smart Delivery / Smart Senders / extended-analytics tools (~50)
  wrap SmartLead UI features **not exposed by the public REST API**. Not absorbed
  — there is no endpoint to call. Documented here so the user knows the gap is in
  SmartLead's API, not this CLI.
- Endpoints #9, #14, #31 have free-form/array-of-object bodies — shipped with a
  `--stdin` / `--body-file` JSON path (no per-field flags). Not stubs; fully wired.

## Transcendence (only possible with our local-store approach)

Final set — 6 survivors from the Phase 1.5c.5 adversarial-cut subagent (scores 7-9/10).

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| 1 | Campaign health scorecard | `health [--campaign <id>]` | 9 | Joins local campaigns + statistics + leads + email_accounts into one bounce%/reply%/silent-count/stale scorecard; API needs 4+ calls per campaign |
| 2 | Silent-lead finder | `silent --campaign <id> --days <n>` | 9 | Time-window query over local send/open/reply event timestamps; no API endpoint answers "sent but no reply in N days" |
| 3 | Cross-campaign dupe + domain ledger | `dupes [--email <a>] [--domain <d>]` | 9 | Scans the whole local leads mirror for emails/domains in 2+ campaigns; `--domain` prints the pitch ledger for one site. Cross-campaign join the campaign-scoped API cannot do |
| 4 | Sender health ranking | `sender-health` | 8 | Ranks email_accounts by a composite of warmup-stats + bounce rate + reply rate, with bounce concentration by domain — local fleet-wide ranking |
| 5 | Warmup-readiness gate | `warmup-gate [--account <id>]` | 7 | Applies warmup thresholds to local warmup-stats and returns a typed pass/fail exit code — a scriptable CI-style launch gate |
| 6 | Reply-rate drift | `drift --campaign <id> [--weeks <n>]` | 7 | Week-over-week reply/open/bounce deltas from successive synced statistics snapshots; the API does not retain history |

Killed (folded into survivors or too thin): seq-audit, triage, stale (→ health flag),
bounce-map / sender-load (→ sender-health columns), cat-breakdown, preflight, thread,
domain-log (→ dupes --domain). Full audit trail in the novel-features-brainstorm doc.
