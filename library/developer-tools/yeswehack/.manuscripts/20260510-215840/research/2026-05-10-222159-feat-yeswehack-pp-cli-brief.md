# YesWeHack CLI Brief

## API Identity
- Domain: Bug bounty platform (European-headquartered, GDPR-aware). Researcher side ("hunters") browses programs, reads scope, submits reports, tracks bounties, learns from disclosed reports. Program/Business Unit side runs the programs, triages reports, pays bounties.
- Users: This CLI targets the researcher (hunter). Program-manager use cases are a non-goal because the user's account is a hunter account and PATs are gated to manager roles.
- Data profile: Hundreds of programs (each with scope items, in-scope/out-of-scope URLs, payout grids, languages, severity allowed). Tens of thousands of reports per researcher across the platform. Hacktivity feed of disclosed reports for learning. Per-program credential pools and email aliases.

## Reachability Risk
- Low for the website (yeswehack.com): standard HTML, no Cloudflare challenge observed.
- Low for api.yeswehack.com: works with JWT bearer or cookie auth from the user's session. Same pattern that YesWeBurp, YesWeCaido, ywh_program_selector, and yeswehack-mcp all depend on, no public reports of widespread breakage.
- Auth tiering risk: the platform has THREE auth surfaces. Picking the wrong one wastes work.
  1. Researcher session (JWT in localStorage / Bearer header) - what the user has
  2. PAT via `X-AUTH-TOKEN` header - role-gated to BU Owner/Manager/Program Manager (user cannot generate)
  3. OAuth2 API app at `apps.yeswehack.com/oauth/v2/{authorize,token}` - needs CSM approval, read-only

## Top Workflows (researcher side)
1. Discover and qualify programs. Filter active programs by reward tier, scope type (web, mobile, api, iot), language, business unit, and freshness. Today this happens through web search and the platform's program list.
2. Understand scope deeply. Pull every in-scope and out-of-scope asset for a program, see what changed since last review, see which assets are shared across programs the user is invited to.
3. Draft and submit reports. Currently web-only. Researchers want to draft offline with proper templates (CVSS, repro steps, impact, recommendation), then submit when ready.
4. Track open reports. See status across all programs, see which reports are waiting for researcher response vs program triage, see payout status.
5. Learn from hacktivity. Read recently disclosed reports to understand what works on which kinds of programs, e.g., "top XSS findings in the last 90 days on fintech programs."

## Table Stakes (what every competing tool already has)
- List programs (public + private the user has access to). yeswehack-mcp, ywh_program_selector, YesWeBurp, YesWeCaido.
- Filter by scope, reward, type. ywh_program_selector --show, --find-by-scope, --extract-scopes.
- Get program details (scope, rewards, status, languages). All tools.
- Get current user profile, business units, role. yeswehack-mcp.
- List reports with status filter. yeswehack-mcp list_reports.
- Get report details (severity, CVSS, bounty). yeswehack-mcp get_report.
- Get report comments / discussion. yeswehack-mcp list_report_comments.
- List email aliases (program-specific outbound addresses). yeswehack-mcp list_email_aliases.
- Get and request program credentials (test account pools). yeswehack-mcp.
- Browse hacktivity feed (public disclosed reports). yeswehack-mcp get_hacktivity.
- Auth via browser, email/password+TOTP, or token paste. YesWeBurp, yeswehack-mcp, ywh_program_selector all support this.
- Caching of programs for offline use. ywh_program_selector has a cache; YesWeBurp has session-based caching.
- Generic API escape hatch (raw GET against any endpoint). yeswehack-mcp yeswehack_api_get.

## Data Layer
- Primary entities: programs, scopes (rows per asset, parent program), reports, comments, hacktivity_items, business_units, email_aliases, credential_pools, credentials, user_profile.
- Sync cursor: per-resource `updated_at` cursor for programs and reports (the API exposes pagination by date for both).
- FTS5 indexes: programs (title, description, scope text), reports (title, description, comment thread), hacktivity (title, description, category, tags).
- Drift snapshots: scope tables get a periodic snapshot table so the CLI can answer "what changed in this program's scope since last week."

