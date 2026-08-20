# Resend CLI Brief

## API Identity
- **Domain:** transactional + broadcast email API (developer-first, React Email native)
- **Users:** modern startup engineers (Vercel/Next.js heavy), application notification systems, marketing teams sending lifecycle campaigns
- **Data profile:** emails (sent/delivered/opened/clicked/bounced), audiences, contacts, broadcasts, domains, API keys, templates, segments, topics, automations, webhooks, logs, events — 14 resource categories, 100 ops across 49 paths

## Reachability Risk
- **None.** Plain HTTPS REST at `https://api.resend.com` with Bearer auth (`Authorization: Bearer re_…`). No Cloudflare/WAF challenges, no anti-bot headers. `net/http` is the canonical transport — every official SDK uses it. Documented 429 with `Retry-After`.

## Spec Source
- **Canonical:** `https://raw.githubusercontent.com/resend/resend-openapi/main/resend.yaml` (v1.5.0, Feb 23 2026)
- **JSON sibling:** `resend.json` (auto-generated from the YAML, identical content)
- **Repo:** `github.com/resend/resend-openapi`
- **Counts:** 100 operations / 49 paths / 14 resource categories / 39 POST / 37 GET / 13 DELETE / 11 PATCH

## Top Workflows
1. **Send a transactional email** — `POST /emails` — the workhorse (signup confirmations, password resets, receipts). React/HTML/text bodies, attachments, scheduled_at, tags.
2. **Batch-send (up to 100/call)** — `POST /emails/batch` — digest emails, per-user notification fanout.
3. **Verify a sending domain** — `POST /domains` → `POST /domains/{id}/verify` → `GET /domains/{id}`. Onboarding pain point #1.
4. **Inspect delivery state** — `GET /emails/{id}`, `GET /logs`, `GET /events` — replaces dashboard for ops debugging.
5. **Broadcast to an audience** — `POST /audiences` → `POST /contacts` (in audience) → `POST /broadcasts` → `POST /broadcasts/{id}/send` (newsletter / product-launch flow).

Honorable mentions: API-key rotation, webhook setup, contact CSV import (currently hand-rolled), template create/publish/duplicate.

## Data Layer
- **Primary entities to sync:** emails, emails_events, emails_received, domains, audiences, contacts, broadcasts, templates, api_keys, segments, topics, contact_properties, webhooks, logs, automations, automation_runs, events
- **Sync cursors:** every resource exposes `created_at` + pagination; live state (opens/clicks/bounces) requires periodic `/logs` or `/emails/{id}` re-fetch (no event-since cursor on `/events`).
- **FTS columns:**
  - `emails`: subject + to + tags + html_excerpt
  - `contacts`: email + first_name + last_name
  - `broadcasts`: name + subject + html_excerpt
  - `templates`: name
  - `domains`: name

## Competitive Pain Points
- **No scheduled-send rescheduling** beyond cancel-and-resend.
- **No native CSV contact import** — every team hand-rolls a loop.
- **Dashboard analytics window capped at 30 days**, line-chart only. Tinybird/Reflex tutorials exist because deeper analytics are missing.
- **Suppression/bounce lists aren't queryable in aggregate** — deliverability blind spot.
- **Audience segments are limited** vs SendGrid contact-list segments or Mailgun mailing-lists with merge tags.
- **No template-render-with-vars endpoint** — React Email runs client-side only.
- **Common dev questions Resend can't answer in one call:**
  - "Why didn't email X arrive?" (cross-event timeline split across /emails/{id} + /logs)
  - "Which API key sent that?" (logs don't show)
  - "How many of my contacts are subscribed to topic X?" (no rollup endpoint)
  - "Show me everything sent to alice@…" (no recipient filter)

## Product Thesis
- **Name:** `resend-pp-cli`
- **Why it should exist:** the only Resend interface that answers cross-resource questions ("show every email to alice in the last week", "which audiences is bob in", "what's our 7-day bounce rate") because it syncs Resend state into a local SQLite store. The official CLI is one-shot per command; the SDK gives you primitives but no rollups; the MCP exposes 49 endpoint mirrors with the same one-shot shape.

## Build Priorities
1. **Foundation:** ingest the 100-op spec, generate typed clients for all 14 resources, sync into SQLite with FTS, expose `sql` / `search` / `context` built-ins.
2. **Absorbed table-stakes:** every endpoint as a typed Cobra command + MCP tool — match the official 53-cmd CLI + 49-tool MCP surface; `--dry-run` on send/broadcast; `--profile` multi-workspace; auto-JSON on pipe; `doctor` / `whoami`.
3. **Novel cross-resource:** the 8 highest-leverage rollups (emails search/to/timeline, audiences inventory, contacts where, broadcasts performance, domains health, deliverability summary).
4. **Polish:** csv-import dry-run, `webhooks listen` parity, send-side ergonomics (markdown send, batch from CSV, retry-failed).
