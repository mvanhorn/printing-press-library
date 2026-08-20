# Superhuman Reminder API — Browser Sniff Report

Date: 2026-05-15
Plan: docs/plans/2026-05-15-003-fix-superhuman-reminder-api-payload-plan.md
Account observed: mvanhorn@gmail.com (web app session)
Account used for CLI verification: mvh@esperlabs.ai

## Top-line finding

The CLI's `reminders create` and `reminders cancel` commands call `/reminders/create` and `/reminders/cancel`. The Superhuman web app does **not** use those endpoints. Web-app reminders go through the generic `/v3/userdata.write` endpoint as path-keyed writes.

`/reminders/create` exists on the backend (returns 400, not 404), but every body shape we tried gets the same `{"code":400,"error":"missing threadId"}`. The endpoint is either deprecated, gated on a header we are not sending, or expects a wrapping shape the CLI does not produce.

## Web-app observation

Steps:
1. Open thread `19e228de8b1bee34` (Andrea Jacobi, "RE: Tenant Lease Ending")
2. Press `H` to open "Remind me" dialog
3. Type `5 minutes` — UI accepts it and shows "in 5 minutes 4:50 PM" as the preview row
4. Press Return

The web app accepted "5 minutes" without complaint. It also accepted "3 minutes", "2 minutes", and "1 minute" on follow-up runs. The web app has no sub-hour minimum.

Network capture (via `mcp__claude-in-chrome__read_network_requests` filtered to `~backend`):

```
url: https://mail.superhuman.com/~backend/v3/userdata.write
method: POST
statusCode: 200
```

The request body itself could not be extracted via page-level fetch/XHR interception — Superhuman registers a Service Worker (`https://mail.superhuman.com/~backend/build/serviceworker.js`) that routes writes off the page's `fetch`/`XHR` surface. Both `window.fetch` and `XMLHttpRequest.prototype.send` interceptors caught zero reminder-shaped requests. The SW also masks the page's `Authorization` header so a direct `fetch('/v3/userdata.read', ...)` from devtools returns `401`.

What we can infer with high confidence:

