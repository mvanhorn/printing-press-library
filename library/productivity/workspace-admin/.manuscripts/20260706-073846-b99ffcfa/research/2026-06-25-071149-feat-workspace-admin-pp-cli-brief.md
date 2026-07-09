# Google Workspace Admin CLI Brief

## API Identity
- **Domain:** Google Workspace administration, auditing, and security. A GAT-Labs-style
  admin/audit tool built directly on Google's own APIs.
- **Surface (combo, confirmed priority Admin → Drive → Gmail):**
  - **admin** = Admin SDK Directory v1 (122 ep), Reports v1 (audit/usage), Alert Center v1beta1 (security alerts)
  - **drive** = Drive v3 (files, permissions, shared drives) (~48 ep)
  - **gmail** = Gmail v1 (messages, settings, delegation, filters) (~79 ep)
  - All five specs are official Google OpenAPI conversions from apis.guru. Hosts differ
    (admin.googleapis.com, alertcenter.googleapis.com, www.googleapis.com/drive/v3,
    gmail.googleapis.com); the generator's multi-spec merge preserves per-endpoint base URLs
    (verified by dry-run: 3-spec merge → 249 endpoints, host-aware).
- **Users:** Google Workspace super admins, IT admins, delegated auditors, security/compliance teams,
  and education IT (the requesting org is example.org). Agents driving admin workflows.
- **Data profile:** users, groups+members, org units, roles/privileges, ChromeOS+mobile devices,
  third-party OAuth apps/tokens, Drive files+permissions+shared drives, Gmail messages+settings+delegation,
  audit activity logs, usage reports, security alerts, calendars. High-cardinality, joinable across APIs.

## Reachability Risk
- **None.** Official Google APIs, globally reachable. Unauthenticated requests return 401
  (expected → reachability PASS). No bot protection.

## Product Thesis
- **Name (slug):** `workspace-admin` (binary `workspace-admin-pp-cli`). Display: "Google Workspace Admin".
- **Why it should exist:** GAT Labs charges for a metadata-scan-and-index audit engine over Google
  Workspace. GAM (Python) has the broadest admin coverage but is stateless, terse, and not agent-native.
  The official `gws` Rust CLI (28.5k★, Mar 2026) is agent-native but *discovery-driven and general-purpose* —
  it mirrors every Google endpoint without curating admin workflows or persisting anything. **This CLI is the
  missing middle: admin-curated GAT-style audit areas, an offline SQLite metadata store enabling cross-API
  joins and offline reports that GAM/gws cannot do, full agent-native output, and guided service-account+DWD auth.**

## The GAT architectural insight (drives the whole design)
GAT+ is, by its own docs, a **metadata-scan-and-index engine**: it runs a fresh Workspace metadata scan
~once/day, indexes it, then serves *filtered audits, cross-area reports, and near-real-time alert rules over
the indexed data* — not live per query. This is precisely the Printing Press pattern:
**`sync` → local SQLite → offline `search`/`sql`/report commands.** Every GAT+ "audit area" (Users, Drive,
Email, Groups, Apps, Devices, Logins, Meet…) becomes a synced resource + offline query surface. The
transcendence features are the cross-API joins and audits that only the local store makes possible.

## Top Workflows (power-user, first-class commands)
1. **Offboard a departing user** — suspend → reset password → sign out → deprovision (revoke tokens/ASPs/2SV)
   → transfer Drive ownership → set Gmail delegation/forward to manager → wipe devices → remove from groups
   → move to /Suspended OU → (later) delete. (Directory + Drive + Gmail + DataTransfer)
2. **External / public Drive sharing audit + remediation** — find files shared `anyoneWithLink` or to external
   domains; show external users & domain connection graph; bulk-restrict/expire/remove external access.
3. **Third-party OAuth app risk audit** — list every connected app, scope-risk score (Low/Mod/High),
   users per app, "Full Drive access" apps; bulk revoke/ban tokens.
4. **Security & login audit** — login activity (geo, failures), 2SV status, dormant accounts, post-compromise
   reconstruction of a user's actions; ingest native Alert Center alerts.
5. **User 360° audit** — one command joining a user's Drive footprint, quota, security posture, connected apps,
   devices, email settings, group memberships, recent activity (cross-API join over the store).
6. **Bulk provisioning / lifecycle from CSV** — create/update/suspend/move users in bulk with dry-run+per-row errors.
7. **Audit/usage reporting & exports** — scheduled-style filtered reports to JSON/CSV; Meet usage & cost.

## Table Stakes (must match GAM/gws/GAT — absorbed surface)
- Users CRUD + suspend/restore/deprovision/aliases/photos/signatures/2SV/password reset/move-OU
- Groups + members + group settings (who-can-post/join/moderate); OUs CRUD + move users/devices
- Roles & privileges list/assign/revoke; ChromeOS + mobile device list/move/disable/wipe/approve
- Reports: activity (login/admin/drive/token/mobile/meet) + usage (user/customer)
- Drive: list files, permissions/ACL CRUD, transfer ownership, shared drives, external-sharing query
- Gmail: delegation, filters, forwarding, send-as, vacation, message search/modify, IMAP/POP, labels
- Alert Center: list/get/delete alerts, feedback; Data Transfer (Drive/Calendar on offboarding)

## Data Layer (local SQLite store — the GAT engine)
- **Primary entities (syncable):** users, groups, group_members, orgunits, roles, role_assignments,
  chromeos_devices, mobile_devices, tokens(apps), drive_files, drive_permissions, shared_drives,
  gmail_settings/sendas/delegates/filters, activities(audit), alerts.
