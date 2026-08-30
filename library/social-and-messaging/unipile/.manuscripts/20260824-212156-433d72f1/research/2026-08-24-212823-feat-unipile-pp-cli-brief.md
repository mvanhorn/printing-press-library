# Unipile CLI Brief

## API Identity
- **Domain:** Unified communications API. One HTTP surface over LinkedIn (classic + Sales Navigator + Recruiter), WhatsApp, Telegram, Instagram, Messenger, X/Twitter, Gmail, Outlook, IMAP, Google/Microsoft Calendar.
- **Users:** Founders and GTM/recruiting engineers building outreach, inbox, and CRM-sync automation; agencies running LinkedIn sequences; anyone who wants one API instead of nine scrapers.
- **Data profile:** Cursor-paginated lists across accounts, chats, messages, attendees, emails, folders, drafts, calendars, events, posts, comments, reactions, relations, invitations, jobs, applicants, webhooks. High-gravity entities are long-lived and re-read constantly (chats, attendees, relations, emails).
- **Spec source:** Tenant-served canonical OpenAPI at `https://{DSN}/api-json` — 74 paths / **94 operations**, 8 tags (Accounts, Messaging, Users, Posts, LinkedIn Specific, Emails, Webhooks, Calendars). Cross-checked against all 94 per-endpoint OpenAPI fragments embedded in the public reference `.md` pages (identical coverage).
- **Auth:** `X-API-KEY: <access token>` header (`Access-Token` securityScheme). Second scheme `bearer` exists in the spec but is not the documented path — pin `Access-Token`.
- **Base URL is per-tenant.** Every customer has their own DSN (`https://apiNN.unipile.com:PORT`). This is a first-class config concern, not a constant. Docs also allow `https://apiNN.unipile.com/api/v1/...?port=PORT` when custom ports are firewalled.

## Reachability Risk
- **None.** Live probe with the operator's key: `GET /api/v1/accounts` -> `200`, `GET /api/v1/chats` -> `200`, `GET /api/v1/webhooks` -> `200`. `/api/v2/*` -> `404` on this DSN, so this tenant is **v1**; v2 is a separate product line with its own docs branch (178 ops) and is out of scope.
- Tier/permission hints from 4xx body: none — 4xx responses seen were `errors/invalid_parameters` (missing required `account_id`), which is parameter shape, not entitlement.
- Probe-safe endpoint used: `GET /api/v1/accounts` (read-only, no required params).

## Top Workflows
1. **Triage the unified inbox.** List chats across every connected provider, filter unread, read a thread, reply — without opening LinkedIn/WhatsApp/Gmail.
2. **Run outreach without getting the account flagged.** Search LinkedIn people, send invitations with a note, detect who accepted, follow up in-thread — while staying under LinkedIn's undocumented-but-real daily/weekly caps.
3. **Sync communications into local state for analysis.** Pull chats/messages/relations/emails into something queryable so an agent can answer "who went quiet", "who never replied", "what did we say to this account".
4. **Recruiting loop.** Search jobs, post a job, list applicants, pull resumes, message candidates.
5. **Operate the platform itself.** Connect/reconnect accounts, solve 2FA checkpoints, register webhooks, monitor account status.

## Table Stakes
Absorbed from `Sundeepg98/mcp-server-unipile` (95 MCP tools, archived), `honeybluesky/mcp-unipile` (16 stars), the official `unipile-node-sdk` (44 stars, TypeScript), `unipile/unipile-python`, and Unipile's own n8n/Make/Zapier nodes:
- Full account lifecycle: list/get/connect/delete/reconnect/resync/restart + checkpoint solve/resend.
- Messaging: list chats, get chat, chat messages, all messages, start chat, send message, reactions, forward, delete, mark read/unread, sync chat.
- Attendees: list, get, picture, chats-by-attendee, messages-by-attendee.
- Email: list/get/send/delete/draft/trash/unread, folders, labels, contacts, attachments.
- Calendar: list calendars, list/create/edit/delete events.
- LinkedIn: people/company/post/job search, Sales Navigator search, search parameter discovery, profile get/edit, company profile, profile visitors, followers/following, relations, invitations (send/accept/decline/cancel, sent/received lists), InMail + credit balance, skill endorsement, posts (create/get/comment/react/list comments/list reactions), user posts/comments/reactions, hiring projects, job CRUD + publish + close + applicants + resumes, raw LinkedIn passthrough.
- Webhooks: list/create/delete.
- Provider-default account IDs so callers don't repeat `account_id` on every call (the competing MCP does this via `UNIPILE_LINKEDIN_ACCOUNT_ID` / `UNIPILE_EMAIL_ACCOUNT_ID`).

