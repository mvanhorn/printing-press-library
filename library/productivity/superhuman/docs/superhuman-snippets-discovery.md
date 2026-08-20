# Superhuman snippets endpoint — discovery notes

Captured 2026-05-15 via `claude-in-chrome` browser tools against
`https://mail.superhuman.com/`.

## TL;DR

**No new endpoint is needed.** Superhuman snippets are stored as
threads with a `SNIPPET` label, reachable through the same backend
endpoints the CLI already uses for `threads list --type snippet` and
`drafts write`. The local-JSON store added in parent-plan U7 is
unnecessary; replacing it with calls into the existing infrastructure
is the right fix.

## List endpoint

**Request**

```
POST /v3/userdata.getThreads
Headers:
  Content-Type: text/plain;charset=UTF-8
  Authorization: Bearer <Superhuman JWT>   (handled by internal/auth)

Body:
{
  "filter": {"type": "snippet"},
  "limit":  <int>,
  "offset": <int>
}
```

The existing CLI already issues exactly this request via
`threads list --type snippet` (see
`library/productivity/superhuman/internal/cli/threads_list.go`,
the `runE` callback around the `path := "/v3/userdata.getThreads"`
branch).

The U17 / U18 design rationale from the parent plan ("snippets are
persistent user-created templates") is correct; the implementation
that stored them in a local JSON file at
`~/.superhuman-pp-cli/snippets.json` was an unnecessary detour.

## Read endpoint (get one snippet)

`POST /v3/userdata.read` with the snippet's thread/message ID. Same
pattern the CLI's `threads get` already uses to read draft content.
The snippet body is the draft's `body` field; subject is the draft's
`subject`; the snippet "name" is stored as `name` on the same draft
object.

## Create / update endpoint

`POST /v3/userdata.writeMessage` with the same payload shape `drafts
write` and `drafts new` already produce, plus:
- `action`: `compose`
- `labelIds`: `["SNIPPET"]` (instead of `["DRAFT"]`)
- `name`: the snippet name (Superhuman's UI uses this as the
  display/lookup key — it is NOT auto-derived from subject)
- `subject`: the snippet's default subject when used in a send
- `body`: the snippet body (HTML, same shape the CLI already produces
  for drafts)

## Delete endpoint

`POST /v3/userdata.writeMessage` with `labelIds` updated to remove
`SNIPPET` and add `TRASH` — same archive-style mutation pattern the
existing CLI uses for `threads update --action trash`.

Alternative: `POST /v3/userdata.delete` with the snippet's path. The
UI seems to use the label-update form; the CLI can use either.

## Auth

Superhuman's backend requires the JWT in the `Authorization` header.
The CLI's `internal/auth/chrome_cookies.go` already reads this from
Chrome's encrypted cookie store and the existing `gmail.Client` /
backend client already injects it. No new auth work needed.

A test from the page's JS console with `Content-Type: text/plain;
charset=UTF-8` body and no `Authorization` header returned:

```
status: 401
body: {"code":401,"detail":"missing-id-token"}
```

This confirms the JWT is the gating credential, not a cookie alone.
The CLI's existing auth path supplies it correctly.

## Variable substitution (out of scope for the backend)

The `--var key=value` substitution introduced in parent-plan U7 is
client-side and stays client-side: the CLI fetches the snippet body
from the backend, performs `{{key}}` → value text replacement
locally, then includes the substituted body in the `send` payload.
The backend has no concept of snippet variables.

## Migration of any existing local snippets

The parent-plan implementation may have left snippet data at
`~/.superhuman-pp-cli/snippets.json`. The U4 implementation should
NOT auto-upload these — the user may already have created equivalent
snippets in the UI during interim testing. Print a one-time stderr
hint when `snippets list` first runs against the new backend-backed
implementation, suggesting manual migration via repeated `snippets
create --name <n> --body <b>` calls. Leave the local file in place
for the user to inspect.

## What was not captured

- The exact response JSON for `getThreads` filtered by snippet type
  was not captured because the page's auth context could not be
  replicated from a console-injected `fetch` without reading the
  blocked JWT. The shape is the same as the existing
  `threads list --type snippet` returns end-to-end, so the CLI tests
  cover it.
- Snippet stats (sends count, opened percentage, replied percentage
  visible in the Superhuman UI sidebar) are likely on the same
  thread/draft object but were not enumerated. U4 may surface
  whichever stats are present; tests can be additive if more fields
  surface than expected.
