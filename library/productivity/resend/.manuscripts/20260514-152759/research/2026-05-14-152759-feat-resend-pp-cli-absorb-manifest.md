# Resend CLI Absorb Manifest

## Sources Surveyed
- **Official CLI** `resend/resend-cli` (368★, v2.2.1, TypeScript) — 53 commands across 13 resources
- **Official MCP** `resend/mcp-send-email` (510★, v2.6.0) — ~49 MCP tools
- **Official SDK** `resend/resend-node` (907★, v6.12.3) — services across emails, batch, domains, audiences, contacts, broadcasts, webhooks, segments, topics, contact-properties, templates, automations, events, logs
- **Official SDK** `resend/resend-go` (v3) — services Emails, Batch, ApiKeys, Domains, Audiences, Contacts, Broadcasts, Receiving, Webhooks
- **Resend's own Claude Code skill** shipped at `resend/resend-cli/skills/resend-cli/SKILL.md`
- **Spec:** `resend/resend-openapi/main/resend.yaml` — 100 ops / 49 paths / 14 resources

## Absorbed (match or beat everything that exists)

The 100-op spec generates the bulk of these automatically. The table below is curated to show coverage parity with the official CLI/MCP/SDKs; the full enumeration is the generated command tree.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Send transactional email | resend-node, resend-cli `emails send` | `emails send` (generated from POST /emails) | --json, --dry-run, --select, MCP tool, local store of sent emails |
| 2 | Batch send (≤100) | resend-cli `emails batch send` | `emails batch send` (POST /emails/batch) | --dry-run, agent-native |
| 3 | Send scheduled | resend-cli `emails send --scheduled-at` | `emails send --scheduled-at` | Natural-language parsing |
| 4 | Cancel scheduled email | resend-cli `emails cancel` | `emails cancel <id>` | --idempotent, typed exit codes |
| 5 | Update scheduled email | resend-cli `emails update` | `emails update <id>` (PATCH /emails/{id}) | --json |
| 6 | Get email + delivery state | resend-cli `emails get` | `emails get <id>` | --select for compact agent output |
| 7 | List emails | resend-cli `emails list` | `emails list` | --json, --select, --limit, store-backed `--data-source local/auto/live` |
| 8 | Received email inspection | resend-cli `emails receiving get` | `emails receiving get <id>` | Attachment download with hash |
| 9 | Attachment download | resend-node attachments | `emails receiving attachments get <id>` (GET attachments) | Local cache, dedupe by hash |
| 10 | Domain create | resend-cli `domains create` | `domains create <name>` (POST /domains) | --region, --dry-run |
| 11 | Domain verify | resend-cli `domains verify` | `domains verify <id>` (POST /domains/{id}/verify) | --idempotent |
| 12 | Domain status (DKIM/SPF/DMARC) | resend-cli `domains get` | `domains get <id>` | --select records summary |
| 13 | Domain update | resend-cli `domains update` | `domains update <id>` (PATCH /domains/{id}) | --json |
| 14 | Domain delete | resend-cli `domains remove` | `domains delete <id>` (DELETE /domains/{id}) | --ignore-missing |
| 15 | Domain list | resend-cli `domains list` | `domains list` | Store-backed |
| 16 | API key create | resend-cli `api-keys create` | `api-keys create <name>` (POST /api-keys) | --permission flag |
| 17 | API key delete | resend-cli `api-keys remove` | `api-keys delete <id>` | --ignore-missing |
| 18 | API key list | resend-cli `api-keys list` | `api-keys list` | Store-backed |
| 19 | Audience create | resend-cli `audiences create` | `audiences create <name>` | --json |
| 20 | Audience list | resend-cli `audiences list` | `audiences list` | Store-backed, FTS |
| 21 | Audience get | resend-cli `audiences get` | `audiences get <id>` | --select |
| 22 | Audience delete | resend-cli `audiences remove` | `audiences delete <id>` | --ignore-missing |
| 23 | Contact create | resend-cli `contacts create` | `contacts create --audience <id>` | --first-name, --last-name, --unsubscribed |
| 24 | Contact list per audience | resend-cli `contacts list` | `contacts list --audience <id>` | Store-backed, FTS |
| 25 | Contact get | resend-cli `contacts get` | `contacts get <id> --audience <id>` | --select |
| 26 | Contact update | resend-cli `contacts update` | `contacts update <id> --audience <id>` | --json, PATCH semantics |
| 27 | Contact delete | resend-cli `contacts remove` | `contacts delete <id> --audience <id>` | --ignore-missing |
| 28 | Broadcast create | resend-cli `broadcasts create` | `broadcasts create` (POST /broadcasts) | --html-file, --markdown-file |
| 29 | Broadcast send | resend-cli `broadcasts send` | `broadcasts send <id>` | --dry-run |
| 30 | Broadcast schedule | resend-cli `broadcasts schedule` | `broadcasts schedule <id> --at <when>` | Natural-language |
| 31 | Broadcast list | resend-cli `broadcasts list` | `broadcasts list` | Store-backed, FTS on name/subject |
| 32 | Broadcast get | resend-cli `broadcasts get` | `broadcasts get <id>` | --select |
| 33 | Broadcast update | resend-cli `broadcasts update` | `broadcasts update <id>` | --json |
| 34 | Broadcast delete | resend-cli `broadcasts remove` | `broadcasts delete <id>` | --ignore-missing |
| 35 | Webhooks create | resend-cli `webhooks create` | `webhooks create` | --events filter |
| 36 | Webhooks list | resend-cli `webhooks list` | `webhooks list` | Store-backed |
| 37 | Webhooks get/update/delete | resend-cli `webhooks get/update/remove` | `webhooks get/update/delete` | --idempotent / --ignore-missing |
| 38 | Webhooks listen (local tunnel) | resend-cli `webhooks listen` | `webhooks listen --port <n>` *(stub — requires ngrok-style tunnel, deferred)* | Status: (stub — requires external tunnel infra) |
| 39 | Segments CRUD | resend-cli `segments *` | `segments create/list/get/update/delete` | Store-backed |
| 40 | Topics CRUD | resend-cli `topics *` | `topics create/list/get/update/delete` | Store-backed |
| 41 | Contact-properties CRUD | resend-cli `contact-properties *` | `contact-properties create/list/get/update/delete` | Custom-attr schema introspection |
| 42 | Templates create/list/get | resend-cli `templates *` | `templates create/list/get` | Store-backed, FTS on name |
| 43 | Templates publish/duplicate | resend-cli `templates publish/duplicate` | `templates publish/duplicate` | --idempotent |
| 44 | Automations list/get | resend-mcp automations | `automations list/get` | Store-backed |
| 45 | Automation-runs list | resend-mcp automation-runs | `automation-runs list --automation <id>` | Store-backed |
| 46 | Logs list | resend-mcp logs | `logs list` | Store-backed, --filter |
| 47 | Events list | resend-mcp events | `events list` | Store-backed, --filter event_type |
| 48 | `doctor` (auth + reachability) | resend-cli `doctor` | `doctor` | Generated framework command |
| 49 | `whoami` | resend-cli `whoami` | `auth status` | Framework command |
| 50 | `commands` (CLI tree JSON) | resend-cli `commands` | `agent-context` | Generated framework command |
| 51 | `sql <query>` | (none) | `sql 'SELECT … FROM emails …'` | **Beats every competitor** — SQL over local store |
| 52 | `search <text>` | (none) | `search 'invoice'` | FTS5 across all resources |
| 53 | `--profile` multi-workspace | resend-cli `--profile` | `--profile <name>` | Framework: store separated per profile |