## Data Layer
- **Primary entities:** accounts, chats, messages, chat_attendees, emails, folders, calendars, events, posts, comments, reactions, relations, invitations (sent/received), followers/following, jobs, applicants, webhooks.
- **Sync cursor:** every list route is cursor-paginated (`cursor` + `limit` <= 250; `cursor: null` means done). Sync stores the last cursor per resource per account.
- **FTS/search:** message bodies, email subject/body, attendee and relation names, post text. Offline full-text search across *every provider at once* is something no existing tool offers — the API has no cross-provider search route.
- **Extra local table (not in the API):** an action ledger recording every rate-sensitive write (invitation, profile visit, search result page, InMail) with account_id + timestamp, so the CLI can compute remaining daily/weekly budget.

## Codebase Intelligence
- `unipile-node-sdk` open issues expose the real friction: `UnsuccessfulRequestError` does not surface `body.{status,type}`, cursor types break (`TypeSystemDuplicateTypeKind on EncodedQueryCursor`), `only_unreads` was ignored on `getAllChats`, `linkedin_sections` serializes wrong. Translation: **error transparency and pagination correctness are where incumbents leak.**
- Error envelope is structured: `{"status":400,"type":"errors/invalid_parameters","title":...,"detail":...}` — the CLI should map `type` to typed exit codes and print `detail` verbatim rather than swallowing it.
- The archived 95-tool MCP is the feature ceiling to beat; it is a thin per-endpoint mirror with zero local state.

## Pain Points (concrete)
1. **No CLI exists.** Today the only ways in are: write Node/Python against an SDK, run an MCP server inside an AI client, or hand-roll `curl` with a DSN and header. Nothing is shell/pipe/cron-native.
2. **LinkedIn caps are on you.** Unipile explicitly does not enforce limits ("we don't enforce any limits on our side"). Exceeding them returns 422/429/500 and can get a real account restricted. No tool tracks spend against those budgets.
3. **`account_id` is required almost everywhere** and is an opaque 22-char blob. Every existing tool makes you carry it by hand.
4. **Cross-provider questions are unanswerable in one call.** "Show me everything from this person across LinkedIn and email" requires N calls and manual joins.

## Product Thesis
- **Name:** `unipile-pp-cli`
- **Why it should exist:** Unipile unified nine providers into one API but stopped at the API. There is no shell-native way to use it, no local mirror to query, and nothing standing between an outreach script and a LinkedIn account restriction. This CLI is the missing operator layer: every one of the 94 endpoints as a typed command, a local SQLite mirror that makes cross-provider search and cohort queries possible offline, and a rate-budget ledger that knows how many invitations you have left today before LinkedIn tells you the hard way.

## Build Priorities
1. Data layer + `sync` for accounts, chats, messages, attendees, emails, relations, invitations — the substrate every novel feature needs.
2. Full 94-endpoint absorbed surface with `--json`, `--select`, `--dry-run`, typed exit codes, cursor auto-pagination, and structured error passthrough (`type`/`detail`).
3. Account-alias resolution so `--account linkedin` works instead of a 22-char id; `UNIPILE_BASE_URL` / DSN config with the `?port=` fallback.
4. Transcendence tier: rate-budget ledger, cross-provider unified search, contact 360, silence/no-reply detection, invitation funnel.