- Endpoint: `POST /v3/userdata.write`
- Cancel (un-snooze) is the same endpoint with `"value": null` on the same path (Superhuman's convention for "delete" via `userdata.write` per the comment at `send.go:487` — "writing null on the FULL value").
- Reminder path: `users/<userId>/threads/<threadId>/reminder`

**Body shape for create — actual shape recovered by reading a snoozed thread back via `/v3/userdata.read`**:

```json
{
  "clientCreatedAt": "2026-05-15T23:45:39.257000000Z",
  "keepOnReply": false,
  "messageIds": ["19e228de8b1bee34", "19e2792e318ad31c", "..."],
  "onDesktop": false,
  "reminderId": "6e0940b4-7f96-4654-9a9d-e5a8857d42e7",
  "source": "USER",
  "threadId": "19e228de8b1bee34",
  "triggerAt": "2026-05-15T23:50:00.000000000Z"
}
```

Important corrections to the initial hypothesis:

- `triggerAt` is an **ISO-with-nanos string**, not unix-ms int. The integer form does exist on the `sendReminder` embedded shape under `/v3/userdata.writeMessage`, but standalone thread reminders use the ISO form.
- There is **no `condition` field**. The web app's "if no reply" toggle maps to `keepOnReply: bool` -- `keepOnReply: true` means the reminder fires regardless of reply (CLI surfaces as `--condition always`); `keepOnReply: false` cancels the reminder if a reply arrives (CLI surfaces as `--condition if-no-reply`).
- `messageIds` is the **list of the thread's message ids**. The reminder write requires it -- a missing or empty `messageIds` causes the backend validator to reject the payload with a content-less 400. The CLI fetches this via `/v3/userdata.read` of the thread before issuing the write.
- `reminderId` is a fresh UUID per reminder. The CLI generates it via `google/uuid.NewString()`.
- `source: "USER"`, `onDesktop: false`, `clientCreatedAt: <now-in-ISO-nanos>` are required envelope fields. Missing any of them yields the same 400.

**Empirical confirmation, post-rewrite (mvh@esperlabs.ai account, fresh inbox thread):**

```
reminders create  -> HTTP 200, currentHistoryId advances
reminders cancel  -> HTTP 200, currentHistoryId advances
```

The reminder is visible in the Superhuman UI immediately after the CLI write.

## CLI empirical confirmation

Tested against thread `19d64dc254c28380` (an old inbox thread on the mvh@esperlabs.ai account, guaranteed ingested). Result: every variant of the body produces the same error.

```
$ TS=$(date -u -v+10M +"%Y-%m-%dT%H:%M:%SZ")

# Variant 1: ISO string, camelCase
$ echo "{\"threadId\":\"19d64dc254c28380\",\"triggerAt\":\"$TS\"}" \
    | superhuman-pp-cli reminders create --stdin --agent
Error: POST /reminders/create returned HTTP 400: {"code":400,"error":"missing threadId"}

# Variant 2: unix-ms int, camelCase (matches sendReminder shape)
$ TS_MS=$(($(date -u -v+10M +%s) * 1000))
$ echo "{\"threadId\":\"19d64dc254c28380\",\"triggerAt\":$TS_MS}" \
    | superhuman-pp-cli reminders create --stdin --agent
Error: POST /reminders/create returned HTTP 400: {"code":400,"error":"missing threadId"}

# Variant 3: with condition field
$ echo "{\"threadId\":\"19d64dc254c28380\",\"triggerAt\":$TS_MS,\"condition\":\"always\"}" \
    | superhuman-pp-cli reminders create --stdin --agent
Error: POST /reminders/create returned HTTP 400: {"code":400,"error":"missing threadId"}
```

`/reminders/create` is not the right path. The CLI was generated against an old or speculative API surface that the web client does not exercise.

## Confirmed CLI bugs (from session 2026-05-15)

1. **Wrong endpoint.** `reminders create` calls `POST /reminders/create`. The real endpoint for thread snooze is `POST /v3/userdata.write` with a path-keyed `reminder` write. `reminders cancel` has the same problem (`POST /reminders/cancel`).

2. **Wrong body shape.** Even if the endpoint were right, the CLI emits flat snake_case keys (`thread_id`, `trigger_at`, `reminder_id`). The API everywhere else in this CLI uses camelCase, and the reminder field shape is `{triggerAt: int64-ms, condition: "always"|"if-no-reply"}` per the existing `sendReminder` struct.

3. **`--remind-in` 1h floor is wrong.** Validation at `send.go:642-644` rejects any duration `< time.Hour` with `--remind-in must be at least 1h`. The web app accepts 1-minute reminders; the 1h floor is purely a CLI-side fiction. `--remind-on` with an RFC3339 timestamp 5min out bypasses this validation cleanly, which is how we confirmed the floor is the only blocker.

4. **(Open) Freshly-sent thread 400.** When we tried to set a reminder on a thread that was sent <30 seconds prior, even via the right endpoint shape from devtools we hit 401 (auth-header issue not API-shape issue). Likely orthogonal to the bugs above. Defer per plan.

## What this changes about the plan

The plan's U2 was "swap snake_case to camelCase on `reminders_create.go` and `reminders_cancel.go`." That is insufficient. The right fix is to **rewrite both commands to call `POST /v3/userdata.write` with the path-keyed write shape** — the same pattern `send.go` already uses for `writeMessage`.

Practical implication for U2 scope:

- Both commands switch endpoint annotations from `/reminders/create` / `/reminders/cancel` to `/v3/userdata.write`.
- Both commands need the user ID (path prefix `users/<userId>/...`). The CLI already resolves user ID elsewhere in this codebase — `send.go` references it in dry-run output (e.g. `users/101619200793245775104/threads/...`). Reuse the same helper.
- `reminders create` payload becomes `{writes:[{path: "users/<uid>/threads/<tid>/reminder", value: {triggerAt: <ms>, condition: <s>}}]}`.
- `reminders cancel` payload becomes `{writes:[{path: "users/<uid>/threads/<tid>/reminder", value: null}]}` (same path, null value to delete). The CLI's existing `--reminder-id` flag is meaningless under this shape — reminders are keyed by thread, not by a separate reminder ID. The `--reminder-id` flag should either be removed or repurposed as a synonym for `--thread-id` for backward compat.
- Add an optional `--condition always|if-no-reply` flag on `reminders create` to surface the "if no reply" semantic that the web app exposes.

U3 is unchanged: lower the floor.

U4 is unchanged in shape, but the patch entries will list more files than originally anticipated.

## Multi-CLI template check (U4)

Ran `grep 'body\["\w\+_id"\]' library/**/internal/cli/*_create.go` across the published catalog. Three other CLIs use snake_case body keys in their generated create commands:

- `library/ai/openrouter/internal/cli/keys_create.go` -- `creator_user_id`, `expires_at`, `workspace_id`
- `library/media-and-entertainment/substack/internal/cli/drafts_create.go` -- `draft_title`, `draft_subtitle`, `draft_body`
- `library/productivity/notion/internal/cli/views_create.go`

Of those, OpenRouter and Substack both have **upstream APIs that genuinely use snake_case** -- the generator's emission is correct for them. So the snake_case body emission is not a generator template bug; the bug is that the generator does not track the upstream API's casing convention when emitting bodies. That is a deeper feature gap (a per-CLI casing decision, ideally informed by spec ingestion or a sniff phase), not a one-line template fix.

Decision: no generator-side companion fix is filed for this round. The Superhuman rewrite is recorded as a one-off published-library patch in `.printing-press-patches.json`. If a future CLI lands with camelCase API requirements and hits the same body-shape mismatch, that justifies revisiting the generator's casing logic upstream.
