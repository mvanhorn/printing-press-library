# GreatClips CLI Brief

## API Identity
- Domain: walk-in haircut salon waitlist. Customer-facing wait times and check-in.
- Users: parents organizing family haircuts; commuters who want to walk in just as their seat is up; impatient people watching the wait clock; agents booking on behalf of busy people.
- Data profile: ~4,500 salons across US/Canada; per-salon wait time (refreshes every ~30s); per-customer profile with favorites + recent visits; ephemeral active check-ins.

## Reachability Risk
**Low.** Auth0 OAuth tenant for customers (no anti-bot). Browser-sniff with logged-in session captured all endpoints cleanly. No 403/429 evidence, no CAPTCHA, no Cloudflare challenge. Token-based access via Bearer header works in plain HTTP.

## Top Workflows
1. **"How long is the wait at my favorite salon?"** -- single fast read. User looks at the clock once, decides whether to leave the house.
2. **"Check me + my kids in"** -- party check-in with 1-5 people on one slot. The headline mutation.
3. **"Watch the wait until it drops"** -- poll until estimated wait under N minutes, then notify or auto-check-in.
4. **"Compare all salons within X miles"** -- which is shortest right now? Includes drive distance from a reference address.
5. **"Where am I in line?"** -- active check-in status; "should I leave the house yet?"
6. **"Schedule-shop the family weekend"** -- next 14 days of salon hours including holiday closures.

## Table Stakes (no competitor; this is blue-water)
- List wait times by zip / city / coords / favorited
- Single-salon detail with hours, phone, "next to" cross-street
- Salon-hours forecast 14 days out
- Profile read (name, phone, favorites)
- Active check-in status + cancel
- Submit a check-in with party size 1-5

## Data Layer
- **Primary entities:** salon (~4,500 rows), salon_hours (forecast rows), wait_snapshot (timeseries), check_in (active + historical), profile (one row).
- **Sync cursor:** salon table syncs by geographic search outwards; wait snapshots are timeseries with `salon_number, captured_at, wait_minutes, state`.
- **FTS/search:** salon by name + city + cross-street ("Next to Einstein, across from QFC" is queryable text the official site doesn't surface).

## Codebase Intelligence
- Source: live browser-sniff of `app.greatclips.com` (Next.js SPA) + JS bundle grep for path constants.
- Auth: Auth0 PKCE flow at `cid.greatclips.com`. Single Bearer JWT used for `webservices.greatclips.com/*` AND `www.stylewaretouch.net/*`. localStorage key: `auth0.eq2A3lIn48Afym7azte124bPd7iSoaIZ.is.authenticated`.
- Data model: salon metadata and wait state live in different services. The CLI MUST join them on `salonNumber` to produce the user-facing rows everyone wants.
- Rate limiting: not observed in capture. Stylewaretouch is shared infrastructure across all ICS Net Check-In customers (not just GreatClips) so per-key limits likely exist. Implement per-source `cliutil.AdaptiveLimiter`.
- Architecture: two-host pattern (vendor brand + third-party operations) is the load-bearing insight. Without it, the agent would write one client and miss half the surface.

## User Vision
- "How long is great clips wait for my closest store, or favorited (mine is Mercer Island)"
  -> map directly to `greatclips wait` with `--favorite` (default) and `--near "98040"` modes.
- "Great, add me to the list + 3 kids" type thing.
  -> `greatclips checkin --party 4` (or `--me-plus-kids 3`). The killer flow.
- User confirmed logged in during research. Auth shape is "browser cookie + Auth0 token from logged-in Chrome session"; capture the tokens once and replay.

## Source Priority
Single source (vendor brand wraps third-party ops); no priority gate needed.

## Product Thesis
- **Name:** `greatclips-pp-cli`
- **Why it should exist:** no CLI, no MCP server, no Claude skill exists for GreatClips. The Online Check-In experience is locked inside a Next.js SPA that hides the data. Agents and CLI users want one command to answer "should I leave the house?" and another to put a family of four in line. Both turn 30-second decisions into one keystroke and unlock new workflows (polling, comparison, scheduling) that the SPA can't express.

## Build Priorities
1. **Data layer + auth.** OAuth login via `auth login --chrome` (capture tokens from logged-in Chrome via cookie extraction or a one-time device-flow) + SQLite store with `salons`, `wait_snapshots`, `check_ins`, `salon_hours`, `profile`.
2. **Read commands.** `wait`, `salons near`, `salon <num>`, `hours <num>`, `profile`.
3. **Mutation.** `checkin <salon> --party N`, `status`, `cancel`.
4. **Transcendence.** `watch` (poll until wait under threshold), `compare` (rank nearby), `family-plan` (party size N with optional auto check-in when wait drops), `history` (where have I been), `next-open` (which nearby salon is open earliest on day D), `drift` (was today's wait higher or lower than this same hour-of-week historically).

## Reachable from a plain Go HTTP client?
Yes. Auth0 token + JSON body + JSON response. No browser sidecar required. The printed CLI ships standalone with a one-time `auth login --chrome` flow to copy the token out of a logged-in Chrome session; afterwards it is pure HTTP.
