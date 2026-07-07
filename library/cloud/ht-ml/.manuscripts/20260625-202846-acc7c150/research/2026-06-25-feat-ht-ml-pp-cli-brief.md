# ht-ml.app CLI Brief

## API Identity
- Domain: HTML hosting with an API built for AI agents. POST one HTML document, get a public URL at `https://{site_id}.ht-ml.app/` instantly. No accounts, no signup, no global API key.
- Base URL: `https://api.ht-ml.app/v1` (self-describes at `/v1/help`; LLM doc at `/llms.txt`).
- Users: AI coding agents (Claude Code, Codex, OpenClaw, Hermes) and the people they act for. Use cases: prototypes, architecture diagrams, code reviews, decks, status reports, illustrations.
- Data profile: **sites** (one HTML document each, served on own subdomain), **assets** (images/videos referenced by the HTML), **update_key** (per-site bearer secret returned once at creation). Optional per-site password.

## The contract (authoritative, from /v1/help + /llms.txt + docs — all three agree)
- `POST /v1/sites` — no auth. Body `{"html_content": "<...>", "password": "<optional>"}` → `{site_id, update_key, url, status}`. HTML content-scanned (422 on fail).
- `GET /v1/sites/{site_id}` — no auth (read public). Returns `status`, referenced `assets` list (with `missing` flag), next-step endpoints. Password-protected sites return 401 unless `ht_ml_pwd=<password>` cookie sent.
- `PUT /v1/sites/{site_id}` — `Authorization: Bearer <update_key>`. Body `{"html_content": "<...>", "password": "<optional>"}`. Replaces HTML, CDN invalidated immediately. password: include=set, `""`=clear, omit=leave unchanged.
- `POST /v1/sites/{site_id}/assets?relative_path=PATH` — `Authorization: Bearer <update_key>`. Body `multipart/form-data` file. Asset must already be referenced in the HTML (403 if not).
- `GET /v1/help` — self-describing JSON (endpoints + error codes).
- Errors are conversational: every error includes an actionable `message`. 401 (write: bad key / read: needs password cookie), 403 (asset not referenced or wrong key), 422 (HTML failed safety scan).
- No DELETE. No list-assets endpoint. **No list-sites endpoint (no accounts).**

## Reachability Risk
- None. `GET /v1/help` → 200, landing → 200. Official API, AI crawlers explicitly welcomed in robots.txt. Reachable from plain HTTP (`curl`), no bot-protection.

## Auth model (unusual — drives the whole design)
- No global credential. Create + read are unauthenticated.
- Writes (update, asset upload) use a **per-site `update_key`**, returned ONLY ONCE at creation. No recovery endpoint. Lose it → the site is orphaned (read-only forever).
- Spec auth.type = none; write endpoints carry a Bearer header the CLI must supply per-site. The local store is the only place that key can live.

## Top Workflows
1. **Publish**: HTML file (or stdin/string) → live URL. The #1 flow.
2. **Publish with assets (one shot)**: parse HTML for referenced images/videos, create the site, then upload every referenced local asset automatically (the documented 3-step create→discover-missing→upload flow, collapsed to one command).
3. **Update**: change a site's HTML by `site_id` without the human ever handling the `update_key` (CLI resolves it from the store).
4. **Track / recall**: list every site I've published, search their content, recover a `url` or `update_key` I'd otherwise have lost.
5. **Protect**: set / clear / rotate a per-site password; remember the shared secret locally.

## Table Stakes (match the nsmith/html skill + WebMCP tool)
- create site, update site, upload referenced assets, optional password — all present in the `nsmith/html` Agent Skill and the WebMCP `publish_html_site` tool.
- 20 ready-made page templates (slide-deck, status-report, incident-report, implementation-plan, annotated-pr, pr-writeup, module-map, code-approaches, visual-designs, design-system, component-variants, animation-sandbox, clickable-flow, flowchart, svg-figure-sheet, feature-explainer, concept-explainer, triage-board, feature-flags, prompt-tuner). We match with a built-in starter-template set + `new --template`.
- `GET /v1/help` self-description → we mirror as `ht-ml help-api` / `doctor`.
- Conversational/actionable errors → we surface the API's `message` verbatim and map to typed exit codes.

## Data Layer (the reason this CLI should exist)
- **sites**: `site_id` (PK), `url`, `update_key` (secret), `status`, `title` (extracted from `<title>`), `password` (if set), `html` snapshot, `created_at`, `updated_at`, `last_synced_at`. FTS over title + html.
- **assets**: `site_id`, `relative_path`, `status` (present/missing), `uploaded_at`.
- **versions**: `site_id`, `version`, `html`, `created_at` (history for diff/rollback).
- Sync cursor: per-site `GET /v1/sites/{id}` refreshes `status` + asset list (no global list endpoint, so "sync" is per-known-site).
- Search/FTS: FTS5 over stored HTML + titles — find "that deck I published last week" with no API support for it.

## Codebase Intelligence
- Competitor `nsmith/html` (GitHub, MIT, ~3 stars, pushed 2026-06-16): an Agent *Skill* (not a CLI). Workflow create/update/assets/password + 20 templates + `scripts/publish.sh`. No local state, no registry, no search, no history. We absorb the workflow + template concept and beat it with persistence.
- WebMCP browser tool `publish_html_site` (create-only, in-browser). We beat it with the full lifecycle + offline registry.

## User Vision
- Not provided ("Let's go"). Adjacent to the user's `here-now` CLI (publish folder → URL); ht-ml.app is the lighter, accountless, single-HTML-doc, agent-native cousin. The registry/recall angle mirrors how `here-now` tracks expiring sites.

## Product Thesis
- Name: **ht-ml.app CLI** (slug `ht-ml`, binary `ht-ml-pp-cli`).
- Why it should exist: ht-ml.app is deliberately stateless and accountless. That makes it frictionless to publish and impossible to manage. This CLI is the missing memory layer: it remembers every site you published, keeps the once-only `update_key` safe, lets you update by id without ever touching the key, auto-uploads referenced assets in one command, versions every change, and searches your whole published history offline. It is the only ht-ml.app tool that survives losing an `update_key`.

## Build Priorities
1. **Data layer + capture**: every create/update writes site_id, url, update_key, status, html, title to SQLite. Foundation for everything.
2. **publish** (file/stdin/string, optional `--password`, `--assets` auto-upload) and **update** (by site_id, key auto-resolved).
3. **list / get / search / open** — the registry surface the API can't provide.
4. **assets** (upload by referenced path; auto-discover missing) + **password** (set/clear/rotate).
5. **Transcendence**: history/diff/rollback, key vault export/import (disaster recovery), orphan/missing-asset doctor, templates, bulk publish.
