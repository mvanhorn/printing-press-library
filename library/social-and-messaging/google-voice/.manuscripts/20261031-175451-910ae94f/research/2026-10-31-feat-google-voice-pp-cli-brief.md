# Google Voice CLI Brief

## API Identity

- Target: Mike's personal Google Voice account and the `voice.google.com` web service.
- Data: direct/group SMS and MMS metadata, thread participants, timestamps and direction, call records, and voicemail transcripts when returned.
- Primary workflow: collect personal message history reliably into a local database, then read and search it without keeping a browser running.
- No consumer OpenAPI contract was found.
  Existing current code documents the web RPC protocol; official Google documentation documents Voice data export, not an equivalent consumer message API.

## Sources

- Official export scope: https://support.google.com/voice/answer/10130510
- Current Go implementation and protobuf schemas: https://github.com/mautrix/gvoice/tree/main/pkg/libgv
- Current authentication instructions: https://docs.mau.fi/bridges/go/gvoice/authentication.html
- Older Python wrapper: https://github.com/jaraco/googlevoice
  Its README explicitly says login is broken.
- Alternative offline export importer: https://github.com/timelinize/timelinize
  Google Takeout is an alternative archive source, not a substitute for verifying live Voice reads.

## Reachability Risk

High until an authenticated read is successfully replayed outside Chrome.
The controlled Chrome profile currently redirects Voice to Google's product page and offers sign-in; no signed-in Voice session has been established.
Native Chrome accessibility lookup timed out.
The modern Go implementation uses ordinary HTTP for reads and a separate browser signature mechanism for sending.
Do not generalize write-path failures into read-path impossibility.
Checked current mautrix/gvoice issue list: open #8 concerns random disconnections; closed #7 and #4 concern sending/anti-abuse signatures; closed #9 concerns HTTP 429.
Authentication documentation requires a single-account private session and warns that Device Bound Session Credentials can make replay unsuitable.
No new account, credential, or permission change is authorized by this brief.

## Top Workflows

1. Authenticate privately, verify the account, and report truthful session health without exposing cookies.
2. List personal text threads and archived threads with explicit opaque pagination cursors.
3. Fetch and paginate a thread without marking it read or changing its labels.
4. Resume bounded local SQLite sync, deduplicate by stable source IDs, and expose incomplete coverage.
5. Search/filter/export local messages with source account, participant, direction, timestamp, and attachment metadata preserved.

## Users

- Mike, checking what personal communication history is already collected and asking his assistant to recover relevant past messages across his existing messaging accounts.
  His established Signal/Telegram workflow uses an hourly local database and separate skills for interpretation.
  The recurring ritual is checking recent communication and recovering earlier context without manually scrolling each app.
- Mike's chief-of-staff agent, answering a concrete question from saved messages while preserving the difference between observed history and complete history.
  Its recurring ritual is querying an account-scoped local archive and checking source freshness before using an absence of messages as evidence.
- The operator of Mike's Mac Studio collectors, maintaining unattended imports with bounded retries and inspectable checkpoints.
  The existing system already separates hourly ingestion from relationship judgment.
  Its recurring ritual is checking successful imports, retrying gaps, and proving that repeated collection does not duplicate messages.

## Table Stakes

- Cookie-based web authentication and on-demand direct HTTP reads.
- Account/profile isolation and safe credential storage outside Dropbox.
- Text and group thread support, attachment metadata, reliable pagination, and actionable auth/rate-limit failures.
- JSON output with bounded page/row limits, field selection, and stable exit codes.
- Searchable SQLite mirror and an accurately documented companion skill/MCP surface.
- No Matrix server or always-running browser is required for ordinary CLI commands.

## Protocol Contract Observed in Source

- RPC origin: `https://clients6.google.com/voice/v1/voiceclient`.
- Read RPCs are HTTP POST: `/account/get`, `/api2thread/list`, `/api2thread/get`.
- Source client also supplies read-only contact autocomplete/lookup RPCs on Google's PeopleStack service.
- Authentication uses Google session cookies, `X-Goog-AuthUser`, and a time-bound `SAPISIDHASH` derived from the Voice origin.
- Requests use protobuf JSON arrays (`application/json+protobuf`); responses may be protobuf, protobuf JSON arrays, or a framed/base64 safety encoding.
- `ListThreads` returns thread rows plus pagination and version tokens.
- `GetThread` returns a thread with message rows and a thread-specific pagination token.
- Source enums identify text folder 2 and archived folder 6.
  Calls/voicemail/spam folder mappings are explicitly uncertain in source and must not be advertised as verified before a live check.
