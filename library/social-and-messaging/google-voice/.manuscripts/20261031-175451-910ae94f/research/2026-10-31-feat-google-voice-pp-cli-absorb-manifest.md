# Google Voice CLI scope and absorb manifest

Status: approved by Mike, 'Approved and generate', October 31, 2026.
The proposed reader scope and its explicit exclusions are accepted.
Primary chat: codex://threads/01a19430-bdf9-7172-9d90-4527cd171b91

## Decision

Build a standalone, read-only Google Voice message reader and local SQLite mirror.
The proposed shipping scope is 12 core capabilities plus the five local query capabilities below.
This explicitly trims broader community write, realtime, contact-service and Matrix functionality rather than promising full ecosystem parity.
Approval of this manifest accepts those exclusions; there are no proposed stub commands.
Existing Signal/Telegram collectors and the relationship-memory database are separate systems.
Installing a scheduler, changing that database or merging identities is outside this CLI build.

## Verified foundation

Account and conversation-list requests replayed over ordinary HTTP with status 200; removing authentication returned 401 for both.
A read-only thread-detail replay returned 200, matched its requested thread ID and yielded 100 messages plus an older-history cursor.
The list yielded 20 threads.
These are protocol/access proofs, not full-history or all-folder coverage proofs.
Private raw captures and responses remain outside the run/archive tree; proofs contain outcomes and synthetic samples only.
The current authenticated session works without a resident browser or a changed browser-security setting.

## Absorbed and core capabilities

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Private session onboarding | mautrix/gvoice authentication docs; googlevoice-mcp (source) gv_login | google-voice-pp-cli auth import-har | Import only allowlisted Voice read credentials from a private file; verify account before activation; never print cookies or require Chrome at runtime. |
| 2 | Session health and isolation | googlevoice-mcp (source) gv_check_login; libgv auth handling | google-voice-pp-cli auth status | Report local configuration and optional live verification; separate expiry/failure from empty data; pin account identity per profile. |
| 3 | Account read | mautrix/gvoice (source) GetAccount | google-voice-pp-cli account get | Typed safe output through verified ordinary HTTP with current SAPISIDHASH. |
| 4 | Paginated thread inventory and unread filter | googlevoice-mcp (source) gv_list_sms; mautrix/gvoice (source) ListThreads | google-voice-pp-cli threads list | Explicit folder/cursor/page bounds, unread filtering and continuation metadata; no source-state change. |
| 5 | Paginated conversation read and unread guard | googlevoice-mcp (source) gv_read_sms; mautrix/gvoice (source) GetThread | google-voice-pp-cli threads get | Stable IDs, source timestamps, direction, explicit older-history cursor; unread guard means thread-unread status, never invented per-message unread certainty. |
| 6 | MMS, group participants and attachment metadata | mautrix/gvoice (source) GetThread and thread schemas | (behavior in google-voice-pp-cli threads get) Preserve verified text/MMS, participant identifiers, MIME, media IDs and dimensions when supplied | Preserve metadata without downloading or uploading media; unknown or absent fields remain unknown. |
| 7 | Call/voicemail metadata preservation | mautrix/gvoice (source) message/transcript schemas | (behavior in google-voice-pp-cli threads get) Preserve event type, duration and transcript when returned by the verified thread surface | No dedicated call/voicemail folder claim; unknown enum values preserved; no calling capability. |
| 8 | Bounded resumable local collection | User briefing; libgv ListThreads/GetThread pagination; Printing Press SQLite framework | google-voice-pp-cli mirror sync | Account-scoped ID deduplication; transactional page/checkpoint commits; bounded requests and retries; separate refresh and older-history work; report unfinished work truthfully. |
| 9 | Literal offline full-text lookup | User briefing; Printing Press SQLite search | google-voice-pp-cli search | Search locally mirrored text with stable source IDs and collection hints; no semantic interpretation or network call. |
| 10 | Structured local export | User briefing; official Voice export workflow as comparison | google-voice-pp-cli mirror export | Bounded JSON/JSONL export with source account/thread/message identifiers, timestamps and observed coverage; stdout or explicitly chosen local output only. |
| 11 | Bounded machine-readable output | Printing Press framework | (behavior in google-voice-pp-cli threads get) Shared --agent, --json and --select output and typed errors apply across the approved commands | Empty collections are arrays; credentials are never emitted; dry runs describe requests without auth or network. |
| 12 | Companion skill and read-only MCP access | googlevoice-mcp (source); Printing Press framework | google-voice-pp-cli mcp | The real command tree supplies accurate agent/MCP tools; local writes such as importing auth or running sync must be annotated honestly and never represented as pure reads. |

## Transcendence

Scores show **Domain Fit + User Pain + Build Feasibility + Research Backing**, with maxima **3 + 3 + 2 + 2**. Research backing is kept conservative where the finding supports the workflow and data shape rather than a separately documented demand for this exact command.

