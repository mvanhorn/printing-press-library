# peloton-pp-cli — Agent Guide

This directory ships a hand-rolled `peloton-pp-cli` printed CLI. It is
**not** generator-emitted — there is no Peloton OpenAPI spec to feed
[CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so
every endpoint is reverse-engineered from the `members.onepeloton.com`
SPA and the cobra command tree is hand-written. Treat shape changes
(SKILL.md frontmatter, README sections, exit-code conventions) as
upstream Printing Press conventions worth aligning with; treat
endpoint / field changes as local Peloton-SPA-shifted-on-us bugs to fix
in `internal/client/`.

## Local Operating Contract

This CLI does **not** ship the generator-template `--agent` /
`--select` / `--deliver` / `--profile` / `feedback` / `doctor` /
`agent-context` / `which` suite. The contract is leaner; agents should
rely on:

- **Auto-JSON when piped.** Every list/show/search command emits JSON
  to stdout when stdout is not a TTY. `--json` forces it on a TTY.
- **`--compact`** projects to high-gravity fields (one-line JSON,
  60–80% token reduction). Implies `--json`.
- **Typed exit codes** (0 / 2 / 3 / 4 / 5 / 7) — see SKILL.md.
- **`peloton-pp-cli --help`** + `peloton-pp-cli <command> --help`
  for runtime discovery. There is no `which` or `agent-context`
  subcommand.

Before touching local state, prefer to read first:

```bash
peloton-pp-cli auth status     # is a token even saved?
peloton-pp-cli me              # who's it for, how old is it?
peloton-pp-cli sync --limit 1  # tiny smoke before a full backfill
```

`auth login` is the **only** command that spawns Chrome; do not run it
from a long-lived agent loop. Bootstrap interactively, then run
everything else against the saved token.

## Reverse-engineering posture

Every endpoint and the Auth0 localStorage harvest are reverse-engineered
from members.onepeloton.com SPA traffic. When something breaks:

1. Open `members.onepeloton.com` in a real browser, open DevTools
   Network tab, and reproduce the action that's broken (load workouts,
   open a ride detail page, etc.).
2. Compare the live SPA request/response shape against the on-the-wire
   types in `internal/client/client.go` (`rawWorkout`, `rawRideDetails`,
   `rawList`).
3. Patch the `raw*` types and the projection in `fromRaw` /
   `GetRideDetails`; keep the public `Workout` / `RideDetails` /
   `Song` shapes stable so callers (CLI, MCP, store) don't have to
   move in lockstep.

If the Auth0 SPA cache key format itself changes, fix
`readTokenExpr` in `internal/cli/auth_login.go` — the JS scans
`localStorage` for the prefix and the `body.access_token` shape, so a
key-format shift is the most likely break.

## Local Customizations

This CLI was hand-rolled, so the
`.printing-press-patches.json` convention used by generator-emitted
CLIs does not apply here directly. Code-level customizations should
still be marked at the call site with a one-line comment:

```go
// PATCH: <one-line summary of why this differs from the obvious shape>
```

…so a future maintainer can `grep -rn 'PATCH' .` and surface every
intentional deviation. There's no separate manifest to update beyond
that.

## Testing against a real account

Live tests need a Peloton account. The smoke loop is:

```bash
go build ./... && go vet ./...
go build -o /tmp/peloton-pp-cli ./cmd/peloton-pp-cli
/tmp/peloton-pp-cli auth status                 # exits 4 if no token
/tmp/peloton-pp-cli workouts list --limit 3     # newest-first, JSON when piped
/tmp/peloton-pp-cli sync --limit 5              # populate the local store
/tmp/peloton-pp-cli search 'house' --limit 5    # FTS5 sanity
```

If `workouts list` returns an empty array but `me` looks healthy, the
saved token is probably stale — `auth login` again.
