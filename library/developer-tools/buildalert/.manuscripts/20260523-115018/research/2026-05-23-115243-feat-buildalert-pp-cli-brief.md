# BuildAlert CLI Brief

## API Identity
- Domain: UK construction lead generation — every planning application from 400+ UK councils, normalized, filtered, and surfaced with applicant addresses (subscriber-only) and agent details
- Users: builders, loft-conversion specialists, structural engineers, scaffolders, roofers, window/bifold suppliers, refurb/extension contractors — the same trades ZAZU targets
- Data profile: planning applications (~20K/week), each with description, project type, location/radius, estimated value, supporting docs (drawings), applicant address (subscribers), agent details (subscribers), and an outreach surface (letter/postcard send, follow-up scheduling, ROI logging)
- Site: https://www.buildalert.uk (Next.js, CloudFront). Dashboard at https://www.buildalert.uk/dashboard/leads
- No public OpenAPI / documented API. Browser-sniff is the path.

## Reachability Risk
- Low to Medium. Marketing pages return 200 OK, no Cloudflare/WAF challenge headers detected on `HEAD /dashboard/leads`. CloudFront protects the static surface but the JSON-returning backend behind `/dashboard/*` is the actual capture target. Once a session cookie is present, the backend (Next.js BFF) should serve JSON or RSC payloads cleanly. No GitHub issues exist (this is a SaaS, not an open project).

## User Vision
- (No explicit briefing given; inferred from project memory: user is the ZAZU builder, treats BuildAlert as a peer lead source. The CLI should be ZAZU-aware — feed BuildAlert leads into bd-mirror.sqlite, dedup against existing applications, surface unique-to-BuildAlert leads, and let the user track BuildAlert letter spend.)

## Top Workflows
1. **Pull today's matched leads** — list new planning applications BuildAlert has matched to my saved filters since I last checked
2. **Filter dashboard by project type + radius + min-value** — e.g. all extension projects within 30 miles, min £40K, posted last 7 days
3. **Drill into a lead** — read the planning description, fetch supporting docs (drawings), get applicant address + agent details
4. **Send a branded letter to a lead** — pay-as-you-go £2/letter or use included subscription credits; schedule a follow-up
5. **Track letter campaigns** — see what's been sent, what's pending, what responses have come back, and how much I've spent
6. **Cross-reference BuildAlert leads with ZAZU's bd-mirror** — find leads BuildAlert has that ZAZU's scrapers missed, or vice versa

## Table Stakes (anything a competent BuildAlert user does on the web today)
- List leads with filters (project type, value range, radius from postcode, keyword)
- Save a filter as a "smart alert" / saved search
- View lead detail incl. supporting docs (drawings, planning statements, design and access)
- Send a letter from a template
- Schedule a follow-up letter
- View letter history (sent / scheduled / responded)
- View account credit balance / subscription tier
- View ROI tracking (responses, project values logged against sent letters)
- Browse free (non-subscriber) view — limited fields
- Switch between subscription tiers (Standard/Business/Premium)

## Data Layer
- Primary entities:
  - `applications` (the lead — planning ref, council, description, project_type, project_value_estimate, applicant_address, agent_details, lat/lng, posted_date, decision_date, source_doc_urls)
  - `filters` (saved search definitions — name, project_types[], min_value, max_value, postcode, radius_miles, keywords)
  - `letters` (campaign — template_id, application_id, status [draft/queued/printing/sent/responded], cost, scheduled_for, sent_at, response_logged_at)
  - `templates` (letter templates — id, name, body, header_image)
  - `responses` (ROI tracking — letter_id, response_type, project_value_realized, notes)
- Sync cursor: by `posted_date` (per-filter) or `last_seen_application_ref`
- FTS/search: index `applications.description`, `applications.applicant_address`, `applications.agent_details`, council_name → fast offline keyword search across pulled leads

## Codebase Intelligence
- BuildAlert is a closed SaaS (Next.js frontend on CloudFront/AWS), no public source code, no MCP server, no community CLIs, no DeepWiki entry. Skip Step 1.5a.5 / 1.5a.6.
- Browser-sniff is the only discovery path. Expect: Next.js Server Actions, RSC payloads, REST or tRPC at `/api/*`, JSON responses behind session cookie.

## Product Thesis
- **Name:** `buildalert-pp-cli` (slug `buildalert`)
- **Why it should exist:**
  - BuildAlert is a paid SaaS for one of the highest-frequency workflows a UK builder runs — daily lead triage. The web dashboard is fine for browsing; it's painful for sub-second filter chains, bulk export, dedup against your existing pipeline, or cron-driven sync. The CLI takes BuildAlert leads offline, makes them queryable with SQL/FTS, scriptable via JSON, and pipeable into other tools (ZAZU bd-mirror.sqlite is the headline pipe).
  - Critical differentiator for this user: **a CLI is the only way to bridge BuildAlert's curated leads into ZAZU's bd-mirror.sqlite** so the same building/owner isn't mailed twice across systems.
- **Headline:** Every BuildAlert lead at your fingertips offline — filter, dedupe, and pipe to your own pipeline before the competition even sees the alert.

## Build Priorities
1. **Discovery via browser-sniff** — capture an authenticated session, identify the Next.js / JSON endpoints behind `/dashboard/leads`, `/dashboard/filters`, `/dashboard/letters`, `/dashboard/account`. Generate a spec.
2. **Sync + local store** — pull leads with filters, persist to SQLite (`applications`, `letters`, `filters`, `templates`), provide FTS on `description` + `applicant_address`.
3. **Absorb the dashboard** — list leads, filter, view detail, list filters, list letters, view templates, view account/credit, send letter (with `--dry-run` because £2 is real money).
4. **Transcendence (ZAZU-aware)** — dedup against ZAZU's bd-mirror.sqlite, surface BuildAlert-unique leads, cost tracker, follow-up alerter, ROI roll-up.

## Open Questions / Risks
- The `/dashboard/leads` endpoint shape: REST vs RSC vs Server Action. Browser-sniff will resolve this.
- Whether anonymous browsing returns enough data to be useful (subscribers get applicant address; non-subscribers don't).
- Rate-limiting policy of the BuildAlert backend — unknown until sniff. Need to add `cliutil.AdaptiveLimiter` to any sibling client.
- Cookie/session expiry. Likely a Next.js session cookie + CSRF token. Long-lived but may need `auth login --chrome` for refresh.
- Letter-send is a real money-spending mutation. The CLI must default to `--dry-run` for `letter send`; an explicit `--confirm` (and a `--mock` for tests) is mandatory.
