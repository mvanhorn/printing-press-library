Manifest transcendence rows: 10 planned, 0 built. Phase 3 will not pass until all 10 ship.

# Unipile CLI Build Log

## Generation
- Spec: canonical tenant OpenAPI from `https://{DSN}/api-json` (74 paths / 94 operations), cross-checked against all 94 per-endpoint OpenAPI fragments in the public reference `.md` pages.
- Spec preprocessing applied before generate:
  - Rewrote all 94 `operationId`s. ReadMe emitted junk single-letter prefixes (`T_listAccounts`, `h_listAllChats`, `u_listChatsByAttendee`), which produced command names like `chats h-list-all` and `chat-attendees u-list-by-attendee`. Now: `chats list`, `chat-attendees chats`, `accounts solve-checkpoint`.
  - Stripped the wire `cursor` query param from all 25 paginated GET operations so the generator's own pagination input owns `cursor` and `--all` works.
  - Pinned `Access-Token` (X-API-KEY) over the spec's unused `bearer` scheme; attached `x-auth-env-vars: [UNIPILE_API_KEY]` and `x-auth-vars`.
  - Added `x-learn` provider vocabulary (13 provider canonicals + aliases, 9 domain synonyms).
  - Rewrote `servers` to the operator DSN with a documented `UNIPILE_BASE_URL` override.
- Generated: 32 resources, 94 endpoint commands. All quality gates PASS (go mod tidy, go test, govulncheck, go vet, go build, --help, version, doctor).
- MCP: Cloudflare pattern auto-applied (94 > 50) — code orchestration, endpoint tools hidden, stdio+http transport.

## Generator issues found (retro candidates)
1. **MCP cursor-input collision on every cursor-paginated GET.** `generate` aborted with `MCP input names collide: the generated pagination input and query input from wire param "cursor" both emit schema key "cursor"`. Hit 25 operations. Workaround: strip the wire `cursor` param from the spec. `x-pp-pagination` is documented in the error message as a remedy but only accepts a string, and neither `"cursor"` nor a `{cursor_param: ...}` map resolves it (`"cursor"` is rejected as "not supported"; `"none"` suppresses generated pagination and therefore `--all`).
2. **`x-pp-pagination` error message is misleading.** It suggests `flag_name` or `x-pp-param-url-names`, neither of which is documented for OpenAPI input, and it accepts `"none"` but not `"cursor"`.
3. **Novel-feature stub skipped silently-ish for name collisions with framework commands.** `novel feature command "search" maps to existing internal/cli/search.go without expected constructor newNovelSearchCmd; skipping novel stub` — correct behaviour here (framework `search` already delivers the capability) but worth a clearer message.

## Body-shape limitations carried forward
Nine POST/PATCH bodies use `oneOf`/`anyOf` provider unions and fall back to `--body-json`:
`POST /accounts`, `POST /webhooks`, `POST /accounts/{id}`, `POST /linkedin/search`, `POST /hosted/accounts/link`, `POST /linkedin/user/{user_id}`, `POST /linkedin/jobs/{draft_id}/publish`, `POST /users/invite/received/{invitation_id}`.
`PATCH /users/me/edit` body is skipped entirely (multipart/form-data + oneOf).

## Transcendence build
Manifest transcendence rows: 10 planned, 10 built.

All ten approved transcendence commands ship, plus the `accounts alias` behavior
row (manifest row 36). No stubs.

| # | Command | Shape | Notes |
|---|---------|-------|-------|
| 1 | `budget` | local | Invitation headroom per 24h / 7d against configurable caps, counted from synced invitation history so UI-sent invitations are included. |
| 2 | `search` | local | Framework FTS command, re-pointed at the local mirror by default (see below) and un-hidden from MCP. |
| 3 | `contact` | local | Joins connections, invitations sent/received, chats, and per-direction message counts for one human. |
| 4 | `inbox` | local | Unread across every provider, newest first, with the last message and who sent it. |
| 5 | `silent` | local | Threads where you spoke last, bounded by `--days` and `--max-days`. |
| 6 | `accepted` | local | Connections formed in the window with no message exchanged, via relation/chat set-diff. |
| 7 | `funnel` | local | Sent / received / accepted / replied with an explicit cohort caveat. |
| 8 | `digest` | local | Per-resource new-in-window counts computed from record timestamps. |
| 9 | `thread` | local | One conversation with attendee ids resolved to names. |
| 10 | `engagement` | local | Post reactions/comments crossmatched against connections. |
| 36 | `accounts alias` | local | Resolves `linkedin` / `li` / an account name to the 22-char account id; `--export` emits a shell line. |

## Post-generation patches to generated files

