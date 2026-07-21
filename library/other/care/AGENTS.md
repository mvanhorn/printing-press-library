# Care Printed CLI Agent Guide

This directory is a generated `care-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
care-pp-cli doctor --json
care-pp-cli agent-context --pretty
```

Use runtime discovery instead of relying on a copied command list:

```bash
care-pp-cli which "<capability>" --json
care-pp-cli <command> --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, no color, and confirmation-safe scripting:

```bash
care-pp-cli <command> --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
care-pp-cli <command> --help
care-pp-cli <command> --dry-run --agent
```

Use `--yes --no-input` only after the target, arguments, and side effects are clear.

## Self-Learning Loop

This CLI ships a self-capturing teach/recall loop backed by the local SQLite store. The CLI journals every invocation, derives `flag_alias` candidates from failed-flag + corrected-retry pairs, and synthesizes a playbook candidate when a family is taught without one - no manual failure bookkeeping. The agent's role is judgment:

1. On a new user question, call `care-pp-cli recall "<question>" --agent` FIRST. If `found=true` and the top result has `entity_match == "exact"` and `confidence >= 2`, skip discovery and go straight to the live fetch for the returned resource IDs. If the store is cold (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of the session.
2. When the envelope carries a `candidates` section (warning `candidates_present`), candidates are try-then-confirm, never facts: follow each candidate's two-step `next_action` verbatim (trial command first, then `learnings confirm <id>` only after the trial verified the behavior), and reject wrong ones with `learnings reject <id>`. Never re-teach something recall surfaced as a candidate; confirm or reject it instead.
3. After answering, always fire `care-pp-cli teach --query "<question>" --resource <id> --resource-type <type> &` in the background - teaching is unconditional and is the anchor that triggers playbook synthesis. Teach the structural question with identifiers stripped (no names, emails, phone numbers, account ids); the CLI warns on obvious PII shapes but does not block.
4. Use `learnings list` to inspect taught rows, `learnings forget "<question>"` to undo a bad teach, `learnings candidates` for the full open candidate set, and `learnings stats` for the loop's local metrics. `teach-pattern` and `teach-lookup` install manual generalization rules when one teach should cover a whole family (e.g. one country alias unlocks every per-country query).
5. If `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and keep the rest of the flow.

Annotations: `recall`, `learnings list`, `learnings candidates`, and `learnings stats` carry `mcp:read-only=true`; `teach`, `teach-playbook`, `playbook amend`, `learnings confirm`, `teach-pattern`, and `teach-lookup` carry `mcp:local-write=true` (writes land only in the CLI's own local store); `learnings forget` and `learnings reject` keep honest may-write/destructive defaults.

### Success definition

Measurement is local-only: the `learn_events` table and `learnings stats`; nothing leaves this machine. Judge the loop on recall hit rate and teach-to-reuse at a minimum denominator of 50+ recall events. Near-zero rates at that denominator mean the loop is not earning its keep for this CLI - surface that in retros. An empty or thin events table means insufficient adoption, not failure.

The store's schema stamp is one-way: once this binary opens the database, an older binary refuses it (README.md carries the upgrade note).

Disable the loop with `--no-learn` per-invocation or `CARE_NO_LEARN=true` for the whole session - useful for deterministic agent flows that don't want a learning row to silently change subsequent query results.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Release Ledger

`CHANGELOG.md` and `.printing-press-release.json` are the public library's per-CLI release ledger. Fresh prints carry an unstamped runtime version such as `0.0.0-dev`; the final `YYYY.M.N` CLI release version is assigned only after a publish PR merges in `mvanhorn/printing-press-library`. Do not hand-bump those files or edit `var version = ...` for release bookkeeping; preserve existing ledger files on reprint and let the library workflow stamp the next release.

## Care.com design deviations (read before regen)

care.com has no public API. Everything runs over a single authenticated
GraphQL BFF (`POST https://www.care.com/api/graphql`, cookie auth). The
generated `internal/cli/*.go` commands are the raw BFF operations; a
hand-built friendly layer sits on top (`find`, `recommend`, `caregiver`,
`sync`, `outreach`, `messages`, `favorite`, `job`). Two deviations from the
standard Printing Press pattern were required — keep both across regen:

**1. Auth: persistent-profile chromedp instead of press-auth.** care.com login
is a passwordless magic-link, which breaks press-auth's controlled-window
completion detection, and care.com enforces a single active session, so any
login elsewhere invalidates a captured token. Instead, `care-pp-cli auth
profile-login` does a one-time headful login into a persistent Chrome profile
under `~/.care-pp-cli` (stays valid for weeks); `auth refresh` re-extracts
cookies headlessly with no re-login. The client sends them as a `Cookie`
header (`CARE_SESSION_COOKIE` overrides). macOS-focused (chromedp). Files:
`internal/cli/care_auth.go`, `internal/client/care_cookie.go`.

**2. Messages read via Stream Chat, not the GraphQL BFF.** care.com messaging
runs on Stream Chat (getstream.io). The `/app/messages` page embeds a per-user
`streamToken` (JWT) + `streamApiKey`; the CLI fetches those with the session
cookie, then reads the conversation list and message bodies from Stream's REST
API. Sends do **not** go direct-to-Stream — they route through care.com's
moderated `sendCareNeedMessage` mutation so trust-and-safety scanning and
care.com-side record-keeping stay intact. Files: `internal/cli/care_messages.go`.

**Safety model (locked).** The hand-built write commands that contact people or
change account relationships — `outreach send`, `messages reply`, and
`favorite`/`unfavorite` — are draft-and-confirm: dry-run by default, `--confirm`
required per invocation, no blast mode, scoped to the user's own authenticated
account. User-facing draft text is ASCII-only (smart punctuation mojibakes when
pasted into care.com / Mac apps). Command `Example:` strings never include
`--confirm` — a send example would fire for real when copy-pasted or exercised
by a live dogfood matrix.

The raw generated GraphQL passthrough commands under the hidden `care-jobs`
group (`application-interest`, `child-care-one-time-update`) are standard
Printing Press generated mutations: they are live-by-default like every other
generated CLI, are not advertised in `--help` (the group is `Hidden`), and
support the global `--dry-run` flag to preview the request without sending. They
are not part of the hand-built draft-and-confirm layer above; use `--dry-run`
before running one.

**Intentionally not shipped.** The `JobApplications` applicant-list is omitted:
care.com returns empty `edges` even in a real logged-in browser (a stateful
server-side gate), so it can't be replayed reliably from a stateless cookie
session. The Stream messages inbox covers applicant review instead.

**Live testing.** Because auth is a browser session, the session-less publish/CI
harness cannot run the Phase 5 live matrix (`phase5-skip.json` /
`cookie-auth-no-harness-session`, matching merged `jimmy-johns`/`janeapp`). The
commands were verified live during development — see
`.manuscripts/<run>/proofs/manual-live-verification.md`.

Hand-built (non-generated) files, for regen durability: `care_search.go`,
`care_recommend.go`, `care_caregiver.go`, `care_sync.go`, `care_outreach.go`,
`care_messages.go`, `care_favorite.go`, `care_jobs_helper.go`, `care_auth.go`,
`care_queries.go`, `care_mutations.go`, and `internal/client/care_cookie.go`.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`) so a regen carries the intent forward instead of silently dropping it.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the public library's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.
