# Tiimo CLI Brief

## API Identity
- **Domain:** Visual daily planning / executive-function support. Tiimo is a
  timeline-first planner built for ADHD, autistic, and otherwise neurodivergent
  users. ~500k users, 2.5M+ downloads, 4.6/5.
- **Users:** People who need *visible* time. The product's core bet is that a
  color-coded, icon-rich timeline plus a visual countdown beats a flat task list
  for anyone with time-blindness or task-initiation difficulty.
- **Data profile:** Small per-user, high personal gravity, heavily time-indexed.
  Days of scheduled activities, each with start/duration/icon/color, optional
  checklists, plus standalone to-dos, reusable routines, focus-timer sessions,
  and mood/energy entries. All of it is *personal* data the user already owns
  but currently cannot get out.

## Source & Access Model
- **No public API.** Repeatedly requested on the official feedback board and
  archived without commitment (nolt.io/704, nolt.io/105). No OpenAPI, no
  developer docs, no SDK.
- **No prior art.** GitHub repo search and code search for `tiimo` /
  `tiimoapp.com` return zero unofficial clients, wrappers, CLIs, or MCP servers.
  Every `tiimo`-named repo is an unrelated name collision. The only code hits
  are ADHD-app listicles that mention the product.
- **Surface:** `webapp.tiimoapp.com` is a Next.js app on Vercel. The landing
  shell exposes no backend host or API path — those live in the JS chunks.
  Backend discovery requires traffic capture.
- **Auth:** Browser session. The web app is a Pro/trial-gated feature behind
  `/sign-in`. No API-key convention exists; environment scan found nothing.

## Reachability Gate (Phase 1.9)
- **Decision: PASS.** Settled by live evidence, not inference: ~40 authenticated
  requests against `api.tiimoapp.com` during Phase 1.7 returned 200/201/204,
  including a full create-update-delete cycle on two resources.
- `traffic-analysis.json` reports `reachability.mode: standard_http`. No
  Cloudflare, WAF, DataDome, CAPTCHA, or clearance requirement at any point.
- The printed CLI ships a plain Go HTTP client with a bearer token. No Surf, no
  browser transport, no `auth login --chrome`.
- Tier/permission hints from 4xx body: the two 400s encountered were RFC 7807
  validation errors naming missing query parameters (`fromDate`, `from`), not
  entitlement gates. The web app itself is Pro-gated, but the API accepted every
  read and write attempted on this account.
- Probe-safe endpoint used: not applicable — no spec declared one. Write probing
  was done with explicit user approval on a self-created, self-deleted fixture.

## Reachability Risk (pre-capture assessment, retained for the record)
- **Unknown, pending capture.** No 403/blocked evidence exists because nobody
  has published an attempt. Rated *unknown* rather than *low*: an unprobed
  target is not a safe one.
- Not a bot-protection story so far — the landing page returned a clean
  `200 text/html` with no challenge headers.
- Real risk is **auth shape**, not blocking: if the backend uses short-lived
  bearer tokens minted in page context rather than a replayable cookie, the
  printed CLI needs a token-import path rather than cookie replay. Capture
  settles this.
- Tier/permission hints from 4xx body: none captured yet.
- Probe-safe endpoint used: none — no spec exists to declare one.

## Top Workflows
1. **See today.** Pull the day's timeline — what is scheduled, when, in order.
   This is the single most-used view in the product.
2. **Capture into the plan.** Get a task or activity onto the timeline fast,
   before the thought is lost. Quick capture is the whole ballgame for the
   target user.
3. **Run a routine.** Instantiate a saved routine/checklist onto a day and work
   its steps.
4. **Focus on one thing.** Start a timer bound to a specific activity, and later
   see what was actually completed versus planned.
5. **Look back.** Review completion, timer sessions, and mood/energy over a
   window to notice patterns — the product surfaces some of this as "insights"
   but does not let you query or export it.

## Table Stakes
No Tiimo-specific tool exists to absorb from, so table stakes are inherited from
adjacent planner CLIs (todoist-cli, taskwarrior, gcalcli, khal, timewarrior):
- List with date/range filters (`today`, `tomorrow`, `--from/--to`)
- Add/complete/reschedule/delete an item
- Quick capture from stdin and from a single positional string
- Search across items
- `--json` everywhere, field selection, exit codes, `--dry-run`
- Recurring item support
- Calendar interchange (`.ics` in/out)
- Local mirror so reads work offline and fast

## Data Layer
- **Primary entities:** activities/events (the timeline entries), tasks/to-dos,
  checklists (child items of an activity), routines (reusable templates),
  timer sessions, mood/energy entries, categories/icons/colors.
- **Sync cursor:** date-window based (`from`/`to` day range), plus per-entity
  `updated_at` if the backend exposes one.
- **FTS/search:** title + notes + checklist step text across activities, tasks,
  and routines.

## Why This CLI Should Exist
Tiimo **deliberately refuses data export and two-way calendar sync.** This is
not a roadmap gap — the team stated it as a design decision, and it is one of
the most-upvoted complaints on their own feedback board (nolt.io/528,
nolt.io/68). Users have explicitly asked for a read-only calendar export as a
compromise and been declined.

That refusal is the entire product opportunity:

| User asks for | Vendor position | What a CLI can do |
|---|---|---|
| Export my data | Declined by design | `export --format json/csv/ics` |
| Two-way calendar sync | Declined; import only | Generate an `.ics` feed from the local mirror |
| Public API for automation | Archived, no commitment | The CLI *is* the automation surface |
| Rolling 7-day view | Not shipped | Trivial from a local store |
| Overlap/gap detection | Requested, not shipped | Local query over the timeline |
| Deeper insights than the app shows | Intentionally simple | Arbitrary SQL over your own history |

Nobody else has built this. The vendor has said they will not. The data is the
user's own.

## Product Thesis
- **Name:** `tiimo-pp-cli`
- **Thesis:** *Your Tiimo plan — finally exportable, queryable, and yours.*
  A local SQLite mirror of your own Tiimo data plus the export, calendar-feed,
  and analysis surfaces the vendor has explicitly declined to build.

## Build Priorities
1. **Data layer + sync.** Mirror activities, tasks, checklists, routines, and
   mood entries into SQLite over a date window. Everything else compounds on it.
2. **Read the plan.** `today`, `agenda --from/--to`, `search` — offline, fast,
   `--json`.
3. **Export.** `export --format json|csv|ics`. This is the headline feature and
   the direct answer to the #1 vendor refusal.
4. **Write back.** Add/complete/reschedule an activity or task, if capture shows
   safely replayable mutation endpoints.
5. **Transcendence.** Gap/overlap detection, plan-vs-actual drift, rolling
   windows, routine adherence over time — all local joins the app cannot do.

## Open Questions for Capture (Phase 1.7)
- Backend host and protocol (REST vs GraphQL vs BFF envelope)
- Auth shape: replayable cookie vs page-context bearer token
- Whether write endpoints are safely replayable
- Pagination/date-window parameters on the timeline read
- Whether mood/insights data is exposed to the web client at all
