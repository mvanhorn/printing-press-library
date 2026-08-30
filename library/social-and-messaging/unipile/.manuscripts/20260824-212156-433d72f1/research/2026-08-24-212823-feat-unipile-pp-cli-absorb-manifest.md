# Unipile Absorb Manifest

Sources catalogued: `Sundeepg98/mcp-server-unipile` (95 tools, archived, Python), `honeybluesky/mcp-unipile` (16 stars), `unipile/unipile-node-sdk` (44 stars, official TS), `unipile/unipile-python`, `unipile/unipile-node`, Unipile's own n8n / Make / Zapier nodes, Unipile Hosted Auth wizard. **No competing CLI exists** — this is the first.

## Absorb Manifest

### Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List / get / connect / delete / reconnect / resync / restart accounts | mcp-server-unipile (10 account tools) | (generated endpoint) accounts list/get/create/delete/reconnect/sync/restart | `--json`, `--select`, typed exits, local mirror |
| 2 | 2FA checkpoint solve + resend | mcp-server-unipile `solve_checkpoint`, `resend_checkpoint` | (generated endpoint) accounts checkpoint / checkpoint-resend | `--dry-run` before submitting a code |
| 3 | Hosted auth link generation | Unipile Hosted Auth wizard | (generated endpoint) hosted accounts-link | Scriptable onboarding link minting |
| 4 | List chats / get chat / update chat / delete chat | mcp-server-unipile (14 messaging tools) | (generated endpoint) chats list/get/update/delete | Offline re-read after `sync`, cursor auto-pagination |
| 5 | List chat messages, all messages, get message | unipile-node-sdk `messaging.*` | (generated endpoint) chats messages / messages list / messages get | Fixes the SDK's ignored `only_unreads` by passing filters straight through |
| 6 | Start chat / send message | mcp-server-unipile `start_chat`, `send_message` | (generated endpoint) chats create / chats send-message | `--dry-run` shows the exact request before it sends |
| 7 | Message reactions, forward, delete | mcp-server-unipile | (generated endpoint) messages reaction / messages forward / messages delete | Composable with `jq` |
| 8 | Message + email attachment download | mcp-server-unipile `get_message_attachment` | (generated endpoint) messages attachment / emails attachment | Binary-safe to stdout or file |
| 9 | Sync a chat / sync account messaging | mcp-server-unipile `sync_chat`, `resync_account` | (generated endpoint) chats sync / accounts sync | Poll status without writing a loop |
| 10 | List / get attendees, attendee picture | mcp-server-unipile (6 attendee tools) | (generated endpoint) chat-attendees list/get/picture | Mirrored locally so name resolution is free |
| 11 | Chats by attendee, messages by attendee | mcp-server-unipile `list_chats_by_attendee` | (generated endpoint) chat-attendees chats / chat-attendees messages | Feeds `contact` 360 view |
| 12 | Email list / get / send / delete / update (read, trash) | mcp-server-unipile (11 email tools) | (generated endpoint) emails list/get/create/delete/update | Gmail + Outlook + IMAP through one command |
| 13 | Email drafts | mcp-server-unipile `draft_email` | (generated endpoint) drafts create | `--stdin` body, agent-native |
| 14 | Email folders / labels | mcp-server-unipile `list_email_folders` | (generated endpoint) folders list/get | Cached offline |
| 15 | Email contacts | mcp-server-unipile `list_email_contacts` | (generated endpoint) emails contacts | Joins into the contact index |
| 16 | Calendars list / get | mcp-server-unipile (7 calendar tools) | (generated endpoint) calendars list/get | Google + Microsoft unified |
| 17 | Events list / create / get / edit / delete | mcp-server-unipile `create_event`, `edit_event` | (generated endpoint) calendars events / calendars event | `--dry-run` on every mutation |
| 18 | LinkedIn people / company / post / job search | mcp-server-unipile (6 search tools) | (generated endpoint) linkedin search | Result pages counted against the local budget ledger |
| 19 | LinkedIn search parameter discovery | mcp-server-unipile `get_search_params` | (generated endpoint) linkedin search-parameters | Makes filter values discoverable from the shell |
| 20 | LinkedIn Sales Navigator / Recruiter search | mcp-server-unipile `search_people_sales_nav` | (generated endpoint) linkedin search (api variant) | Same command, `--api` switch |
| 21 | Get profile / company profile / edit own profile | mcp-server-unipile (7 profile tools) | (generated endpoint) users get / linkedin company / users me-edit | Profile fetches counted against the ~100/day visit budget |
| 22 | Profile visitors | mcp-server-unipile `get_profile_visitors` | (generated endpoint) linkedin (visitors action) | Snapshot into local store for week-over-week diffing |
| 23 | Followers / following | mcp-server-unipile `list_followers` | (generated endpoint) users followers / users following | Offline set-difference across syncs |
| 24 | Relations (connections) | mcp-server-unipile `list_relations` | (generated endpoint) users relations | Mirrored; powers acceptance detection |
| 25 | Invitations: send / accept / decline / cancel, sent + received lists | mcp-server-unipile (7 connection tools) | (generated endpoint) users invite / users invite-received / users invite-sent | Every send is written to the budget ledger |
| 26 | InMail send + credit balance | mcp-server-unipile `send_inmail`, `get_inmail_credits` | (generated endpoint) linkedin inmail-balance / messages send (InMail variant) | Balance surfaced in `budget` |
| 27 | Skill endorsement | mcp-server-unipile `endorse_skill` | (generated endpoint) linkedin profile-endorse | — |
| 28 | Posts: create / get / delete, comments, reactions | mcp-server-unipile (6 post tools) | (generated endpoint) posts create/get/comments/reactions/reaction | — |
| 29 | User posts / comments / reactions | mcp-server-unipile `list_user_posts` | (generated endpoint) users posts / users comments / users reactions | Feeds `engagement` |
| 30 | LinkedIn jobs: search, get, create, edit, publish, close | mcp-server-unipile (13 job tools) | (generated endpoint) linkedin jobs / linkedin job / linkedin jobs-publish / linkedin jobs-close | — |
| 31 | Job applicants + resume download | mcp-server-unipile `get_applicant_resume` | (generated endpoint) linkedin jobs-applicants / linkedin jobs-applicant-resume | Resumes straight to disk |
| 32 | Hiring projects (Recruiter) | mcp-server-unipile `get_hiring_projects` | (generated endpoint) linkedin projects | — |
| 33 | LinkedIn contracts + contract select | unipile-node-sdk | (generated endpoint) linkedin contracts / linkedin contracts-select | Needed for Recruiter/Sales Nav seats |
| 34 | Raw LinkedIn passthrough ("magic route") | mcp-server-unipile `raw_linkedin_request`, `perform_linkedin_action` | (generated endpoint) linkedin create | Escape hatch preserved, piped through the same auth/retry |
| 35 | Webhooks list / create / delete | mcp-server-unipile (3 webhook tools) | (generated endpoint) webhooks list/create/delete | — |
| 36 | Default account id so callers stop repeating it | mcp-server-unipile env vars `UNIPILE_LINKEDIN_ACCOUNT_ID` / `UNIPILE_EMAIL_ACCOUNT_ID` | (behavior in unipile-pp-cli accounts alias) Named aliases resolved from the local mirror; `--account linkedin` or `--account "<account name>"` instead of a 22-char id, on every command | Beats env-var-per-provider: aliases are per-account, discoverable, and work for all nine providers |
| 37 | Per-tenant DSN configuration | Every SDK requires hand-passing `BASE_URL` | (behavior in unipile-pp-cli doctor) `UNIPILE_BASE_URL` / `--base-url` config with the documented `?port=` fallback for firewalled custom ports | The `?port=` fallback is documented but no tool implements it |
| 38 | Structured API error surfacing | unipile-node-sdk issue: `UnsuccessfulRequestError` hides `body.{status,type}` | (behavior in unipile-pp-cli doctor) Unipile's `{status,type,title,detail}` envelope printed verbatim and mapped to typed exit codes | The incumbent's #1 open complaint, fixed |
| 39 | Cursor pagination | unipile-node-sdk issue: cursor type breakage | (behavior in unipile-pp-cli sync) `--all` auto-follows `cursor` until null, honouring the `limit<=250` ceiling | No manual cursor loops |

### Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | LinkedIn rate-budget ledger | `budget` | hand-code | Unipile explicitly enforces no limits ("we don't enforce any limits on our side"); LinkedIn does, silently, and punishes with 422/429/500 and account restrictions. Requires a local append-only ledger of every rate-sensitive write per account per day/week joined against the published caps. No API call can answer "how many invitations do I have left today". | Use this before any invitation, profile fetch, or search run to see remaining daily and weekly headroom per account. Do NOT use it to read LinkedIn's own counters; it reports what this CLI has spent. |
| 2 | Offline cross-provider search | `search` | hand-code | The API has no search route that spans providers. Requires every message, email, attendee, and relation in one local FTS index. | Use this to find a phrase across LinkedIn, WhatsApp, Telegram, and email at once, offline. Do NOT use it for LinkedIn people discovery; use 'linkedin search' for that. |
| 3 | Contact 360 | `contact` | hand-code | Joins attendees, relations, invitation state, chats, emails, and posts for one human. Six separate API calls plus a manual join today. | Use this for everything the CLI knows about one person across every provider. Do NOT use it to fetch a live LinkedIn profile; use 'users get'. |
| 4 | Unified unread triage | `inbox` | hand-code | One table of unread across every connected provider. The API returns unread per-provider only, with no cross-provider ordering. | Use this as the daily triage view. Do NOT use it to read a full thread; use 'thread'. |
| 5 | Silence detection | `silent` | hand-code | "I spoke last and got nothing back for N days" requires per-chat last-message-direction plus timestamp math over the local mirror. | Use this to find conversations that died after your message. Do NOT use it for people who never replied to an invitation; use 'funnel'. |
| 6 | Accepted-but-not-messaged | `accepted` | hand-code | Unipile's own docs call this a core need and warn that polling relations looks like automation. Local relation-set diffing across syncs answers it with one API call instead of a poll loop. | Use this to find new connections you have not followed up with yet. Do NOT use it to send the follow-up; pipe into 'chats send-message'. |
| 7 | Invitation funnel | `funnel` | hand-code | sent -> accepted -> replied conversion over time. Requires the local ledger joined against relations and chats; three data sources the API never returns together. | Use this to measure outreach conversion week over week. Do NOT use it for per-person state; use 'contact'. |
| 8 | Cross-provider change digest | `digest` | hand-code | "What happened since my last sync" across nine providers. Requires sync-cursor bookkeeping the API does not keep for you. | Use this after 'sync' to see what changed. Do NOT use it as a live feed; it reports against the local mirror. |
| 9 | Resolved thread view | `thread` | hand-code | Renders a full conversation with attendee ids resolved to names from the local mirror and provider-normalized ordering. | Use this to read one conversation end to end. Do NOT use it to list conversations; use 'inbox' or 'chats list'. |
| 10 | Post engagement crossmatch | `engagement` | hand-code | Cross-references who reacted to or commented on your posts against your relations and invitation history, so you can see which engagers are not yet connections. | Use this to turn post engagement into outreach targets. Do NOT use it for post metrics alone; use 'posts reactions'. |

**Stubs:** none. Every row above is shipping scope.

**Hand-code count:** 10 transcendence commands hand-written after generation. 94 endpoint commands emitted by the generator. Rows 36-39 are behaviors layered onto generated/framework commands.
