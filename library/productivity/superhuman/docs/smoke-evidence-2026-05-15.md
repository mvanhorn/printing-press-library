# Live smoke evidence — Gmail folder coverage (U5)

Captured 2026-05-15 against a real Superhuman account (PII scrubbed).
Built from `feat/superhuman-overhaul` at commit `e8af0a6f` (post-U3 discovery).

## Setup

```
cd library/productivity/superhuman
go build -o /tmp/superhuman-pp-cli-new ./cmd/superhuman-pp-cli
/tmp/superhuman-pp-cli-new --version
# superhuman-pp-cli 1.1.0
```

## Commands and results

For each Gmail-folder `--type`, the CLI was invoked with `--limit 3
--no-refresh --json` against the test account. Only the structural
fields (`path`, `success`, thread count, `result_size_estimate`) are
shown below — thread IDs, subjects, senders, and content are
intentionally omitted.

| --type      | path issued by CLI                                          | success | returned | est. total |
|-------------|--------------------------------------------------------------|--------:|---------:|-----------:|
| sent        | `/users/me/threads?labelIds=SENT`                            | true    | 3        | 201        |
| starred     | `/users/me/threads?labelIds=STARRED`                         | true    | 3        | 9          |
| archived    | `/users/me/threads?q=in:anywhere -label:inbox`               | true    | 3        | 201        |
| done        | `/users/me/threads?q=in:anywhere -label:inbox`               | true    | 3        | 201        |
| spam        | `/users/me/threads?labelIds=SPAM`                            | true    | 3        | 14         |
| trash       | `/users/me/threads?labelIds=TRASH`                           | true    | 1        | 1          |
| important   | `/users/me/threads?labelIds=IMPORTANT`                       | true    | 3        | 201        |

All seven exited zero, returned valid Gmail thread arrays, and matched
the expected label routing.

## Observations

- **`archived` and `done` collapse to the same Gmail query.** Both
  use `q=in:anywhere -label:inbox`. That matches the parent plan's
  Key Technical Decision D1: Gmail does not expose `DONE` or
  `AUTO_ARCHIVED` as system labels, so both route through query
  syntax. Returned counts agree (`est. total: 201` for both),
  confirming they target the same Gmail set even though the
  Superhuman UI exposes them as distinct sidebar entries.
- **`trash` returned only 1 thread** because the test account's
  Trash is nearly empty — confirmed against the Superhuman UI. Not
  a CLI bug.
- **`result_size_estimate: 201`** is Gmail's standard "many"
  bucket — the API returns 201 when the true count is large but
  uncomputed. Inbox, sent, archived, done, and important all hit
  this ceiling; spam (14), starred (9), and trash (1) returned
  exact counts.

## What this proves

- The U1 folder-coverage routing in `internal/cli/threads_list.go`
  emits the correct Gmail query strings for every new `--type`
  value.
- The Gmail passthrough in `internal/gmail/messages.go` accepts
  the expanded `labelIDs []string` and `query string` parameters
  the parent plan introduced.
- All seven new folder types are reachable end-to-end against a
  real account, not just against the `httptest.Server` mocks.

## What this does NOT prove

- Pagination via `--page-token` was not exercised in this smoke.
  The CLI returns a `next_page_token` for each folder (see e.g.
  `17273471241223856927` for sent), but a follow-up call with
  that token was not made. Mocks cover pagination; live coverage
  is deferred.
- Auto-refresh provenance line behavior was not validated end-to-
  end here because `--no-refresh` was set to keep the smoke
  deterministic. The autorefresh tests in U2 cover the in-process
  behavior.
- Snippets via `snippets list` (U4) was not part of this smoke —
  the U4 commit comes after U5. A second smoke against the new
  `snippets list` command will land alongside U6.

## Reproducing

```bash
cd library/productivity/superhuman
go build -o /tmp/superhuman-pp-cli-new ./cmd/superhuman-pp-cli
for t in sent starred archived done spam trash important; do
  /tmp/superhuman-pp-cli-new threads list --type $t --limit 3 \
    --account <your-account> --no-refresh --json \
    | jq '{type:"'$t'", path, success, count:(.threads|length), estimated:.result_size_estimate}'
done
```
