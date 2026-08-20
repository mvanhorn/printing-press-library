# Slack CLI Brief

_Reprint run `20260801-150754-271fdf29`. Research mode: **REDO** (prior brief 2026-04-09 was 114 days old, lacked a `narrative` block, and recorded all competitor stars as 0)._

## API Identity
- **Domain:** Team communication and collaboration.
- **Users:** Developers, ops/on-call engineers, workspace admins, and terminal-resident power users. Increasingly: AI agents acting on a user's behalf.
- **Data profile:** Messages (high volume, threaded, timestamp-keyed), channels, users, files, reactions, reminders, usergroups, pins, stars. Rich metadata — `thread_ts`, permalinks, reaction counts, cursor pagination on nearly every list method.
- **Base URL:** `https://slack.com/api` — flat method namespace (`conversations.history`, `chat.postMessage`), not REST-shaped paths.

## Reachability Risk

**Low for the target user, but with a real and newly-discovered structural caveat the April research missed.**

**1. Rate-limit tier change on the two most important read methods.** `conversations.history` and `conversations.replies` moved from **Tier 3 to Tier 1** for apps distributed outside the Slack Marketplace: **15 messages per request, 1 request per minute**. Announced 2025-05-29 for new apps; existing non-Marketplace installs became subject on **2026-03-03** — already in force as of this run.

**The exemption is what makes this CLI viable.** Slack's 2025-06-03 clarification, verbatim from the primary source:

> "Any internal customer-built apps will maintain their existing rate limits and will not be subject to the new posted limits."

A user who creates their own app in their own workspace — the exact install shape this CLI assumes — is an internal customer-built app and **keeps Tier 3**. The punitive limit only bites redistributed non-Marketplace apps. This must be stated plainly in the README and SKILL: the CLI's history-dependent features are fine for their intended use and would be crippled under a redistributed token.

**2. The official OpenAPI spec is abandoned.** `slackapi/slack-api-specs` is **archived**, last pushed **2021-09-07**. Its `slack_web_openapi_v2.json` is OpenAPI 2.0 and ~5 years stale — it will both omit methods added since 2021 and retain since-deprecated ones. It is not a trustworthy primary source in 2026.

**3. No bot-detection or access-blocking.** Slack's Web API is a documented, stable, token-authenticated JSON API. No 403/challenge risk. Community SDK issues are about *rate-limit handling ergonomics* (python-slack-sdk #287, #436, #960, #1693 — a persistent complaint since 2018), not reachability.

## Reachability Gate (Phase 1.9)
- **Decision: PASS.** `auth.test` returns `ok: true` against workspace `the test workspace` (`T01234567`) with a bot token.
- Live probe of 16 methods from the pruned official spec found **zero deprecations** — the 2021 spec's method names are all still live in 2026.

### Token-type split (discovered by live probe, not documented in the spec)
Three families return `not_allowed_token_type` for a bot token and require a **user token (`xoxp-`)**:

| Family | Bot token (`xoxb-`) | User token (`xoxp-`) |
|---|---|---|
| `search.*` | ✗ `not_allowed_token_type` | ✓ (needs `search:read`) |
| `stars.*` | ✗ `not_allowed_token_type` | ✓ |
| `reminders.*` | ✗ `not_allowed_token_type` | ✓ |
| everything else probed | ✓ | ✓ |

This is a hard capability boundary, not a scope misconfiguration — adding bot scopes cannot fix it. The SKILL must state it explicitly so agents don't burn calls rediscovering it, and `doctor` should report which token types are configured and therefore which command families are reachable.

## Top Workflows
1. **Search across history the Slack UI won't show you** — free-plan workspaces hide messages older than 90 days from search entirely; a local archive outlives that.
2. **Read channel and thread history** without opening the app — triage, catch-up, on-call review.
3. **Send messages / thread replies** to channels and DMs from scripts, CI, and agents.
4. **Digest and catch-up** — what happened while I was away, what mentions me, what's unanswered.
5. **Workspace ops** — channel inventory, user lookup, usergroup membership, quiet-channel cleanup.

## Table Stakes
Drawn from the competitor sweep below. Any credible Slack CLI must have: send message (channel/DM/thread), read channel history, read thread replies, search messages, list/info channels, list/info users, file upload, add/remove reactions, status and presence, DND/snooze, reminders, usergroup management, emoji list, pins, team info, structured output (`--json`), pipe-friendliness, and multi-workspace support.

## Competitive Landscape

