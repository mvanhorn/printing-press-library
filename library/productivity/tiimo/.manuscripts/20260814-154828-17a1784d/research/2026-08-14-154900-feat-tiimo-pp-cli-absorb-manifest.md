# Tiimo CLI — Absorb Manifest

## Ecosystem Search Result: Empty

Searches run (Phase 1.5a): Claude plugins, MCP servers/registries (lobehub,
mcpmarket, fastmcp), competing CLIs on GitHub, npm, PyPI, automation scripts,
SDK wrappers, `gh search repos tiimo`, `gh search code tiimoapp.com`.

**Result: zero prior art.** No Tiimo MCP server, CLI, SDK, wrapper, or plugin
exists. Every `tiimo`-named repo is an unrelated name collision. The only code
references are ADHD-app listicles.

Consequences:
1. There is nothing Tiimo-specific to absorb. The absorbed table below is
   inherited from adjacent planner CLIs (todoist-cli, taskwarrior, gcalcli,
   khal, timewarrior) — the table stakes any planner CLI must meet.
2. There is no MCP source to read for auth patterns (Step 1.5a.5 skipped) and no
   GitHub repo for DeepWiki analysis (Step 1.5a.6 skipped). Auth was recovered
   directly from the OIDC discovery document instead, which is stronger evidence.
3. Nearly all differentiation is transcendence, not parity.

## Absorbed (table stakes from adjacent planner CLIs)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List today's plan | todoist-cli `list`, khal `calendar` | `tiimo-pp-cli today` | Offline from local mirror, `--json`, grouped by TimeOfDay bucket |
| 2 | List an arbitrary range | gcalcli `agenda` | `tiimo-pp-cli agenda --from --to` | Offline, `--json`, `--select` |
| 3 | Show one item | taskwarrior `task <id>` | `(generated endpoint) activities get` | Typed endpoint, `--json` |
| 4 | Create an item | todoist-cli `add` | `tiimo-pp-cli add` | `--stdin`, `--dry-run`, natural duration parsing |
| 5 | Complete an item | taskwarrior `done` | `tiimo-pp-cli done` | Idempotent, `--dry-run`, typed exit codes |
| 6 | Reschedule an item | taskwarrior `modify` | `tiimo-pp-cli move` | `--dry-run`, relative time shifts |
| 7 | Delete an item | todoist-cli `delete` | `(generated endpoint) activities delete` | `--dry-run` |
| 8 | Search across items | taskwarrior filters | `tiimo-pp-cli search` | FTS5 over titles, notes, and checklist step text — offline |
| 9 | List tasks / to-dos | todoist-cli `list` | `tiimo-pp-cli todo list` | Offline from mirror, grouped by priority bucket, `--json` |
| 9a | Capture a to-do | todoist-cli `add`, taskwarrior `add` | `tiimo-pp-cli todo add` | `--stdin` brain-dump (one task per line), `--dry-run`, typed exit codes |
| 9b | Check off a to-do | taskwarrior `done` | `tiimo-pp-cli todo done` | Idempotent, `--dry-run` |
| 9c | Delete a to-do | todoist-cli `delete` | `tiimo-pp-cli todo rm` | `--dry-run` |
| 9d | Promote a to-do onto the timeline | — (Tiimo-specific) | `tiimo-pp-cli todo schedule` | Reads the to-do's `duration`, creates the matching activity — the app's own "drag to timeline" as one command |
| 10 | List tags/labels | todoist-cli `labels` | `(generated endpoint) tags` | `--json`. Promoted single-endpoint resource, so the path is `tags <profile_id>`, not `tags list`. |
| 11 | Recurring item support | taskwarrior `recur` | `(behavior in tiimo-pp-cli agenda) surfaces repetition rules and expands recurrence in range output` | Recurrence is read from the API's `repetition` object rather than re-derived |
| 12 | Local mirror + sync | timewarrior database | `tiimo-pp-cli sync` | SQLite mirror, date-window cursor, offline reads |
| 13 | Arbitrary query | taskwarrior reports | `tiimo-pp-cli sql` | Raw SELECT over your own planner history |
| 14 | Multi-profile support | — (Tiimo-specific) | `(generated endpoint) profiles list` | Tiimo supports up to 5 shared profiles |
| 15 | Calendar interchange | gcalcli / khal `.ics` | `tiimo-pp-cli export --format ics` | See transcendence #1 — this is the headline |

