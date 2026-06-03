# pp-sync — private-as-source-of-truth Printing Press sync

`pp-sync` keeps our agent-facing `*-pp-cli` tools built from **our private library**
(`cathrynlavery/printing-press-library`) and treats the public fork
(`mvanhorn/printing-press-library`) strictly as **upstream-for-detection** — we fetch it
to notice new/changed CLIs, but we never build from it and never push to it.

## Why this exists

Before this, the daily knox cron pulled the private repo (correct) but then installed the
actual binaries from the **public** `@mvanhorn/printing-press-library` npm package and
`go install …mvanhorn…@latest` (public GitHub). So our private edits (e.g. the `kit`
composition commands, `ahrefs` composite commands) never reached agents. `pp-sync` builds
every installed CLI from the private checkout instead, making private the real source of
truth for what agents run.

## Install (knox)

The deployed copy lives at `~/.local/bin/pp-sync`. The daily cron wrapper
(`~/.hermes/scripts/sync_printing_press_library.sh`) self-updates it from this repo copy
(`tools/pp-sync/pp-sync`) on each run, so merging a change here propagates to knox within a
day. To force it: `install -m 0755 tools/pp-sync/pp-sync ~/.local/bin/pp-sync`.

## Commands

| Command                     | What it does                                                                                                                                                                               |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `pp-sync update`            | (cron default) ff-pull private `main`, build the installed CLI set from private, sync skills from `cli-skills/`, smoke-test, diff against public upstream, write + print the daily report. |
| `pp-sync upstream` (`diff`) | Read-only. Lists **new** upstream CLIs and CLIs where **upstream is ahead** of us (actionable imports), plus CLIs where **we are ahead** (our private work, FYI).                          |
| `pp-sync import <slug> …`   | Bring an upstream CLI (`library/<cat>/<slug>` + `cli-skills/pp-<slug>`) into private, commit, and **push to private only**.                                                                |
| `pp-sync build [name …]`    | (Re)build named CLIs, or all installed, from private.                                                                                                                                      |
| `pp-sync backlog [add "…"]` | Show or append the agent "we need a CLI for X" backlog (`~/.hermes/state/pp-cli-backlog.md`).                                                                                              |
| `pp-sync report`            | Print the last daily report.                                                                                                                                                               |
| `pp-sync doctor`            | Print resolved paths, remotes, and the push-safety status.                                                                                                                                 |

## Safety

- Every push targets `origin`, and `pp-sync` aborts unless `origin`'s push URL contains
  `cathrynlavery` (override with `PP_PRIVATE_OWNER`). It also refuses to run if an
  `upstream` push URL points at `mvanhorn`.
- `update`/`import` refuse to run when the repo has tracked local changes, and `update`
  only fast-forwards (`--ff-only`) — never a merge that could rewrite local work.
- The `google-ads` OAuth-refresh wrapper at `~/go/bin/google-ads-pp-cli` is preserved; only
  the `-real` binary it execs is rebuilt.

## Upstream-update flow (pulling public changes into private)

1. `pp-sync upstream` — see what's new / upstream-ahead.
2. Decide what to take (Cathryn's call).
3. `pp-sync import <slug>` — merges that CLI's tree from `upstream/main`, commits, pushes
   to private `main`. Open a PR against `cathrynlavery/printing-press-library` if you'd
   rather review before landing. **Never** push or PR to any `mvanhorn/*` remote.

## Agent backlog hook

When an agent finds itself wanting a CLI that doesn't exist, it runs:

```
PP_REQUESTER=<agent> pp-sync backlog add "<service> — <what for>"
```

The daily report surfaces the open backlog count so new-CLI ideas accumulate from real use.

## Environment overrides

`PP_LIBRARY_REPO`, `PP_GOBIN`, `PP_BINDIR`, `PP_SKILLS_DST`, `PP_STATE_DIR`,
`PP_PRIVATE_OWNER`, `PP_REQUESTER`.