**Total absorbed:** 53 features matching the official CLI's 53-command surface + the SDK's full method set. Item 38 (webhooks listen) ships as a stub with honest messaging because it requires external tunnel infrastructure (ngrok-style) outside the generator's scope.

## Transcendence (only possible with our approach)

Eight novel cross-resource commands that no Resend tool (CLI, MCP, SDK, or dashboard) offers — every one of them is enabled by the local SQLite store.

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | Recipient email timeline | `emails to <recipient>` | Resend has no "emails by recipient" endpoint; requires local index on `to` across all sent emails | 9/10 |
| 2 | Cross-event delivery trace | `emails timeline <id>` | API splits delivery state across `/emails/{id}` + `/logs`; this collapses them into one rolled-up view | 8/10 |
| 3 | Audience inventory rollup | `audiences inventory` | Per-audience contact count, unsubscribed rate, last-broadcast — no aggregate endpoint exists | 9/10 |
| 4 | Cross-audience contact lookup | `contacts where <name\|email>` | Requires scanning every audience via N API calls today; one local query | 9/10 |
| 5 | Broadcast performance dashboard | `broadcasts performance` | Open/click/bounce rate across all broadcasts in one table; dashboard caps at 30d and shows one broadcast at a time | 8/10 |
| 6 | Domain health summary | `domains health` | Verification + DKIM/SPF/DMARC status across all domains, flags missing records; no aggregate endpoint | 8/10 |
| 7 | Deliverability summary | `deliverability summary --window 7d` | Direct answer to the "blind spot" complaint — bounce/complaint rate + suppression count over rolling window from local events | 9/10 |
| 8 | API-key rotation audit | `api-keys rotation` | Keys sorted by age + last-used (joined from logs); flags stale keys; no endpoint surfaces last-used at scale | 7/10 |

All 8 score ≥ 5/10 and are recommended for shipping scope.

### Anti-reimplementation discipline
Every novel command above reads from `internal/store` (the local SQLite cache populated by `sync`). No fake endpoint stubs, no canned JSON, no in-process aggregation when the API has an aggregation endpoint. The cross-resource commands exist precisely because Resend has no aggregation endpoints — pure local-query value-add.

## Stubs (explicit)

| Feature | Reason |
|---------|--------|
| `webhooks listen` | Requires external tunnel infrastructure (ngrok or equivalent). Ships as `(stub — requires external tunnel)` with print-only output explaining the setup. |
