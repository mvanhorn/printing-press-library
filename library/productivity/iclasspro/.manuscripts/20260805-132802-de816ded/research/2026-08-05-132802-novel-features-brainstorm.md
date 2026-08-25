# iClassPro — Novel Features Brainstorm

> **Procedure note.** Phase 1.5 Step 1.5c.5 mandates a Task subagent. This session is
> configured with "Do not call the AgentTool unless the user requested it," which is
> user-scoped environment config and takes precedence over skill instructions. The
> three-pass procedure (customer model → 2× candidates → adversarial cut) was executed
> inline against the same inputs, rubric, and output contract. First print — no prior
> `research.json`, so Pass 2(d) reprint reconciliation is omitted. Brief has no
> `## User Vision` section (user selected "Let's go"), so source (e) is omitted.
> Brief has `## Codebase Intelligence`, so source (f) applies.

## Customer model

### Marisol — agency web developer (3–8 gym clients)

**Today:** Builds and maintains websites for gyms and swim schools running iClassPro. Her options are iClassPro's own iframe widget (doesn't match the client's brand, can't be styled) or hand-copying the schedule into the CMS. She wrote a scraper once and re-runs it when a client complains. She has the portal open in one tab and the CMS in another, eyeballing differences. She cannot answer: *did anything change on this client's schedule since I last updated the site?*

**Weekly ritual:** Before each client's weekly content refresh, check what classes and camps currently exist and whether the live site still reflects them.

**Frustration:** There is no change signal. A gym adds a camp on Tuesday, and she finds out on Friday when the owner asks why it isn't on the website.

### Dana — multi-location gym owner / front-desk manager

**Today:** Logs into the staff dashboard and clicks location by location, program by program, to see which classes are filling and which are dead. The portal shows a live openings count and nothing else. She cannot answer: *which classes have been sitting near-empty for three weeks?* — because there is no yesterday.

**Weekly ritual:** Monday capacity review — decide which classes to cancel, merge, or add a section to.

**Frustration:** Openings is a present-tense number. "Is this class trending toward full or toward cancellation?" is structurally unanswerable from the portal.

### Priya — parent chasing a specific class or camp

**Today:** Opens the portal, filters to her kid's level and day, sees "Full", and checks again tomorrow. For camps, registration opens on a set date and popular sessions sell out the same morning. She cannot answer: *tell me the moment a Saturday 10am Level 3 spot opens.*

**Weekly ritual:** Refreshing the portal. That is the whole ritual.

**Frustration:** Manual polling, plus camps whose `registrationStartDate` is still in the future are effectively invisible until the moment they're already competitive.

### Tomás — franchise / marketing ops across 10+ affiliated gyms

**Today:** Each gym is a separate iClassPro tenant with its own slug, its own program names, and its own typeIds. He maintains a hand-rolled Python script that walks every gym to build the brand's combined events calendar. (This persona is not hypothetical — `Jaymelynng/master-events-calendar` is exactly this, for exactly 10 gyms.) He cannot answer: *which of our gyms haven't posted their fall camps yet?*

**Weekly ritual:** Rebuild the combined calendar for the brand site and the newsletter.

**Frustration:** Nothing lines up across gyms. Program names differ, typeIds differ, and the one endpoint that looks like it gives typeIds (`camp-programs`) gives the wrong IDs.

## Candidates (pre-cut)

