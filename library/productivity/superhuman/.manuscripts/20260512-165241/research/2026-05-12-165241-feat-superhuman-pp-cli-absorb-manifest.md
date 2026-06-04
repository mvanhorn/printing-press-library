# Superhuman Absorb Manifest

## Scope summary
- 20 absorbed features (every feature edwinhu/superhuman-cli implements + the official MCP's 4 tools + email-CLI table stakes)
- 12 transcendence features (post-cut, all >= 6/10, evidence-backed)
- 32 total features. Ambitious but grounded — each survivor's evidence and persona is in the brainstorm doc.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|--------------------|-------------|--------|
| 1 | List recent threads | edwinhu/superhuman-cli + official MCP `list_threads` | `POST /v3/userdata.getThreads {filter:{}, offset, limit}` | `--json`, `--select`, `--limit`, `--csv`, agent-native | Shipping |
| 2 | Read thread + messages | edwinhu `read.ts` | `POST /v3/userdata.getThreads {filter:{threadId}}` + local store fallback | `--json`, `--with-body`, agent-native | Shipping |
| 3 | Create draft | edwinhu `draft create` + MCP `create_or_update_draft` | `POST /v3/userdata.writeMessage` (path: `users/<uid>/threads/<tid>/messages/<draftId>/draft`) | `--body-stdin`, `--body-file`, `--dry-run` | Shipping |
| 4 | Update draft | edwinhu `draft update` | Same `userdata.writeMessage` (merge fields) | `--stdin`, `--merge` | Shipping |
| 5 | List drafts | edwinhu `draft list` | local store query (synced from `userdata.getThreads`) | `--json`, `--recent` | Shipping |
| 6 | Delete draft | edwinhu `draft delete` | `userdata.writeMessage` with `discardedAt` | `--dry-run`, `--confirm` | Shipping |
| 7 | Send draft | edwinhu + MCP `send_email` | `POST /messages/send/log` then `POST /messages/send` (with 20s undo via `delay`) | `--undo <sec>`, `--no-undo`, `--at <time>` (Send Later) | Shipping |
| 8 | Reply | edwinhu `reply` | Compose with inReplyTo + send pipeline | `--inline-quote`, `--to-only` | Shipping |
| 9 | Reply-all | edwinhu `reply-all` | Compose with all recipients + send pipeline | `--exclude-cc` | Shipping |
| 10 | Forward | edwinhu `forward` | Compose with forwarding-from + send pipeline | `--strip-attachments`, `--to <email>` | Shipping |
| 11 | Archive thread | edwinhu `archive.ts` | `userdata.writeMessage` labels `removeLabelIds:["INBOX"]` | `--bulk`, `--dry-run` | Shipping |
| 12 | Delete thread | edwinhu `delete.ts` | `userdata.writeMessage` labels `addLabelIds:["TRASH"]` | `--confirm` required | Shipping |
| 13 | Mark read / unread | edwinhu `read-status.ts` | `userdata.writeMessage` labels `+/- UNREAD` | `--bulk` | Shipping |
| 14 | Star / unstar thread | edwinhu `labels.ts` | `userdata.writeMessage` labels `+/- STARRED` | `--bulk` | Shipping |
| 15 | Snooze | edwinhu `snooze.ts` + MCP | `POST /reminders/create` | `--until <date>`, `--duration <human>`, `--bulk` | Shipping |
| 16 | Unsnooze | edwinhu `snooze cancel` | `POST /reminders/cancel` | `--bulk` | Shipping |
| 17 | Labels: list, apply, remove | edwinhu `labels.ts` | local store list + `userdata.writeMessage` label edits | `--bulk`, `--json` | Shipping |
| 18 | Snippets: list, use | edwinhu `snippets.ts` | `userdata.getThreads {type:"snippet"}` + local template engine | `--vars` (see novel #9) | Shipping |
| 19 | Attachments: list, download | edwinhu `attachments.ts` | local store + Gmail/Graph API passthrough (uses user's OAuth) | `--output <path>`, `--dir <path>` | Shipping |
| 20 | Semantic AI search | edwinhu `ai` + MCP "Ask AI" | `POST /v3/ai.askAIProxy` SSE stream | `--json`, `--stream`, `--max-results <N>` | Shipping |
| 21 | Contact search / get | himalaya patterns + Google People passthrough | local store + Gmail/Graph passthrough | `--json` | Shipping |
| 22 | Calendar list/create/update/delete | edwinhu `calendar.ts` + MCP `create_or_update_event` | Google Calendar passthrough via user OAuth | `--json`, `--ics` | Shipping (Gmail path); MS Graph stub for Outlook accounts |
| 23 | Sync (incremental) | edwinhu `sync` | `POST /v3/userdata.sync {startHistoryId}` populates local SQLite | `--full`, `--since <id>` | Shipping |
| 24 | Multi-account first-class | himalaya, edwinhu | per-account token bundles + `--account <email>` everywhere | `--account` global flag | Shipping |
| 25 | Doctor | every email CLI | JWT freshness, RPC ping, store integrity, OAuth refresh check | `--json`, `--fix` | Shipping |
| 26 | Local FTS5 search | mu, aerc | SQLite FTS5 over `messages_fts(subject, from, to, body)` | `--json`, `--limit` | Shipping |
| 27 | Auth: status / logout | every CLI | inspect tokens.json, revoke locally | `--json` | Shipping |
| 28 | `sql` (local store query) | generator standard | SELECT-only query over local SQLite | `--json` | Generator-built |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| N1 | Chrome-CDP durable login | `auth login --chrome` | 10/10 | Native Go manages Firebase JWT refresh lifecycle that the official MCP demonstrably fails to maintain — solves "MCP always logs me out" pain |
| N2 | Unified cross-account inbox | `unified inbox`, `unified search` | 9/10 | Per-account SQLite stores joined in one Go query — Superhuman's loudest missing feature; no native cross-account view exists |
| N3 | Awaiting-reply tracker | `awaiting --older-than 3d --vip` | 9/10 | Local SQL over `messages WHERE from_me=1 GROUP BY thread_id` — no API exposes this, agents and sales leads both need it |
| N4 | Read-status leaderboard | `opens --since 7d --by contact` | 8/10 | Aggregates `read_status_events` time-series by contact — Superhuman shows per-thread only |
| N5 | Reply-latency analytics | `latency --from me --since 30d`, `latency --to me` | 8/10 | SQL aggregation over `messages.received_at` deltas — VP-measured sales metric absent from Superhuman |
| N6 | Pattern-driven bulk triage | `triage --rule rules.yaml --dry-run` | 9/10 | YAML DSL → predicates over local store → fan-out to archive/snooze/label/reply; absorbs auto-snooze; agent-shaped |
| N7 | Snooze-coming-back radar | `returning --within 7d` | 7/10 | Local view over snooze metadata; service-specific content pattern Superhuman doesn't surface as a list |
| N8 | Daily inbox-zero digest | `digest --today --markdown`, `digest --week` | 8/10 | Rolls up arrived/archived/snoozed/drafted/awaiting/returning/opens; mechanical (no LLM); pipe-to-`claude` for prose |
| N9 | Snippets with variables | `snippet use intro --var name=Alice` | 6/10 | Local template engine over `{{var}}` placeholders + autovars (`{{today}}`, `{{from}}`); Superhuman snippets are static |
| N10 | Smart-Send hold-and-release | `send --undo 30s`, `unsend <queue-id>` | 7/10 | Local queue with fire-at timestamp; background goroutine fires `/messages/send`; `unsend` aborts before fire — Smart Send pattern as agent-controllable timer |
| N11 | Reminders inbox | `remind list`, `remind add <thread> in 3d`, `remind clear` | 6/10 | Flat queryable reminders list; Superhuman UI exposes only per-thread |
| N12 | Thread brief (mechanical) | `thread brief <id>` | 6/10 | Aggregates participants/message-count/subject-history/attachments/labels/read-status totals; `--json` for Claude Code piping |

## Compound Use Cases (transcendence + transcendence)
- `unified inbox --json | jq '.[] | select(.unread)' | superhuman triage --rule auto-archive-newsletters.yaml` — cross-account stream piped to rule-driven triage.
- `superhuman awaiting --older-than 5d --json | jq -r '.[].thread_id' | xargs -I{} superhuman remind add {} in 1d "follow up"` — auto-create reminders for stale outbound.
- `superhuman digest --week --markdown | claude "summarize what I should focus on this week"` — mechanical digest + LLM synthesis without the LLM-dependency anti-pattern in our code.
- `superhuman opens --since 7d --by contact --json | jq 'sort_by(.opens) | reverse' | head` — sales-leaderboard pipeline.

## Stubs / Known Limitations
- **Microsoft 365 calendar** (#22): Outlook account calendar CRUD requires MS Graph proxy through Superhuman's backend, which is CDP-only. Phase 1 ships Google Calendar; Outlook calendar lands when CDP transport ships.
- **Split-Inbox category filtering** (Important / Other / VIP / Pinned): `userdata.getThreads {listId}` returns 400 on the HTTP backend. Phase 1 ships generic inbox filters from local store labels. CDP `threadInternal.listAsync` would add native pane support.
- **Message HTML body fetching**: HTTP `userdata.getThreads` returns message metadata + snippet + plain-text body when available. Full HTML body requires CDP `messageInternal.getBodyAsync`. Phase 1 ships text bodies; HTML body adds with CDP transport.

## ToS / Risk Note
Superhuman has no public API and no published developer terms for the internal Backend Portal RPC. Personal-use CLI against the user's own logged-in session is consistent with normal client behavior. **Before publishing this CLI to the public Printing Press library**, the user should review the risk surface (Grammarly may issue cease-and-desist; the only public reference impl has 3 stars and no commercial use). Personal use, agent integration in Claude Code, and team-internal use are normal. Phase 6 will surface this at the publish prompt.

## Build Order (Phase 3)
- P0 data layer: threads, messages, drafts, contacts, snippets, labels, calendar_events, attachments, read_status_events, reminders, send_queue
- P1 absorbed (this manifest §1, ~28 features) — generator handles most; hand-build SSE for AI, send pipeline for #7
- P2 transcendence (N1-N12 above) — hand-built
