# Zoho Campaigns Absorb Manifest

Sources absorbed: MCPBundles hosted MCP (~20 tools, closed), Vinkius hosted MCP (13 tools, closed), keepsuit/laravel-zoho-campaigns (PHP SDK, contacts/tags), dltHub recipe, official docs. No open-source CLI/MCP/npm/PyPI exists — this CLI is first-mover.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List recent campaigns w/ status filter | MCPBundles MCP | (generated endpoint) campaigns recentcampaigns | offline mirror, --json/--select |
| 2 | Recently sent campaigns | MCPBundles MCP | (generated endpoint) campaigns recentsentcampaigns | agent-native output |
| 3 | Campaign details | MCPBundles MCP | (generated endpoint) campaigns getcampaigndetails | typed flags |
| 4 | Campaign performance report (opens/clicks/bounces/unsubs/geo) | MCPBundles MCP | (generated endpoint) campaigns campaignreports | snapshot-to-SQLite on read |
| 5 | Last campaign report | MCPBundles MCP | (generated endpoint) campaigns getlastcampaignreport | quick health pulse |
| 6 | Per-recipient engagement data (opened/clicked/bounced/optout) | MCPBundles MCP | (generated endpoint) campaigns getcampaignrecipientsdata | action enum flag |
| 7 | Create campaign | MCPBundles MCP | (generated endpoint) campaigns createcampaign | --dry-run |
| 8 | Send campaign | MCPBundles MCP | (generated endpoint) campaigns sendcampaign | --dry-run |
| 9 | Schedule campaign | MCPBundles MCP | (generated endpoint) campaigns schedulecampaign | --dry-run |
| 10 | Clone campaign | MCPBundles MCP | (generated endpoint) campaigns clonecampaign | |
| 11 | Delete campaign | MCPBundles MCP | (generated endpoint) campaigns deletecampaign | --dry-run |
| 12 | List mailing lists + counts | MCPBundles MCP / keepsuit SDK | (generated endpoint) contacts getmailinglists | offline mirror |
| 13 | List advanced details | docs | (generated endpoint) contacts getlistadvanceddetails | |
| 14 | List subscribers w/ status+pagination | keepsuit SDK (source) | (generated endpoint) contacts getlistsubscribers | offline mirror, FTS |
| 15 | Subscriber count by status | keepsuit SDK (source) | (generated endpoint) contacts listsubscriberscount | |
| 16 | Subscribe contact w/ custom fields | keepsuit SDK (source) | (generated endpoint) contacts listsubscribe | contactinfo encoding handled |
| 17 | Unsubscribe contact | keepsuit SDK (source) | (generated endpoint) contacts listunsubscribe | |
| 18 | Do-not-mail registry move | MCPBundles MCP | (generated endpoint) contacts contactdonotmail | |
| 19 | Bulk add contacts to list | MCPBundles MCP | (generated endpoint) contacts addlistcontactsinbulk | |
| 20 | Create list with contacts | docs | (generated endpoint) contacts addlistandcontacts | |
| 21 | Update / delete mailing list | Vinkius MCP | (generated endpoint) contacts updatelistdetails + deletemailinglist | --dry-run |
| 22 | Contact field schema | keepsuit SDK (source) | (generated endpoint) contacts getallcontactfields | |
| 23 | Segment details + contacts | docs | (generated endpoint) contacts getsegmentdetails + getsegmentcontacts | |
| 24 | Tag CRUD + associate/deassociate | keepsuit SDK (source) | (generated endpoint) tags getalltags/add/delete/associate/deassociate | |
| 25 | Offline sync/search/SQL over campaigns+lists+subscribers | (none has this) | (behavior in zoho-campaigns-pp-cli sync) framework sync/search/sql/analytics | first tool with local store |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Report delta | delta <campaignkey> [--since 7d] | 10/10 | hand-code | Diffs time-stamped rows of the local campaign_report_snapshots table; no Zoho endpoint returns historical diffs. | Use this command to see how one campaign's metrics changed between snapshots. Do NOT use it for org-wide rollups across campaigns; use 'digest' instead. |
| 2 | Org digest | digest [--since 30d] | 9/10 | hand-code | Aggregates synced campaigns + latest report snapshots + list counts into one rollup; the dashboard/daily-brief feed. | Use this command for an org-wide summary of campaign and list performance over a window (including 24h windows for daily briefs). Do NOT use it for a single campaign's change over time; use 'delta' instead. Do NOT use it for list-size trend lines; use 'growth' instead. |
| 3 | List growth | growth [--list <listkey>] [--since 90d] | 9/10 | hand-code | Reads periodic listsubscriberscount snapshots to compute per-list net growth/unsub/bounce trends; Zoho exposes only current counts. | Use this command for mailing-list size and health trends over time. Do NOT use it for campaign open/click performance; use 'digest' instead. |
| 4 | Engagement ranking | engagement [--top N] [--never-opened] [--list <listkey>] | 8/10 | hand-code | Joins recipient-action rows across ALL campaigns to rank contacts; recipientsdata is per-campaign only. | Use this command for ranked cross-campaign contact engagement (most engaged, never opened). Do NOT use it for one contact's full history; use 'journey' instead. |
| 5 | Bounce audit | bounce-audit [--since 90d] | 8/10 | hand-code | Joins bounce actions to current list membership; emits cleanup candidates pipeable into contacts contactdonotmail. | Use this command to find bounced contacts and deliverability cleanup candidates. Do NOT use it for engagement ranking of healthy contacts; use 'engagement' instead. |
| 6 | Contact journey | journey <email> | 7/10 | hand-code | Chronological per-contact history across every synced campaign; no endpoint or incumbent returns the contact-centric inverse. | Use this command for one contact's chronological history across all campaigns. Do NOT use it to rank or list many contacts; use 'engagement' instead. |

Notes:
- All transcendence commands are `// pp:data-source local`, call hintIfUnsynced/hintIfStale, use drain-first SQLite pattern.
- Rate-limit protection (500/5min, 30-min lockout) is client-internal throttling behavior, not a command (killed C12 quota — value absorbed into sync client).
- No stubs. Zero rows are (stub).