| # | Feature | Command | Score | Buildability | Persona served | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|----------------|--------------|----------|------------------|
| 1 | Compound message evidence | `history query` | **9/10** (3+3+2+1) | hand-code | Mike; chief-of-staff agent | This uses local account-scoped messages, thread participants, folder observations and attachment metadata to compute a bounded intersection of exact participant, message-time, direction, attachment and literal-text constraints with no external dependencies. | Brief Top Workflows 5 explicitly requires participant/direction/timestamp/attachment-preserving local filtering; Users and User Vision require recovering concrete past messages; mautrix/gvoice schemas supply the separate message/thread/attachment relationships; the parent-verified browser MCP read surface establishes why another list/read wrapper is insufficient. | Use this command to find message records matching participant, message-time, direction, attachment and literal-text constraints. Do NOT use this command to expand a known message into neighboring conversation; use 'history context' instead. |
| 2 | Message-anchored context packet | `history context` | **9/10** (3+3+2+1) | hand-code | Mike; chief-of-staff agent | This uses a local message's stable account/thread/message IDs, neighboring messages, attachment metadata and thread collection observations to compute a chronological evidence window with explicit clipping and known coverage boundaries with no external dependencies. | Brief Users describes repeated recovery of earlier context without scrolling; source schemas preserve stable IDs, timestamps, bodies and attachments; live thread replay returned 100 messages plus an older cursor, demonstrating that a returned thread is a bounded page rather than guaranteed whole history. | Use this command to retrieve neighboring messages around an exact message ID. Do NOT use this command to discover which messages match a question; use 'history query' instead. |
| 3 | Question-scoped coverage evidence | `mirror coverage` | **9/10** (3+3+1+2) | hand-code | Chief-of-staff agent; collector operator | This uses local folder inventory, per-thread source observations, message bounds, committed page/checkpoint records and sync failures to compute a participant/time-scoped coverage matrix with pending work and observation timestamps with no external dependencies. | Brief Users explicitly requires checking freshness before interpreting absence; Data Layer separates failures/partial runs/coverage from message activity; live list/get proved two pagination levels, including an older-history cursor; current mautrix/gvoice issue findings include disconnections and HTTP 429. | Use this command to inspect collection freshness, inventory discovery, pagination and failed-work evidence for a proposed lookup. Do NOT use this command to infer recent communication from import health; use 'history activity' instead. |
| 4 | Newly mirrored records | `history arrivals` | **7/10** (3+2+1+1) | hand-code | Collector operator; chief-of-staff agent | This uses local immutable first-seen observation/run IDs joined to message and thread records to compute a bounded stream of newly mirrored source IDs with separate source-message and collection timestamps with no external dependencies. | Brief Top Workflows 4 requires bounded resumable sync and source-ID deduplication; Data Layer requires transactional page/checkpoint commits; the operator's explicit ritual includes inspecting imports and repeated-collection behavior; live older-history pagination makes old backfill arrivals a concrete distinction from recent messages. | Use this command to inspect records newly added to the local mirror after a collection-run watermark, including old history found by backfill. Do NOT use this command to ask what communication happened recently; use 'history query' or 'history activity' instead. |
| 5 | Observed conversation activity | `history activity` | **8/10** (3+2+2+1) | hand-code | Mike; chief-of-staff agent | This uses local text/MMS timestamps and direction joined to exact thread participant identifiers and collection observations to compute per-thread inbound/outbound counts and latest observed source timestamps within a requested window with no external dependencies. | Brief Users and User Vision establish repeated recent-communication checks and separate relationship interpretation; source schemas expose timestamp/type/contact relationships and message direction can be normalized from the verified read surface; Data Layer forbids name/number guesses merging identities and separates collection state from activity. | Use this command for deterministic communication counts and last-observed inbound/outbound message times within a window. Do NOT use this command to retrieve exact message evidence; use 'history query' instead. |

The five novel capabilities are hand-code; zero are spec-emits.
The approved implementation must also supply the protocol/auth adapter and resumable collection underlying the core capabilities.
No source-mutation testing is part of this read-only scope.

## Complete source-feature disposition

This inventory makes the proposed trim reviewable.
Only the shipping rows above and the five transcendence rows are build commitments if Mike approves the reader scope.
Excluded functions do not become placeholder CLI commands.