`pipeline/fix-pagination.py` rewrites two generated pagination tables. Kept as a
script so the edit is reproducible after any regeneration.

**Why:** Unipile paginates every list route with `?cursor=` and returns the next
cursor in the response envelope. The generator's cursor detection instead picked
the unrelated `after` ISO-datetime filter on chats/emails/messages, and left
`cursorParam` empty on routes whose only pagination params are `cursor`+`limit`
(accounts, chat-attendees, users, webhooks). Live effect before the patch: page
two of any sync failed with `400 errors/invalid_parameters ... "Expected union
value"` because the base64 cursor was sent as `after`. After the patch a full
sync pulls 21,612 records clean.

Patched: `internal/cli/resource_paths.go` (8 entries),
`internal/cli/sync.go` `determinePaginationDefaults` (24 case blocks),
6 promoted list command call sites.

`internal/cli/search.go` is hand-edited for three reasons:
1. Default data source flipped from `auto` to the local mirror. The API's only
   search route is `POST /api/v1/linkedin/search`, which spends LinkedIn's
   ~1000-results/day budget; auto-routing a bare `search` there is a footgun on
   an API whose own docs state Unipile enforces no limits. `--data-source live`
   still opts in, and `linkedin search` is the full live surface.
2. Provenance reason for the default path changed from `api_unreachable` to
   `local_default` - no API call is attempted, so reporting one as unreachable
   was false.
3. Freshness anchored on the newest `sync_state` row. `search` is never itself a
   synced resource, so the stock lookup reported "local store has not been
   synced yet" on every query against a fully synced mirror.
4. `mcp:hidden` replaced with `mcp:read-only` - offline cross-provider search is
   a headline capability and agents should see it.

## Correctness bugs found and fixed during dogfooding

1. **`digest` counted sync time, not record time.** Every resource reported
   `NEW == TOTAL` after any fresh sync. Now each resource maps to the SQL
   expression yielding its own record timestamp; resources with no usable
   timestamp report `?` and are named explicitly rather than being silently
   counted as new.
2. **`silent` surfaced archaeology first.** Sorting by days-silent descending
   put 2,100-day-old threads at the top. Now ordered most-recently-gone-silent
   first with a `--max-days 90` ceiling (`0` disables).
3. **`funnel` reported a 120% accept rate.** Connections accepted in a window
   include invitations you sent earlier and invitations other people sent you.
   The metric is now named `accepted_per_sent_pct`, invitations received are
   reported alongside, and a cohort caveat fires whenever accepted exceeds sent.
4. **Names came back as raw provider ids.** Only 731 of 1,933 chats join to the
   connection list. Syncing `chat-attendees` (note: the resource name is
   hyphenated; `chat_attendees` fails silently) covers 1,933/1,933 by
   `provider_id` and 6,779/6,915 message senders by attendee id.
5. **`accounts alias <name>` emitted JSON when piped**, breaking the documented
   `--account-id "$(... alias linkedin)"` pattern. Single-alias resolution now
   keys off explicit machine-format flags, not pipe detection.

## Known local-mirror scoping behaviour

The SQLite path is scoped by credential and account id. Syncing with
`UNIPILE_ACCOUNT_ID` set and then querying without it silently reads a different,
empty database. Documented in the README/SKILL troubleshooting entries rather
than worked around, since per-account scoping is correct for a multi-account API.

## Manuscript trimming before publish

Two artifacts were removed from the archived manuscripts to keep the public-library
diff proportionate:

- `research/unipile-openapi.json` (3.5 MB) — the intermediate spec reconstructed by
  merging the 94 per-endpoint OpenAPI fragments. `research/unipile-spec.json` (the
  exact generate input) and `research/unipile-api-json.json` (the raw tenant fetch)
  both supersede it, and this log already records that the two agreed on coverage
  (74 paths / 94 operations).
- `discovery/ref-md/` (4.0 MB, 94 files) — verbatim copies of Unipile's public
  reference pages. `discovery/ref-urls.txt` retains every source URL, so the capture
  is reproducible without duplicating vendor documentation into the library.
- `discovery/guides/` (10 files) — verbatim copies of Unipile guide pages, replaced by
  `discovery/guide-urls.txt`. These also tripped the packager's PII gate on
  a WhatsApp JID-shaped identifier, which is Unipile's own address-format example in
  the "Retrieving users" guide rather than customer data; dropping the copies removes
  the false positive along with the duplication.
- `research/unipile-spec.json` (3.6 MB) — byte-identical to the CLI's own `spec.json`,
  which the package already ships at the CLI root. `research/unipile-api-json.json`
  remains as the raw upstream fetch this spec was derived from.