- Source proto preserves message ID, timestamp, type, read status, text/MMS content, contact/number, transcript, duration, and attachments.

## Data Layer

- Entities: account, thread, thread participant, message/event, attachment metadata, folder membership, source observation, sync run, and per-thread/folder cursor.
- Account identity is verified before any checkpoint advances.
- Source IDs are deduplication keys; phone/name guesses never merge identities automatically.
- Store private bodies and auth only in machine-local runtime data with 0700 directories and 0600 files.
- Commit each imported page and its continuation checkpoint in one SQLite transaction.
- Record partial runs, failures, and coverage separately from last message activity.
- Keep local search literal/parameterized and ensure empty list outputs are arrays.
- Do not install an unattended collector or modify the existing relationship-memory database as part of an unverified scaffold.

## User Vision

Mike invoked Printing Press for Google Voice immediately after reviewing the Signal/Telegram personal-message database.
Prioritize the message reading, sync, search, and export primitives that would support the same use case.
Do not send, call, mark read, archive, delete, change settings, or upload during research or verification.
Agree the concrete feature manifest before generation.

## Product Thesis

- Name: `google-voice-pp-cli`.
- Thesis: reliable, bounded Google Voice message access and a searchable local mirror through a standalone Go CLI.
- Incumbent distinction: contemporary protocol support and local queries without Matrix infrastructure or obsolete password login scraping.

## Build Priorities

1. Prove single-account authentication and a read-only HTTP replay; make failure states explicit.
2. Generate the Printing Press CLI, skill, and MCP scaffold from an evidence-backed read surface, then adapt the protocol transport where generic JSON cannot express it faithfully.
3. Implement page-safe thread reads and local sync/search/export with meaningful failure/pagination/account-isolation tests.
4. Run shipcheck and a live read-only matrix, including repeated sync deduplication and cursor/coverage checks.
5. Promote only when the approved features and live proof pass; preserve an honest hold if authentication or replay is unavailable.

## Current Setup

No matching Google Voice entry in the current 600-entry public registry or local library.
The blocked-API journal URL returned 404, which is non-blocking under Phase 0.
Mike approved use of a verified Nix Printing Press 4.35.0 build for this run.
The active system profile remains 4.34.2 until a later rebuild.

## Live Discovery Update

Mike explicitly approved read-only authenticated capture using his existing login.
The signed-in personal account was located in Dia and inspected in a fresh tab.
Captured the real account and thread-list requests in a private 0600 HAR outside the run/archive directories.
Restored the temporary DevTools export preference and closed only the fresh capture tab.
Direct Python HTTP replay returned 200 for account and thread-list reads; both returned 401 with cookies and authorization removed.
Thread list returned 20 rows.
A source-derived read-only thread-detail request returned 200, matched the requested thread, returned 100 messages, and exposed an older-history cursor.
No message was sent, marked read, or archived by the verification requests.
The source and captured protocol use JSON arrays rather than ordinary JSON objects.
The generic sniffed spec misclassifies the public web-client query key as personal API-key authentication and cannot infer the positional schema.
Generation must repair that spec and implement an explicit positional-codec/auth adapter rather than shipping the rough sniff output unchanged.
Plain standard HTTP replay works; a resident browser or a TLS-fingerprint workaround is not required for the verified reads.

## Ecosystem scope update

Inspected zhuqf/googlevoice-mcp server.ts directly.
Its seven tools include login/check, list/read with unread filters, send/reply and mark-read, using resident Playwright Chromium.
The proposed reader matches its read/onboarding functions through standalone HTTP and explicitly asks Mike to exclude its source writes.
The manifest inventories the current Go SDK public protocol operations, including download, PeopleStack contacts, writes, realtime and Matrix features excluded from the proposed reader.
No exclusion is treated as approved until the phase 08 scope decision.
The required three-pass brainstorm retained five local deterministic features, all requiring hand-written implementation after generation.