## Codebase Intelligence
- Source: DeepWiki not consulted (skipped: time-bounded research stage, returns are diminishing once the four main competitor READMEs have been read).
- Auth pattern (researcher path): `Authorization: Bearer <jwt>` for api.yeswehack.com requests. JWT is in browser localStorage as `access_token`. Same pattern across YesWeBurp, YesWeCaido, ywh_program_selector, and yeswehack-mcp. JWT has a short TTL and is refreshed via `refresh_token` against the same `apps.yeswehack.com/oauth/v2/token` endpoint used by OAuth2 API apps.
- PAT pattern (out of scope for researcher): `X-AUTH-TOKEN: <pat>` header. Documented endpoints include `GET /reports/{id}`, `GET /programs/{id}/reports?status=new`, `POST /reports/{id}/tracker-message`, `POST /asm/assets`.
- Data model: programs have nested scopes (each scope row carries type, asset, in/out of scope, severity allowed). Reports belong to a program, carry a state machine (new -> ask-for-integration -> accepted -> resolved -> closed). Comments are per-report with public/private flag.
- Rate limiting: not publicly documented. Treat as adaptive (back off on 429, exponential).

## User Vision
- Captured from user: "i am logged in. i want my agent to be able to pick up challenges https://yeswehack.com/programs?disabled=0 to get qualified and submit bug bounties and understand all the apis etc. to be GREAT at the program. overall goal is to submit bugs / security stuff."
- This is an agent-driven researcher workflow. The CLI exists so a Claude/Codex agent can: discover suitable programs, understand them deeply, draft reports with platform-correct shape, submit, track, and learn. No program-manager or BU-side features.
- Quality-first ethos: YesWeHack just rolled out a Platform Code of Conduct calling out "program spamming and AI slop." This CLI must help researchers submit BETTER reports, not MORE reports. Submit commands include guard-rails (CVSS sanity check, dedupe against own prior reports, scope-membership confirmation). No bulk-submit or template-flood capabilities.

## Product Thesis
- Slug: `yeswehack`
- CLI binary: `yeswehack-pp-cli`
- Display name: `YesWeHack`
- Why it should exist: Every YesWeHack tool today is one of: a Burp extension, a Caido extension, a Python MCP, or a narrow filter CLI. None of them is an offline SQLite-backed researcher cockpit with agent-native flags. Researchers want to point a Claude agent at the platform and have it triage programs, draft reports, and track responses without re-implementing the bug-bounty workflow on top of `curl` and a browser tab.
- Headline (for narrative.headline): "Every YesWeHack researcher feature, plus an offline SQLite-backed cockpit for scope cartography, drift detection, draft reports, and hacktivity learning that no Burp/Caido extension can match."
- Anti-spam stance: explicit in README and in command-level guard-rails. The CLI is a quality multiplier.

## Build Priorities
1. **Auth + sync foundation.** `auth login --chrome` extracts the JWT from the user's Chromium profile (same pattern used by other PP CLIs with browser-clearance), refreshes via the OAuth2 token endpoint when the JWT is near expiry, stores in `~/.config/yeswehack-pp-cli/auth.json`. `sync` populates the local SQLite store: programs, scopes, reports, comments, hacktivity, email aliases, credentials.
2. **Absorbed read surface.** Every command in yeswehack-mcp's catalog (programs/reports/comments/hacktivity/credentials/email_aliases/current_user/api_get), every command in ywh_program_selector (program scoring, scope extraction, scope search, collaborations), plus YesWeBurp's scope-into-tool flow (export scope to Burp/Caido/proxychains config).
3. **Transcendence.** Scope cartography (which assets span multiple programs the user is invited to), scope drift detection (what changed in this program's scope since last sync), report dedupe (does this title already exist in user's submitted reports), hacktivity-based learning (top categories per program in last N days), CVSS sanity prediction (rule-based, not LLM), per-program calendar (renewal dates, payout pending), draft-report local workflow (`report draft <program> --title --severity --scope --steps --impact --recommendation`).
4. **Polish + agent surface.** Every Cobra command exposed as MCP tool with `mcp:read-only` annotations. Submit/draft commands explicitly NOT marked read-only. SKILL.md trigger phrases tuned for researcher workflows.