- **Sync cursors:** Directory list pageToken; Reports activities startTime/pageToken; Drive changes/files;
  per-user impersonation for Drive/Gmail (DWD).
- **FTS/search:** full-text over users, files, messages metadata, activities. SQL surface for cross-API joins
  (e.g., external-shared files JOIN owner JOIN last-login).

## Auth (decisive design — Pre-Generation Auth Enrichment)
- All specs declare Google OAuth2. The realistic admin auth is **service account JSON + domain-wide delegation
  + impersonated admin subject** (the only path that reaches Directory/Reports/Alert Center), with per-user
  impersonation for Drive/Gmail bytes.
- **Plan:** model the generated client as **bearer_token** (`Authorization: Bearer <access_token>`) with env
  var `GOOGLE_WORKSPACE_TOKEN`, so any access token (from `gcloud auth print-access-token`, ADC, or our own
  flow) works out of the box. Then **hand-author a custom auth flow** (Phase 3, `internal/cli/<api>_auth.go`):
  - `auth service-account --key sa.json --impersonate admin@domain --scopes ...` → mint signed JWT, exchange
    at oauth2.googleapis.com/token for an access token, cache it; `--impersonate <user>` for per-user Drive/Gmail.
  - `auth print-token` / `doctor` shows scope/impersonation status (GAM's `check serviceaccount` equivalent —
    the #1 onboarding pain point to solve).
- This is a committed Phase 3 hand-code item (service-account DWD flow).

## Scope boundary (do NOT promise)
- **GAT Shield's ~50% endpoint-only features are NOT replicable** via Google APIs (live keyword interception,
  screenshots/webcam, on-screen tab actions, real-time browser RegEx DLP, copy/paste audit, geofencing).
  Native parity reaches device lifecycle (Directory), Chrome allow/deny + extension audit (CBCM/Reports),
  historical usage (Reports). The CLI will say so plainly (SKILL anti-triggers + README).
- **Live content DLP** = post-hoc metadata/stored-content scanning, not pre-inbox/pre-upload blocking.

## Source Priority (combo — from source-priority.json)
- Primary: **admin** (Directory + Reports + Alert Center) — official OpenAPI — headline commands.
- Secondary: **drive** — official OpenAPI — sharing audit.
- Tertiary: **gmail** — official OpenAPI — email/offboarding.
- **Economics:** single Google OAuth2/service-account auth across all three; no free/paid split. No inversion risk
  (primary has the richest official spec).

## Build Priorities
1. Generate the merged 5-spec CLI (host-aware), enrich auth → bearer_token + canonical env var, MCP Cloudflare
   pattern (auto, >50 tools), category = `productivity`/`developer-tools` (pick per docs/CATALOG.md).
2. Hand-author the service-account + DWD auth flow (the onboarding differentiator).
3. Build the offline store + sync for the core entities, then the GAT-style transcendence audits.
4. Offboarding workflow + external-sharing audit + app-risk score as flagship novel commands.

## Competitor landscape (absorb sources)
- **GAM7** github.com/GAM-team/GAM — broadest admin coverage (Python, stateless, terse). Breadth reference.
- **gws** github.com/googleworkspace/cli — official Rust, agent-native, discovery-driven (no store/curation). Differentiation target.
- **GAMADV-XTD3** taers232c/GAMADV-XTD3 — advanced features (todrive, CSV filters, asadmin, EML export).
- **taylorwilsdon/google_workspace_mcp** — reference dual-auth (OAuth + SA/DWD) MCP, user-surface only.
- **orvice/google-workspace-mcp** (Go, Admin SDK), **securityfortech/google-admin-mcp** (archived) — admin-MCP precedent.
- **GAT Labs** gatlabs.com — the vision/parity target (GAT+/Flow/Unlock/FlowHR/Graphs).

## Top pain points to beat
- Service-account + DWD setup is the #1 wall → guided `auth service-account` + `doctor` per-scope diagnostics.
- 429 rate limits / quota exhaustion on bulk ops → adaptive limiter, per-item error isolation, resumable sync.
- No dry-run/undo on destructive ops → `--dry-run` everywhere, typed exit codes.
- GAM/XTD3 fork confusion + terse syntax → one curated binary, readable commands, agent-native help.

## User Vision (reprint 2026-07-06)
- Verbatim: "Ok, use the updated skills and now your upgraded power to totally go through the entire process again and improve everything"
- Reprint under printing-press 4.27.1 (prior: 4.25.0). Machine deltas since prior print: sync/store template overhaul (composite reconcile, pagination clamp), MCP tools/template rewrite, publish/PII hardening, catalog removal.
- Accepted spec enrichments: (1) x-mcp intents (user_security_snapshot, group_membership_expand) on top of the auto-applied Cloudflare pattern; (2) cache freshness — BLOCKED: multi-spec merge drops Cache config (retro candidate), noted for the gate.
- Prior patch watch-list: Directory list endpoints require customer/domain; drive-about requires fields (workspace-admin-required-list-params, upstream #3378). Verify whether the 4.27.1 sync template absorbed it.

## Machine re-validation (Phase 0, 4.25.0 -> 4.27.1)
- Reachability: re-probed in Phase 1.9 this run (admin.googleapis.com).
- Scoring: 4.27.1 scorecard re-run against prior tree graded A; new unscored dims (mcp_description_quality, mcp_token_efficiency, path_validity, auth_protocol, live_api_verification) will be scored on the fresh tree.
- MCP surface: Cloudflare pattern still auto-applies at 266 endpoints; intents now added via x-mcp.
- Auth: bearer-token model + hand-authored service-account JWT-bearer flow still the right call (upstream #3379 not yet implemented in 4.27.1).
