# EmailOctopus Novel Features Brainstorm (audit trail)

## Customer model

**Indira "Indie" Kapoor — solo newsletter operator.**

Today: She runs a 4,200-subscriber paid newsletter on EmailOctopus, paying $24/mo because she crossed the free-tier ceiling. She has a Cloudflare Workers app capturing signups from her Astro site and pushing them to EmailOctopus via cURL in a one-off script she wrote at 1am.

Weekly ritual: Sunday afternoon she pulls a CSV out of Stripe, manually dedupes against her EmailOctopus list in a Google Sheet, then re-uploads via the EmailOctopus web dashboard's bulk-import tool because the API has no native import-from-CSV path that handles upsert + tagging + custom fields atomically. Tags churned customers, untags reactivated ones.

Frustration: There is no way to ask "which of my subscribers haven't opened anything in 90 days" without paginating every campaign report and joining client-side. She has tried three v1-era Ruby/PHP wrappers from GitHub and all of them 401 because her API key was minted post-Oct 2024.

**Marcus Chen — small-SaaS growth engineer.**

Today: Marcus runs growth at a 12-person Series-A devtools company. EmailOctopus is wired to lifecycle automations: trial-start, trial-end, churn, activation milestones. He triggers automations from a Rails app via the `/automations/{id}/queue` endpoint inside a Sidekiq job.

Weekly ritual: Monday morning he pulls last-week campaign performance into a Notion doc for the founders. The pull is a screenshot from the EmailOctopus dashboard because there's no built-in export. He also runs a Friday "stale-trial reactivation" job that needs the intersection of (tagged trial-started >14d ago) AND (no opens in last 7 days) — currently a 200-line Ruby script.

Frustration: He can't grep "which contacts are tagged X but not Y" without pulling every contact and filtering in Ruby. Rate limits bite hard during onboarding bursts. The Zapier MCP burns 2 tasks per contact lookup — economically unusable past a few hundred contacts/day.

**Priya Desai — agency operator with multiple client lists.**

Today: Priya runs a 3-person agency managing email for 18 SMB clients, each with their own EmailOctopus account (so 18 API keys). Most lists are 500-5,000 subscribers. She bills hourly so every minute spent in a vendor dashboard eats margin.

Weekly ritual: Friday she runs a "monthly report" sweep for each client — pull campaign summary, top 10 clicked links, contact open-rate distribution, dump into a templated Google Doc. Manual today, 4 hours of repetitive UI clicks.

Frustration: Switching between 18 dashboards. Custom-field schema drift across clients (one uses `first_name`, another `FirstName`). No way to diff "what changed in this list since last week" without keeping her own snapshots.

**Theo Brandt — agent driver (Claude Desktop / Codex user).**

Today: Theo is a power-user of Claude Desktop with ~30 MCP servers wired in. He uses Claude to manage his personal newsletter (~600 subs) and ad-hoc data pulls for his consulting clients. The Zapier EmailOctopus MCP gave him 6 contact-only tools, none of which can read campaign reports or do anything outside contact CRUD.

Weekly ritual: Asks Claude things like "show me opens for last Tuesday's campaign" or "find subscribers who clicked the pricing link." Today Claude either refuses ("I don't have a tool for that") or hallucinates a response.

Frustration: Agent surface is anemic. Wants the *whole* API plus the local-store joins that the API can't do, behind MCP, without 2-task-per-call Zapier billing.

## Candidates (pre-cut)

(See subagent run — 16 candidates total; 8 survived adversarial cut.)

## Killed candidates

| # | Feature | Reason | Sibling that ate it |
|---|---------|--------|---------------------|
| 2 | `contacts stale --days N` | Pure subset of #1 engagement scorecard with a filter flag | #1 — `contacts engagement --inactive-since 90d` is the same query |
| 6 | `contacts domains` | Useful but niche; one-off pivot rather than weekly ritual; collapsible into `campaigns digest --by-domain` and standard SQL via `sql` built-in | #3 digest covers the campaign-context version; built-in `sql` covers ad-hoc |
| 7 | `tags taxonomy` | Cross-list tag report is Priya-only and largely answerable by `sql` over the tags table; weak weekly-use proof outside agency persona | Built-in `sql` |
| 10 | `campaigns leaderboard` | Marcus-only monthly cadence (not weekly); same data as `campaigns list --json \| sort` or one-line `sql` query; weak transcendence vs the table-stakes `campaigns list` | Built-in `sql` and #3 digest |
| 11 | `fields coverage` | Niche maintenance task, not a recurring workflow; one `sql` query answers it; no research-backed pain | Built-in `sql` |
| 13 | `contacts churn --window 30d` | Heavy overlap with #4 `lists diff`; churn-classification is the same diff with relabeled columns | #4 `lists diff` |
| 14 | `campaigns links top --since 30d` | One-pivot query; subsumed by `campaigns digest` (per-campaign) and `sql` (cross-campaign); not weekly-ritual material | #3 digest + built-in `sql` |
| 16 | `contacts find <query>` | Duplicates generator's built-in `search` command; no contact-specific affordance worth a separate command | Built-in `search` |