## Transcendence (only possible with our approach)

Scored /10 on user value. All rows ≥5 are shipping scope.

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|--------------------------|------------------|
| 1 | Data export | `export --format json\|csv\|ics` | hand-code | 10 | Tiimo **refuses** export as a stated design decision (nolt #528, #68). The API returns the data; only an external tool can write it out. | none |
| 2 | Plan-vs-actual drift | `drift --days 30` | hand-code | 9 | Requires joining `startTime`/`duration` against `startTimeActual`/`durationActual`/`durationPaused` across history. The app stores these and shows almost none of it. | Use this to find which activities chronically overrun or start late. Do NOT use it for a single day's schedule; use 'today' instead. |
| 3 | Checklist stall points | `stalls` | hand-code | 9 | Needs per-step `isChecked`/`checkedAt` across many days of nested checklists. Identifies the exact step where a routine breaks down. | Use this to find where multi-step routines fail. Do NOT use it for whole-activity completion; use 'adherence'. |
| 4 | Routine adherence | `adherence --weeks 4` | hand-code | 9 | Requires historical snapshots of recurring activities in SQLite. Answers "I complete Morning routine 62% of weekdays". | Use this for completion rates of recurring activities over time. Do NOT use it for step-level failure; use 'stalls'. |
| 5 | Subscribable calendar feed | `feed --out tiimo.ics` | hand-code | 9 | The exact compromise users asked for and were declined: a read-only calendar view of Tiimo activities. Generated from the local mirror. | none |
| 6 | Overlap detection | `overlaps --from --to` | hand-code | 8 | Explicitly requested on the feedback board, never shipped. Local interval join over the timeline. | none |
| 7 | Free-gap finder | `gaps --min 30m` | hand-code | 8 | Users asked for "visual representation of gaps in the day". Complement of the scheduled intervals per day. | none |
| 8 | Local backup / restore | `backup` / `restore --dry-run` | hand-code | 8 | Lock-in insurance for a product with no export. Full snapshot of your own data. | none |
| 9 | Rolling window view | `rolling --days 7` | hand-code | 7 | Reviewers called the fixed Mon–Sun view "a major oversight" because next week sneaks up. Trivial from a local store. | none |
| 10 | Day capacity | `capacity --from --to` | hand-code | 7 | Committed vs free minutes per TimeOfDay bucket. Needs duration summation the app doesn't expose. | none |

**Hand-code count: 10.** Zero of these come free from the spec — every row is
hand-written Go on top of the generated data layer.

## Stubs

None. No row ships as a stub.

## Deferred (scored <5, not shipping)

- Mood/energy correlation — no endpoint found during discovery (see report §8)
- Focus-timer session history — no endpoint found; timer state appears folded
  into `durationActual`/`durationPaused`
- AI Co-planner passthrough (`ai.tiimoapp.com`) — surface not probed; would need
  its own discovery pass

## Risk Register

- **Auth client_id is unresolved.** OIDC supports `authorization_code`+PKCE and
  `offline_access`, but a public `client_id` with a registered loopback redirect
  has not been confirmed. Fallbacks, in order: (a) discovered public client_id,
  (b) `password` grant prompted at runtime by the CLI, (c) operator-supplied
  token via `TIIMO_TOKEN`. The CLI will ship (c) unconditionally so it always
  works, with (a)/(b) layered on if resolvable.
- **Undocumented API.** No stability contract; endpoints may change without
  notice. Automating it likely sits outside Tiimo's ToS.
- **Activity and to-do write paths are both verified.** Tag and checklist-item
  writes remain inferred, not proven.
- **The two resources use different update conventions.** Activities update via
  `PUT /activities/{activityId}`; to-do tasks update via `PUT /todo-tasks`
  (collection) — item-level PUT returns 405. Create returns 201 for activities
  and 200 for to-dos; delete returns 204 and 200 respectively. The generated
  client must not assume one convention across both.
- **`routines` item schema unverified** — the test account returned an empty array.
