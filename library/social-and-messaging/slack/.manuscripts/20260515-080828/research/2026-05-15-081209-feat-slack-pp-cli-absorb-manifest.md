# slack-pp-cli Absorb Manifest

> Phase 1.5b/d output. Synthesizes baseline 12-verb user vision + scoping doc + addendum's 9 absorb additions + novel-features subagent's 8 transcendence survivors.

---

## Absorbed (match or beat everything that exists)

Slack-side endpoints come from the spec (174 endpoints). The table below covers cross-tool feature parity — what other Slack CLIs and MCPs do, and how slack-pp-cli matches/beats each.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|--------------------|-------------|
| 1 | List channels (public + private + DMs) | MCP `slack_list_channels` | `channels list` (spec) + FTS5 channel search + on-disk cache | Cached after first sync; offline; cross-references with attached pp-* mirrors |
| 2 | Channel history paging | MCP `slack_get_channel_history` | `conversations history` (spec) + auto-pagination + `Retry-After` respect + silent-truncation WARNING | Surfaces the May-2025 page-size cap bug (warns when returned < requested) |
| 3 | Thread replies | MCP `slack_get_thread_replies` | `conversations replies` (spec) + offline `thread-summary` verb | Local thread fan-out join; user-name resolved from mirror |
| 4 | Post message | MCP `slack_post_message` | `post` verb wrapping `chat.postMessage` | `--dry-run` for cron safety; centralized auth; `--blocks-json` for Block Kit |
| 5 | Reply to thread | MCP `slack_reply_to_thread` | `post --thread <ts>` (subset of #4) | Single command shape |
| 6 | Add reaction | MCP `slack_add_reaction` | `reactions add` (spec) | Plus `reactions summarize` transcendence verb (T#4) |
| 7 | List users | MCP `slack_get_users` | `users list` (spec) + offline cache | Cached; cross-referenced with HubSpot/Attio team lookup |
| 8 | User profile | MCP `slack_get_user_profile` | `users info` (spec) + offline cache | Cached |
| 9 | Workspace dump to local | rusq/slackdump `dump` + `archive` | `sync` + SQLite mirror (5× faster than slackdump's old chunk-file format) | FTS5 search, offline; agent-native JSON; resumable cursor |
| 10 | Export to Mattermost/Standard ZIP | rusq/slackdump `export` | OUT-OF-SCOPE v1 (stub via `export --format` flag, returns error with v1.1 message) | Defer — no weekly use case in personas |
| 11 | Convert between archive formats | rusq/slackdump `convert` | OUT-OF-SCOPE v1 (stub) | Defer |
| 12 | Local archive viewer | rusq/slackdump `view` | OUT-OF-SCOPE v1 — pipe `--format md/json` to standard tools | Defer (scope creep kill per rubric) |
| 13 | Multi-workspace config | rusq/slackdump `workspace` | `--workspace <team_id>` flag (deferred to v1.1; single-workspace today) | Stub interface |
| 14 | Search messages cross-channel | MCP `slack_search` (most MCPs lack) + `search.messages` API | `who-said` verb (FTS5 over local mirror) + `--api-passthrough` for live `search.messages` | TWO modes: offline mirror FTS5 (fast, no rate limit) and live `search.messages` (latest, user-token-only, unaffected by May-2025 cap) |
| 15 | Usergroup rendering (`<!subteam^S…>`) | korotovsky/slack-mcp-server | `usergroups list` + substitution pass at emit time | **Fixes known bug in weekly-digest skill** — substitutes handles for human-readable names |
| 16 | Unreads via client.counts | korotovsky/slack-mcp-server | `unreads --priority` transcendence verb (T#5, see survivors below) | xoxp branch: mirror last_read scan; xoxc branch: client.counts (v1.1) |
| 17 | Mark-as-read (non-destructive cron) | korotovsky/slack-mcp-server | `conversations mark` (spec) + `digest --mark-read` flag | Lets cron reads not pollute human unread state |
| 18 | Stealth auth (xoxc + xoxd) | korotovsky/slack-mcp-server | OUT-OF-SCOPE v1 (env var detection only) — xoxp + xoxb v1 | Defer to v1.1 |
| 19 | GovSlack base URL | korotovsky/slack-mcp-server | `--base-url` env var (`SLACK_BASE_URL`, defaults to `https://slack.com/api`) | Cheap to support |
| 20 | Block Kit message construction | piekstra/slack-mcp-server | `post --blocks-json <file>` flag | Standard pattern matching pp-notion's block construction |
| 21 | Scheduled message | (none in MCPs) | `schedule` verb wrapping `chat.scheduleMessage` | Server-side schedule; survives client restart |
| 22 | Channel/user fuzzy lookup | korotovsky `#name` resolution | `channel-find <fuzzy>` + `user-find <fuzzy>` | Local FTS5 over mirror; ties hardcoded channel IDs to handles |
| 23 | Files list / info | spec (`files.*` 13 endpoints) | Generator emits all 13 + offline `files list` against mirror | No competitor differentiation needed |
| 24 | Admin operations (admin.*) | spec (56 endpoints) | Generator emits all 56; documented as "unreachable on non-admin tokens; will return `not_authed`" | Spec coverage for completeness; no novel verbs |
| 25 | Reactions list per message | spec (`reactions.get`) | `reactions get` (spec) + `reactions summarize` transcendence (T#4) | Local GROUP BY emoji |
| 26 | Conversations open (start DM) | spec (`conversations.open`) | `conversations open` (spec) | Standard generator output |
| 27 | Conversations info | spec | `conversations info` (spec) + cached | Standard |
| 28 | Pinned items | spec (`pins.*`) | Generator emits | Standard |
| 29 | Users.lookupByEmail | spec | Generator emits | Useful for HubSpot/Attio cross-ref |
| 30 | Cron-friendly write commands (`--dry-run`) | (none — pp-asana / pp-notion pattern) | All write verbs (`post`, `schedule`, `reactions add`, `conversations mark`) support `--dry-run` | Replaces 4 hardcoded-xoxp-token cron scripts |
| 31 | Audit log of DM reads | (none) | Every read against an IM/MPIM channel writes `audit.log` with timestamp + caller + verb + channel | Privacy guard; required by brief; backs T#8 `agent-audit` |
| 32 | Channel allow/deny list | (none — privacy posture) | `~/.config/slack-pp-cli/skip.yaml` honored by `sync` + `digest` + `customer-intel` | Mirrors weekly-digest skill's sensitivity rules |
| 33 | Sensitivity redaction | weekly-digest skill `Sensitivity Rules` | `--redact-sensitivity` flag strips comp/HR keywords pre-emit | Inherits `feedback_digest_sensitive_content.md` rule |
| 34 | Cross-channel customer search | (none) | `customer-intel` verb (user vision) — see baseline below | Hand-built on mirror |
| 35 | Per-DM TL;DR | (none — weekly-digest does this manually) | `dms-summary` verb (user vision) | Cron-friendly |
| 36 | Drift detection | pp-asana inspiration | `drift` verb (user vision) — Slack-attention version | Composable |
| 37 | Dormant channels | (none) | `dormant` verb (user vision) | Cleanup signal |
| 38 | Attention triage | (none) | `attention` verb (composed: drift + dms-summary + reactions) | Inbox-zero shape |
| 39 | Thread summary | MCP `slack_get_thread_replies` (raw) | `thread-summary` (user vision) — bullet + decisions + action items | User-name resolution from mirror |
| 40 | Multi-channel weekly digest | (none — weekly-digest skill compiles manually) | `digest` verb (user vision) | One call replaces 30+ REST round-trips |

**Total absorbed: 40 features.** 12 are baseline endpoint wrappers from the spec (rows 1-8, 23-29). 12 are user-vision novel verbs (rows 34-40 + baseline 4 verbs). 9 come from the addendum's slackdump + korotovsky findings (rows 9-20). 5 are AtomChat-specific privacy/cron rows (rows 30-33). 2 are spec-derived utilities (rows 26-29).

### Stubs (explicit, must be approved at Phase Gate 1.5)

| # | Feature | Status | Reason |
|---|---------|--------|--------|
| 10, 11, 12 | `export`/`convert`/`view` | (stub — defer to v1.1) | No weekly persona use surfaced; rubric "Scope creep" kill |
| 13 | Multi-workspace config | (stub — accept `--workspace` flag, no-op v1) | Single AtomChat workspace today |
| 18 | xoxc+xoxd stealth auth | (env var detection only, no full implementation v1) | xoxp covers Erick's reads; v1.1 unlock |

---

## Transcendence (only possible with our approach) — Novel-features subagent survivors

| # | Feature | Command | Score | How It Works | Evidence | Persona served |
|---|---------|---------|-------|--------------|----------|----------------|
| T1 | Customer Intel Deep | `customer-intel-deep "Sonria" --window 14d` | **9/10** | SQLite ATTACH DATABASE across pp-slack/pp-attio/pp-asana/pp-fathom mirrors → unified timeline ordered by ts: Slack mentions + Attio deal stage at that moment + open Asana tasks + Fathom call action items, every line cited with permalink | Brief Top Workflows #1; csm-platform-v2 alert engine is system of record for narratives but doesn't produce cross-source timelines | Erick-as-CSM-Coach (P2) |
| T2 | DM Engagement Heatmap | `dm-engagement --report all --window 14d` | **8/10** | Per-direct-report row: DM count + Asana tasks completed/created + Fathom calls attended. Pure SQL volumes, no LLM | Brief Top Workflows #4 (pre-1:1/scorecard DM scan); replaces vault-cached scorecard skill's stale data | Erick-as-Leader (P1) |
| T3 | Action Follow-through | `action-followthrough --report marjorie --window 14d` | **8/10** | pp-fathom action_items × pp-slack FTS5 joined on assignee + time window: did the CSM mention the company in Slack within 14 days of the action item? Binary follow-up + permalink | Brief Top Workflows #1 (L10 prep); csm-platform-v2 doesn't audit follow-through | Erick-as-CSM-Coach (P2) |
| T4 | Reactions Summarize | `reactions summarize --channel "#the-wolf-of-atom" --window 7d` | **7/10** | Local GROUP BY emoji_name on reactions JOIN messages over window. Top messages by reactions, emoji distribution, sentiment-bucket via emoji class (🎉/👍/😢/🔥) | Brief Top Workflows #2 (daily attention); addendum line 67 reactions as approval workflow | Erick-as-Leader (P1) |
| T5 | Unreads Priority | `unreads --priority` | **7/10** | xoxp branch reads sync_state.last_read vs messages.ts from mirror. Two-mode auth: xoxc → client.counts (v1.1), xoxp → mirror scan. DMs > partner > internal | Addendum line 104; korotovsky prior art | Erick-as-Leader (P1, daily) |
| T6 | Usergroups Render | `usergroups list` + emit-time substitution | **7/10** | Local usergroups table + regex substitution `<!subteam^S…>` → `@csm-emea` at digest emit time | Brief Data Layer line 38 + addendum line 109: known bug today | Erick-as-Leader (P1, digest) + Cron (P3) |
| T7 | Goal Channel Pulse | `goal-channel-pulse --quarter current` | **6/10** | pp-asana goals × pp-slack messages joined via `rocks.yml` channel mapping. Per active Rock: 7d discussion volume + stalled flag | Brief Top Workflows #3 (weekly digest); pp-asana has `drift` (task-level) but not conversation-level | Erick-as-Leader (P1, L10 prep) |
| T8 | Agent Audit | `agent-audit --window 7d` | **6/10** | SELECT from audit_log JOIN channels: which pp-*/cron agents read which DMs last Nd. Actionable view over the brief-mandated audit log | Brief Privacy Posture; backs `feedback_digest_sensitive_content.md` policy | Cron (P3) + Erick governance |

**Total transcendence: 8 features, all ≥6/10. Three are cross-source ATTACH-DATABASE joins (T1, T2, T3) that no MCP can build because they require co-located local databases — the unique unlock of having all 4 pp-* mirrors on the same machine. One (T6) is a bug-fix-required feature regardless of score (digest skill mis-renders subteam mentions today).**

### Killed candidates (from subagent — surfaced for user override at gate)

| Feature | Kill reason | Closest sibling |
|---|---|---|
| `export` Slack-Standard/Mattermost zip | No weekly persona use; speculative compliance | (deferred to v1.1, stub at row 10) |
| `mark` standalone verb | Thin wrapper kill — folded to flag | `digest --mark-read` flag (row 17) |
| `files` search/list as transcendence | No weekly persona use surfaced | T1 customer-intel-deep covers customer file mentions |
| `emoji` download | Out-of-scope per addendum line 153 | (none) |
| `view` TUI | Scope creep kill | `--format md/json` pipe-out |
| `socket-mode` listener | Persistent process — rubric kill | Polling + `sync --since` |
| `reaction-impact` (my posts) | Subsumed by `reactions summarize --user me` | T4 Reactions Summarize |
| `huddle-snippets` | Scoping doc §4 excludes calls.* | (none) |
| `canvas` reader | Spec coverage unconfirmed; Erick uses Notion for canvases | pp-notion |
| `alert-trace` (recover signals behind a #csm-signals alert) | csm-platform-v2 narrative-alert-engine is system of record — Reimplementation rubric kill | T1 customer-intel-deep |
