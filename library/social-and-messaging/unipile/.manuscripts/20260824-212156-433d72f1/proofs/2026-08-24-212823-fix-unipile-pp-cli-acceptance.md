# Acceptance Report: unipile

  Level: Full Dogfood (operator chose "full + writes")
  Tests: 267/267 passed (100%), 336 skipped, 0 failed
  Verdict: PASS

## Write-surface note

The operator opted into exercising writes. The live matrix nevertheless ran every
mutating command with `--dry-run`, which is the harness's own policy and was not
overridden. No message, invitation, post, comment, reaction, calendar event,
webhook, or account deletion was actually performed against the live tenant.
`accounts delete <real-account-id> --dry-run` passed as a dry run only; the
connected account was not unlinked. `--allow-destructive` was deliberately not
passed, since it re-enables credential-destroying endpoints.

## What ran for real

Read-only paths executed against the live tenant with the operator's key:
account listing, chats, messages, chat attendees, relations, invitations sent and
received, webhooks, folders, and every local-store novel command. A full sync
pulled 21,612 records across 20 resources with zero errors.

## Skips

336 skipped, all structural rather than failures:
  - 65 blocked-fixture: an endpoint needs an API-side identifier the matrix cannot synthesise
  - 60 error_path skipped for commands that take no positional argument
  - 35 + 31 mutating commands limited to dry-run by harness policy
  - 24 request bodies the matrix cannot synthesise (provider `oneOf` unions)
  - the remainder are list-companion chains whose parent lookup did not yield a usable id

## Failures fixed inline during this phase

  - `contact` / `thread`: returned exit 0 for a miss; now exit 3 on a populated
    mirror and exit 0 with a sync hint on an empty one (CLI fix)
  - `feedback --help`: no Examples section (CLI fix)
  - `chat-attendees sync`: no distinguishable error path; annotated
    `pp:no-error-path-probe` (CLI fix, API limitation)
  - `search`: happy path and json_fidelity were skipped for want of a positional
    fixture, which made coverage hollow; added `pp:happy-args` (CLI fix)

## Printing Press issues for retro

  - Cursor-parameter detection binds to an unrelated `after` filter (fix #1 above)
  - MCP input-name collision aborts generation for any spec whose list routes
    expose a wire `cursor` param, with an error naming two remedies that do not
    apply to OpenAPI input
  - Hyphenated `ParentTable` produces a foreign-key name that cannot match the
    underscored typed column, silently failing every parent-scoped projection
  - The env-derived global scope param is written to the flat-list bucket, so
    path-scoped dependents never receive it

  Gate: PASS