| # | Candidate | Command | Persona | Source | Inline verdict |
|---|---|---|---|---|---|
| 1 | Openings watcher | `watch` | Priya, Dana | (a) | keep — needs history, not a wrapper |
| 2 | Catalog drift | `drift` | Marisol, Tomás | (c) | keep — snapshot diff, local-only |
| 3 | Registration windows | `opens-soon` | Priya, Dana | (b) | keep — `registrationStartDate` is service-specific |
| 4 | Fill-rate trend | `fill-rate` | Dana | (c) | keep — openings history |
| 5 | Empty-class finder | `empty-classes` | Dana | (c) | **kill** — subset of #4 with a threshold |
| 6 | Cross-tenant compare | `compare` | Tomás | (c) | keep — multi-tenant local join |
| 7 | Calendar export (ICS) | `calendar` | Marisol, Tomás | (b) | keep — mechanical, high demand |
| 8 | Catalog lint | `lint` | Marisol, Tomás | (b) | keep — static quality, distinct from drift |
| 9 | Filter honesty explainer | `filters` | all | (f) | **reframe** — nobody runs this weekly; ship as behavior inside `classes list` |
| 10 | Tenant capability probe | `tenant` | Marisol, Tomás | (f) | keep — turns the silent-empty gate into a real answer |
| 11 | Instructor load | `instructor-load` | Dana | (c) | **kill** — join is name-vs-ID and fragile (see kill table) |
| 12 | Level ladder gaps | `levels gaps` | Dana | (c) | **kill** — niche |
| 13 | typeId ↔ programId map | `typeid-map` | Tomás | (b) | **kill as command** — ship as internal sync correctness |
| 14 | ProShop sale watch | `products watch` | Dana | (b) | **kill** — side surface, weak domain fit |
| 15 | Slug validator | `tenant check` | Tomás | (a) | **merge** into #10 |
| 16 | Family schedule fit | `fit` | Priya | (c) | **kill** — scope creep, needs a multi-child model |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Openings watcher | `watch --class 16010` | 10/10 | hand-code | Polls `/classes` + `/camps` on an interval and diffs `openings`/`hasOpenings`/`allowWaitlist` against the previous row in the local `openings_history` table | An entire community repo (`johnmarcovici/iclasspro-driver`, 954-line JWT cart automation) exists solely to win the race for a spot; portal has no notification of any kind | Use this command to be alerted when a spot frees up in a class or camp that is currently full. Do NOT use it to find registration that has not opened yet; use 'opens-soon' instead. |
| 2 | Catalog drift | `drift --since 7d` | 10/10 | hand-code | Compares the two most recent synced snapshots in local SQLite to report added / removed / retimed / repriced classes and camps, including `programIsDeleted` transitions | `master-events-calendar` docs call out `programIsDeleted` as "instant deletion detection!" in their unused-fields table; Marisol's stated frustration is the absence of any change signal | Use this command for what CHANGED between syncs. Do NOT use it to judge whether the catalog is well-formed right now; use 'lint' instead. |
| 3 | Registration windows | `opens-soon --days 14` | 10/10 | hand-code | Scans every synced class and camp for `registrationStartDate` in the future or `registrationEndDate` closing inside the window, and flags `campRegisterExpired` | `registrationStartDate` / `registrationEndDate` / `campRegisterExpired` are verified live fields; the API offers no filter on any of them and the portal surfaces none of them | Use this command to see registration that has not opened yet or is about to close. Do NOT use it to watch for a spot in something already open; use 'watch' instead. |
| 4 | Calendar export | `calendar --format ics` | 10/10 | hand-code | Renders synced camps and classes to RFC 5545 from `schedule[]` / `blocks[]` / `availableDates[]`, one VEVENT per session, with the portal deep link as URL | An entire community project (`Jaymelynng/master-events-calendar`, 10 gyms) exists to build exactly this by hand; iClassPro ships no feed of any kind | none |
| 5 | Tenant capability probe | `tenant --account nadoclub` | 9/10 | hand-code | Calls `locations`, `bookings/{loc}`, `classes`, `camps`, `appointments` and classifies each as open / sign-in-gated / plan-gated by matching the message strings the API returns alongside HTTP 200 | Verified live: `nadoclub` returns `200 {"data":[],"message":"Please sign in to see classes."}` and `scaq` returns `400 "Appointment subscription plan expired"` — both are invisible to status-code-only clients | none |
| 6 | Fill-rate trend | `fill-rate --program 57` | 8/10 | hand-code | Aggregates `openings` over time from `openings_history` per class and program to report direction and velocity of fill | Dana's Monday capacity review; the API is present-tense only, so no external tool can compute this | Use this command for how fast classes are filling over time. Do NOT use it for a point-in-time list of what changed; use 'drift' instead. |
| 7 | Cross-tenant compare | `compare --accounts a,b` | 8/10 | hand-code | Joins the same program/level across multiple synced tenants in local SQLite to compare schedule, age bands, and openings | Tomás's ritual is literally implemented by hand in `collectAllGyms.js`; each gym uses different program names and typeIds so no upstream call can do this | none |
| 8 | Catalog lint | `lint` | 8/10 | hand-code | Flags synced rows with missing descriptions or images, expired registration windows, deleted-but-listed programs, and zero-opening classes with waitlist disabled | `master-events-calendar`'s "Data Available But Not Currently Captured" table is a list of exactly these unused quality fields | Use this command to audit catalog quality as it stands now. Do NOT use it to see what changed since the last sync; use 'drift' instead. |

All eight are tagged `hand-code`. None is auto-emitted by the generator from the spec.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Empty-class finder | A threshold on the fill-rate output, not a separate command | `fill-rate` |
| Filter honesty explainer | Nobody runs a discovery command weekly; reframed into `classes list` behavior that warns when a filter is applied locally rather than server-side | (absorbed row) |
| Instructor load | `classes[].instructors` returns display **names** while `/instructors/{loc}/classes` returns **ids**, and the `instructors=` server filter is id-typed — the join is name-matching and would silently mis-attribute | `fill-rate` |
| Level ladder gaps | Niche; only meaningful for gyms with a formal progression ladder | `lint` |
| typeId ↔ programId map | Real trap, but it is sync-internal correctness, not a thing a human runs | `lint` |
| ProShop sale watch | Retail is a side surface of a class-management API; weak domain fit for every persona | `watch` |
| Slug validator | Same work as the tenant probe | `tenant` |
| Family schedule fit | Scope creep — requires a multi-child model and per-child constraints; not one command | `watch` |