| Source | Capability or public method | Proposed disposition |
|---|---|---|
| googlevoice-mcp | gv_login, gv_check_login | Match onboarding and verification through auth import-har/status; reuse existing login rather than launch resident Playwright Chromium. |
| googlevoice-mcp | gv_list_sms, gv_read_sms; all/unread filters and limits | Match through threads list/get, preserving explicit bounds and source read state. |
| googlevoice-mcp | gv_send_sms, gv_reply_sms, gv_mark_read | Exclude from proposed reader; require an explicit later scope decision to add source writes. |
| mautrix/gvoice libgv | GetAccount, ListThreads, GetThread | Match through account get and threads list/get. |
| mautrix/gvoice libgv | NewClient, GetCookies, SAPISIDHash, MakeRequest, ReadProtoResponse, ReadJSONResponse and auth/error helpers | Internal protocol/auth/error behavior, not separate command features; no raw-cookie dump command. |
| mautrix/gvoice libgv | DownloadAttachment | Exclude binary media download; preserve returned attachment metadata. |
| mautrix/gvoice libgv | AutocompleteContacts, LookupContact | Exclude PeopleStack service access and automatic person merging; preserve exact participants returned in Voice thread data. |
| mautrix/gvoice libgv | SendMessage, UploadPhoto, UpdateThreadAttributes, DeleteThread, GenerateTransactionID | Exclude source writes and their helpers. |
| mautrix/gvoice libgv | RegisterPushNotifications, SubscribeRealtimeChannel, RunRealtimeChannel | Exclude push/realtime registration and resident listeners; expose one-shot mirror sync. |
| mautrix/gvoice libgv | CreateWaa, PingWaa | Exclude send-signature browser machinery with the write path. |
| mautrix/gvoice bridge | Matrix bidirectional text/media, automatic portal creation and Matrix login/session machinery | Exclude Matrix infrastructure; provide a standalone CLI. |
| jaraco/googlevoice | Historical password-login wrapper | Research-only; README reports broken login, so no obsolete login flow is promised. |
| timelinize Google Voice importer | Google Takeout archive ingestion | Research-only alternative; this build verifies live reads and does not import a Takeout archive. |

## Implementation contract

- Three verified read RPC paths are the only remote request allowlist: account/get, api2thread/list and api2thread/get on the fixed Google client host.
- Request and response positional-array codecs must replace the rough sniff analyzer's inferred object schema.
- Recompute SAPISIDHASH per request; retain cookies/account selector in a private machine-local profile; reject redirects to untrusted hosts and unsafe auth/profile paths.
- The embedded public web-client query key is not a personal API key; do not ask Mike for VOICE_GOOGLE_API_KEY or document API-key authentication.
- Account identity is verified before checkpoint advancement; IDs and all local queries are account scoped; exact participants do not imply known-person identity.
- Full folder discovery, per-thread latest refresh and older-history exhaustion have separate observation timestamps, pending cursors and failure state.
- A timestamp range never proves complete history; an exhausted cursor is a source observation at the recorded time.
- Known text and archived folder enum mappings may be used after live verification; other folder mappings are unverified and cannot be advertised as supported by default.
- Each imported page, immutable first-seen observation and cursor update commits in one transaction; no pooled helper starts a nested write transaction.
- Novel local query implementations use parameterized SQL, close result rows before secondary lookups and declare pp:data-source local.
- Local query freshness/unsynced hints go to stderr; --data-source live is rejected for commands with no live equivalent.
- Auth imports, local sync and export describe their actual local side effects; --dry-run performs no network, secret reads or local data mutation.
- The generated generic framework sync/tail paths must either route through the correct Voice adapter/store contract or be removed from advertised support; no second shallow mirror can masquerade as a complete message import.
- Private cookies, raw bodies and message snapshots stay outside source, manuscripts and Dropbox; only sanitized outcomes and synthetic fixtures enter proof artifacts.

## Verification required to ship

- Meaningful codec fixtures for absent/null/new fields, type/direction/transcript/attachment handling, two pagination layers and malformed schema rejection.
- Auth allowlisting, expired/401 behavior, request signing and account-mismatch isolation; no auth in stdout, dry-run, error or debug output.
- Local tests for transactional rollback, cursor resume, first-seen stability, ID deduplication, SQL query boundaries and truthful coverage for partial/failed runs.
- Live ordinary-HTTP account/list/get, unread read-state preservation, bounded text/archive collection, repeat sync deduplication and all five novel commands against real collected records.
- Shipcheck, narrative examples, command/MCP discovery and the complete Printing Press dogfood matrix before promotion.
- If any shipping capability is infeasible, return to this gate for a concrete scope change rather than replace it with a stub.

## Sources and limits

- https://github.com/mautrix/gvoice at commit 166786fdd5a02c5987defd9c3a0a8f635e175cac, especially pkg/libgv/client.go and gvproto/threads.proto.
- https://docs.mau.fi/bridges/go/gvoice/authentication.html
- https://github.com/zhuqf/googlevoice-mcp, server.ts inspected directly; its read/write tools use Playwright/CDP page actions.
- https://github.com/jaraco/googlevoice
- https://github.com/timelinize/timelinize
- https://support.google.com/voice/answer/10130510

The consumer Voice web RPC is undocumented and may change; cookie sessions can expire and require another private import.
No source messages were sent, marked read, archived or deleted during discovery.
The generic sniffed spec is evidence of paths and array shape only and is not yet a ready-to-ship client contract.
