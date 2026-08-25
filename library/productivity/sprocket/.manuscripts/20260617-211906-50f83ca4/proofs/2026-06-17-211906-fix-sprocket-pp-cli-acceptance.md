# Sprocket Sports CLI — Acceptance Report (Phase 5 Full Live Dogfood)

## Level: Full Dogfood (live, against the real JFC tenant)
- Auth: Bearer token pulled read-only from the user's logged-in Chrome session,
  used in-process only, never written to any artifact, shredded after the run.
- Gate: **PASS** — 122/122 matrix tests passed, 0 failures.
- Live novel-feature sample probe: **9/9 (100%)**.

## Tests: 122/122 passed
Full mechanical matrix across every leaf command: help, happy-path, JSON-fidelity,
error-path. Plus behavioral eyeballing of the flagship commands against real data.

## Bugs found and fixed during live dogfood
Live testing caught what mock verification could not:

1. **Calendar returned only club-wide events (flagship-breaking).** The shipped
   `fetchCalendar` POSTed a minimal `{start,end}` body, which returns only club-wide
   events — the user's actual team schedule (training, games) was missing, so
   `next`/`week`/`agenda`/`away`/`conflicts`/`ical`/`since` all returned near-empty.
   Re-sniffed the live calendar XHR (the app uses XMLHttpRequest, not fetch) and
   found it scopes by `teamID`. Fix: `fetchCalendar` now looks up the user's team
   IDs via `/api/club-users/player-teams` and queries a team-scoped layer
   (`teamID:[...]`, all event types incl. practices) merged with a club-wide layer
   (`clubEventTypeID:[1,5,6]`). Verified: `agenda --days 45` went from 0 → 12 events;
   `next` now finds the upcoming training. This also realizes the "merge across all
   your players' teams" value prop (all team IDs are queried together).
2. **`ical --json` emitted non-JSON** (iCalendar text), failing JSON-fidelity. Fix:
   emit `{"format":"ics","content":...}` under `--json`/`--agent`; raw `.ics`
   otherwise (so `ical > jfc.ics` still works).
3. **`tail <bad-resource>` hung forever** (framework command polled an invalid path).
   Fix: validate against `tailKnownResources()`, exit 2 on unknown. NOTE: this is a
   generator-emitted command; the path-convention mismatch and missing validation
   are a **Printing Press framework gap** → retro candidate.
4. **`schedule list` happy-path** sent placeholder dates the API rejected (400). Fix:
   `pp:happy-args` supplies valid date fixtures for the probe; the command works with
   real dates.

## Behavioral spot-checks (live, PII-redacted)
- `next` → returns the next upcoming team training, rendered in correct **local** time
  (event `20:00Z` → 4:00 PM local) — confirms the timezone fix.
- `agenda --days 45` → 12 merged events across the user's team(s).
- `owed` → $0.00 outstanding (consistent: the account has no overdue invoices and no
  open registration balances; endpoints returned empty arrays).
- `teams list` → full club team directory (120 teams).
- `away` → correctly filters to away games only.
- `deadlines` → open programs sorted by close date.

## Printing Press issues for retro
- Framework `tail` command: assumes `/<resource>` REST paths (mismatched for APIs
  with `/api/...` conventions) and does not validate the resource (hangs on a typo).
- (Minor) Generated POST read-commands wrap responses in an `action/data/...`
  envelope while hand-written novel commands emit bare arrays — output-shape
  inconsistency worth a generator note.

## Gate: PASS → promote to library.