| Tool | Shape | Strength | Gap we exploit |
|---|---|---|---|
| **Official Slack MCP server** (`docs.slack.dev/ai/slack-mcp-server`) | Hosted MCP | Search, send, canvases, profiles; admin-governed | Requires admin approval; no local store; no CLI |
| **korotovsky/slack-mcp-server** (~1.7k★, TS) | MCP | Most capable community MCP: stealth `xoxc`/`xoxd` mode, DMs, group DMs, smart history, caching, proxy | MCP-only, no CLI surface, no SQL/offline analytics |
| **slackapi/slack-skills-plugin** | Claude Code plugin | Official agent integration + Block Kit skills | Developer-app-building focus, not workspace data |
| **slackapi/slack-cli** | Official CLI | App scaffold/deploy | Explicitly *not* a workspace-interaction CLI — different niche |
| **rockymadden/slack-cli** (Bash) | CLI | Rich messaging, uploads, pipe-friendly | Bash, no data layer, no offline search |
| **shaharia-lab/slackcli** (TS/Bun) | CLI | AI-friendly JSON/table/text output, canvas | No local persistence or analytics |
| **tumf/slack-rs** (Rust) | CLI | OAuth, secure token storage, agentic design | No local store |
| **piekstra/slack-chat-api**, **lox/slack-cli**, **candrholdings**, npm one-shot senders | CLI | Narrow send/read | No breadth, no persistence |
| **python-slack-sdk** | SDK | Official, complete | Library not CLI; long-standing rate-limit-handling complaints |

## Concrete User Pain Points
1. **90-day history wall.** Free-plan messages older than 90 days vanish from search and the UI. Users lose their own history.
2. **Export is admin-only, unscoped, and unusable raw.** Free/Pro export covers public channels only; scoping to specific channels or users is Enterprise Grid only. Output is raw JSON "basically unreadable without specialized tools."
3. **Discovery API is gated.** Enterprise Grid plus a review Slack can deny.
4. **Rate-limit handling is a footgun.** Six-plus years of SDK issues about `ratelimited` responses being swallowed rather than surfaced — the same class of bug the prior printed CLI was patched for.

## Data Layer
- **Primary entities:** messages, channels, users, threads, reactions, files, usergroups, reminders, pins, emoji.
- **Sync cursor:** cursor-based (`cursor`, **not** `after` — see prior patches), per-channel `latest_ts` for incremental sync.
- **FTS/search:** FTS5 over message text, channel names, user display names.
- **Why it compounds:** a local mirror is the only way to (a) outlive the 90-day wall, (b) run cross-entity analytics Slack exposes nowhere, and (c) query without re-paying rate-limit cost.

## Prior-Print Reconciliation
The April print shipped **8 of 12** planned transcendence features: `health`, `response-times`, `digest`, `threads-stale`, `trends`, `quiet`, `activity`, `funny`. Dropped: `network` (who-talks-to-whom), `daemon`, `notifications`, `archive-search`. Notably `archive-search` scored 9/10 and maps directly to pain point #1 — worth re-examining.

## User Vision
The user chose reprint over install-as-is or a fresh build, driven by the machine delta (v1.2.1 → v4.29.0) with the MCP surface as the named concrete gap. Their stated interest at the enrichment gate was **reducing the agent-facing tool surface**. No freeform feature vision was supplied — this section is not a feature mandate.

**Correction to prior research:** the April brief recorded "User owns their company's Slack workspace (admin access)." That is **no longer accurate** — the user stated this session they cannot install an app in the company workspace they want to use it with. Live validation for this run happens against a personal test workspace. Do not carry the admin-access assumption forward into auth narrative or onboarding copy.

## Product Thesis
- **Name:** `slack-pp-cli`
- **Why it should exist:** The official CLI builds apps, not workspaces. The MCP servers — including Slack's own — can read and send but keep nothing, so every question re-pays rate-limit cost and nothing survives the 90-day wall. The community CLIs are thin wrappers with no data layer. Nobody combines full Web API breadth, a local SQLite mirror with FTS5, cross-entity analytics, and a thin agent-native MCP surface in one binary. The archive *is* the product: it turns Slack's own retention limit into a solved problem.

## Build Priorities
1. Data layer + sync with correct cursor pagination and honest `ok:false` error surfacing (the prior patch class).
2. Full absorbed feature set from the competitor sweep.
3. Transcendence features that only the local store makes possible — re-examine the four dropped rows, especially `archive-search`.
4. Thin MCP surface: Cloudflare pattern (`slack_search` + `slack_execute`) plus named multi-step intents, per the accepted reprint enrichment.
5. Rate-limit posture: surface `RateLimitError` distinctly from empty results everywhere; document the internal-app exemption.
