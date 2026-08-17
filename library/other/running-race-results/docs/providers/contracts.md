# Provider API Contracts

Captured request/response shapes per provider. Each adapter (Phase 2) is built
from the matching `testdata/fixtures/<provider>/` fixture + the note below.

> Status: Phase 0 discovery. Fill from real captures only — never fabricate.

## NYRR

### Request

```
POST https://rmsprodapi.nyrr.org/api/v2/runners/finishers-filter
Content-Type: application/json
```

Body (`testdata/fixtures/nyrr/request.json`):
```json
{
  "eventCode": "26MINI",
  "searchString": "Smith",
  "gender": null,
  "ageFrom": null,
  "ageTo": null,
  "sortColumn": "overallTime",
  "sortDescending": false,
  "pageIndex": 1,
  "pageSize": 20
}
```

- Event code comes from the URL: `results.nyrr.org/races/{eventCode}/results`
- No auth token needed — open API with standard browser headers
- Pagination: `pageIndex` (1-based), `pageSize`
- Bib lookup: set `searchString` to bib number (numeric string)

### Response → Result mapping

Response (`testdata/fixtures/nyrr/search.json`):
```json
{
  "totalItems": 40,
  "items": [
    {
      "runnerId": 52475309,
      "firstName": "Rachel",
      "lastName": "Smith",
      "bib": "19",
      "age": 34,
      "gender": "W",
      "city": "Flagstaff",
      "countryCode": "USA",
      "stateProvince": "AZ",
      "overallPlace": 20,
      "overallTime": "0:33:48",
      "pace": "05:27",
      "genderPlace": 20,
      "ageGradeTime": "33:26",
      "ageGradePlace": 23,
      "ageGradePercent": 90.76,
      "racesCount": 7
    }
  ]
}
```

Field mapping:
- `bib` → BIB
- `firstName` + `lastName` → name
- `overallPlace` → overall rank
- `genderPlace` → gender rank
- `overallTime` → finish time (gun, format `H:MM:SS`)
- `pace` → pace per mile (format `MM:SS`)
- `age`, `gender` ("W"/"M"), `city`, `countryCode`, `stateProvince` → athlete metadata
- `ageGradePercent` → age grade %

### Runner history (cross-event)

NYRR **does** expose a cross-event runner history. The runner profile page at
`results.nyrr.org/runner/{runnerId}/races` is powered by three endpoints, all
keyed by the internal `runnerId` (integer from `runners/search` — **not**
`externalRunnerId`).

**Step 1 — name → runnerId disambiguation:**

```
POST https://rmsprodapi.nyrr.org/api/v2/runners/search
Content-Type: application/json

{"searchString":"Sample Runner","pageIndex":1,"pageSize":51,"sortColumn":null,"sortDescending":false}
```

Response (`testdata/fixtures/nyrr/runner-search.json`): `{totalItems, items:[{runnerId, externalRunnerId, firstName, lastName, age, gender, city, stateProvince, countryCode, bib, racesCount, eventCode, ...}]}`.
Each item is the runner's most-recent event entry. `racesCount` is the total NYRR race count for that person.
Runner is identified in subsequent calls by `runnerId` (integer, NYRR-internal).

**Step 2 — fetch race history:**

```
POST https://rmsprodapi.nyrr.org/api/v2/runners/races
Content-Type: application/json

{
  "runnerId": "2969961",
  "searchString": null,
  "year": null,
  "distance": null,
  "teamCode": null,
  "overallPlaceFrom": null, "overallPlaceTo": null,
  "paceFrom": null, "paceTo": null,
  "overallTimeFrom": null, "overallTimeTo": null,
  "gunTimeFrom": null, "gunTimeTo": null,
  "ageGradedTimeFrom": null, "ageGradedTimeTo": null,
  "ageGradedPlaceFrom": null, "ageGradedPlaceTo": null,
  "ageGradedPerformanceFrom": null, "ageGradedPerformanceTo": null,
  "pageIndex": 1,
  "pageSize": 51,
  "sortColumn": "EventDate",
  "sortDescending": true
}
```

Response (`testdata/fixtures/nyrr/runner-history.json`):
```json
{
  "totalItems": 34,
  "items": [
    {
      "runnerId": "2969961",
      "bib": "7629",
      "eventCode": "a51113",
      "eventName": "NYRR Cross Country Champs.",
      "venue": "Van Cortlandt Park, Bronx, NYC",
      "distanceName": "5 kilometers",
      "startDateTime": "2005-11-13T10:00:00",
      "actualTime": "0:21:40",
      "actualPace": "06:59"
    }
  ]
}
```

Field mapping:
- `eventName` → race name
- `startDateTime` → race date (ISO 8601)
- `distanceName` → distance (free-text, e.g. `"Half-Marathon"`, `"10 kilometers"`)
- `actualTime` → finish time (format `H:MM:SS`)
- `actualPace` → pace per mile (format `MM:SS`)
- `bib` → bib number
- `eventCode` → NYRR event code (use to link back to per-event results via `runners/finishers-filter`)
- `venue` → race location
- `runnerId` → varies per row — note: **row-level `runnerId` is a per-event result ID**, not the
  athlete ID; the athlete ID (`"2969961"` in the request body) stays fixed. Use the request body
  value, not per-row values, to identify the athlete.

**Notes:**
- No auth needed — standard browser headers only (`Referer: https://results.nyrr.org/`)
- Pagination: `pageIndex` (1-based), `pageSize`
- Optional filters: `year` (string e.g. `"2005"`), `distance` (e.g. `"HALF"`, `"10K"`) from the
  `runners/racesFilter` helper endpoint which returns available year/distance facets for a given runner
- Runner is keyed by `runnerId` (integer from `runners/search`); `externalRunnerId` is a secondary
  ID present on some records but not required for this call

---

## Mika

Site: `https://berlin.r.mikatiming.com/` (Berlin Marathon 2025)

### Request

**Search (POST — returns full HTML page with embedded runner list):**
```
POST https://berlin.r.mikatiming.com/?event=BML_HCH3C0OH2F2&pid=search
Content-Type: application/x-www-form-urlencoded

search[name]=Runner&search[start_no]=&search[nation]=&Search=Search
```

- `event` param: event code in URL (e.g., `BML_HCH3C0OH2F2` for Berlin Marathon 2025)
- `pid=search`: triggers search results page
- Returns HTML (not JSON); runner list in `<ul class="list-group list-group-multicolumn">`

**Detail page (GET):**
```
GET https://berlin.r.mikatiming.com/?content=detail&fpid=search&pid=search&idp={runner_id}&lang=EN_CAP&event={event_code}
```

- `idp`: unique runner ID extracted from search result links
- Returns full HTML detail page with split table

**Autocomplete AJAX (GET — returns JSON, NOT used for structured results):**
```
GET https://berlin.r.mikatiming.com/index.php?content=ajax2&func=getSearchResult&event={event}&lang=EN&search[name]=Runner
```

### Response → Result mapping

Search HTML (`testdata/fixtures/mika/search.html`):
- Runner rows in `<ul class="list-group list-group-multicolumn">`
- Each runner link: `?content=detail&fpid=search&pid=search&idp={runner_id}&lang=EN_CAP&event={event_code}`
- Extract `idp` parameter for detail lookup

Detail HTML (`testdata/fixtures/mika/detail.html`), CSS class selectors:
- `td.f-__fullname` → full name
- `td.f-start_no_text` → bib
- `td.f-time_finish_netto` → chip time (net)
- `td.f-time_finish_brutto` → gun time
- `td.f-place_nosex` → **overall** place ("Place (Total)")
- `td.f-place_all` → **gender** place ("Place (M/W/D)")  ⚠️ class name is misleading; verified against the captured detail page
- `td.f-place_age` → age group place
- Split times at checkpoints: `td.f-time_15000`, `td.f-time_half`, `td.f-time_30000`, `td.f-time_40000`, etc.

Example (Sample Runner, bib 73664, Berlin Marathon 2025):
- Net time: 04:21:19, Gun time: 04:29:35
- Overall: 24556, Gender: 17968, Age group: 3322

---

## RaceResult

Site: `https://my.raceresult.com/{eventId}/results`

Event discovery: `GET /RREvents/list?group=0&user=0&userID=0&geoLocation=IP&lang=en&modes=topResults` returns array of event objects with `id`, `name`, `dateFrom`, `location`, `countryCode`.

### Request

**Config (required first — returns key + list names):**
```
GET https://my.raceresult.com/{eventId}/results/config?lang=en
```
Returns: `{ key, contests, splits, eventname, EventOver, server, Tab: { Config: { Lists: [...] } } }`.
- `server`: hostname for subsequent data calls (e.g., `my-us-1.raceresult.com`)
- `Tab.Config.Lists[].Name`: list name to pass to the list endpoint

**Results list:**
```
GET https://{server}/{eventId}/results/list?key={key}&listname={encodedListName}&page=results&contest={contestId}&r=leaders&l=10&fav=&openedGroups=%7B%7D&term=
```
- `listname`: URL-encoded list name from config (e.g., `Ergebnislisten%7CInternet-einzel%20-%20Frauen`)
- `contest`: contest ID (from `config.contests` keys, typically `"1"`)
- `term`: name/bib search filter (empty for full list)
- `page`: `"results"` for initial load; numeric for pagination

**Participant detail:**
```
GET https://{server}/{eventId}/{detailsTabName}/view?lang=en&noVisitor=1&mid=0&standalone=false&pid={pid}
```
- `detailsTabName`: from `config.Tab.Config.StandardDetails` (e.g., `"details0"`)
- `pid`: participant ID (= BIB, same as first two DataFields columns in list response)

Captured event: 17. REWE Team Challenge Dresden, 2026-06-17
- `eventId`: `390537`
- `key`: `93941475da0e781fdf01c051062b7423`
- `server`: `my-us-1.raceresult.com`

Fixtures: `testdata/fixtures/raceresult/config.json`, `testdata/fixtures/raceresult/results.json`, `testdata/fixtures/raceresult/detail.json`

### Response → Result mapping

List response: `{ DataFields: [...], data: { "#group_name": [[row], [row], ...] }, mid }`.
- `DataFields` names columns; `data` rows are parallel arrays
- Typical columns: `BIB`, `ID` (= pid), `RANK2p` (place label e.g. "1."), `AnzeigeName` (display name), `CLUB`, `Organisation`, `TIME1` (finish time)
- `ID` column (index 1) = `pid` for detail lookup

Field mapping from list row:
- `row[0]` (BIB) → BIB
- `row[1]` (ID) → pid for detail lookup
- `row[2]` (RANK2p) → rank display ("1.", "2.", ...)
- `row[3]` (AnzeigeName) → full name
- `row[4]` (CLUB) → team name
- `row[6]` (TIME1) → finish time (format `H:MM:SS`)

Detail response: `{ Data: { SplitsAndLegs: { Splits, Legs }, Fields, Certificates, Photos }, PID, MID, Server }`.
- `Data.Fields`: null in this event (splits not exposed); when present, contains split times
- `Data.Photos`: array of photo URLs (Sportograf etc.)

---

## Athlinks

Site: `https://www.athlinks.com/event/{masterEventId}/results/Event/{eventId}/Course/{raceId}/Bib/{bib}`

Backend: `reignite-api.athlinks.com` (new events) + `results.athlinks.com` (legacy events).

Auth: **optional.** The athlete (`alaska.athlinks.com/athletes/api/*`), reignite `results/search`, and per-athlete `result` detail endpoints are publicly accessible with **no** `Authorization` header (verified live 2026-06-18). A token is needed only to derive your own racer id (the `athlete --me` path) and as a fallback for the occasional auth-gated endpoint. When supplied it is a short-lived (~2h) user JWT from the Keycloak authorization-code flow (realm `athlinks`, client `www`, at `accounts.athlinks.com/auth/realms/athlinks/protocol/openid-connect/auth`), passed as `Authorization: Bearer <token>`.

**Anonymous access (verified live 2026-06-18):** `athletes/api/find`, `athletes/api/{racerId}/Races`, `event/{eventId}/results/search`, the per-athlete `result` detail, and the paged `event/{eventId}/results` list all return real data with **no** `Authorization` header (HTTP 200). The adapter sends requests anonymously and only falls back to requiring `ATHLINKS_TOKEN` if a request returns 401/403. (Historical note: a direct browser POST to the Keycloak token endpoint is CORS-blocked and no `client_credentials` grant for `client_id=www` is confirmed — but neither matters now that the data endpoints are open. The `login-status-iframe.html/init` silent-SSO check is not a token grant.)

### ID chain

```
masterEventId   — top-level event in URL (e.g. 390468)
eventId         — reignite-api event instance (e.g. 1094411)
raceId          — sub-course / eventCourseId (e.g. 2530164) — returned by search as eventCourseId
azpEventId      — timer system event ID (ctlive, etc.) (e.g. 83293)
azpEntryId/id   — per-entry ID (e.g. 70078023) — same value as thirdPartyEntryId in detail
```

Event metadata (courses, IDs):
```
GET https://alaska.athlinks.com/MasterEvents/Api/{masterEventId}
```
Returns full event structure including `eventRaces[].eventCourses[]` with `eventCourseId` (= raceId).

### Request

**1. Bib/name search (required to resolve raceId per athlete):**
```
GET https://reignite-api.athlinks.com/event/{eventId}/results/search?from=0&limit=20&term={bib_or_name}
Authorization: Bearer <token>   # optional — sent only when ATHLINKS_TOKEN is set
```
- `term` does prefix-match on bib and name
- Returns array of entry objects; use `eventCourseId` as `raceId` for the detail call

**2. Per-athlete detail:**
```
GET https://reignite-api.athlinks.com/event/{eventId}/race/{raceId}/bib/{bib}/result
Authorization: Bearer <token>   # optional — sent only when ATHLINKS_TOKEN is set
```
- `raceId` = `eventCourseId` from search response

**3. Paged results list (all athletes in event):**
```
GET https://reignite-api.athlinks.com/event/{eventId}/results?correlationId=&from={from}&limit={limit}
Authorization: Bearer <token>   # optional — sent only when ATHLINKS_TOKEN is set
```
- Returns array grouped by race (course); each group has `division`, `intervals[].results[]`
- Pagination: increment `from` by `limit`

**4. Legacy results (no auth required — older events only):**
```
GET https://results.athlinks.com/event/{legacyEventId}?eventCourseId=&divisionId=&intervalId=&from=0&limit=20
```
- No `Authorization` header needed
- Use for events not served by reignite-api

Fixtures: `testdata/fixtures/athlinks/search.json`, `testdata/fixtures/athlinks/results.json`, `testdata/fixtures/athlinks/detail.json`

Captured event: Paraguay Multisport Challenge 2024
- `masterEventId`: 390468
- `eventId`: 1094411
- sample `raceId`: 2530164 (2 km Group 1)
- sample bib: 8420

### Response → Result mapping

**From search (`/results/search`):**
- `bib` → BIB
- `displayName` → name (all-caps in search; mixed case in detail)
- `gender` ("M"/"F") → gender
- `age` → age
- `eventCourseId` → raceId (needed for detail call)
- `azpEventId` → timer system event ID (needed for media/photo calls)
- `azpEntryId` → entry ID

**From detail (`/race/{raceId}/bib/{bib}/result`):**
- `bib` → BIB
- `displayName` → name (mixed case)
- `gender` ("M"/"F") → gender
- `age` → age
- `status` → `"CONF"` = confirmed finish; also `"DNF"`, `"DNS"`
- `intervals[]` where `full=true` → finish interval:
  - `chipTimeInMillis` / 1000 → net time in seconds
  - `gunTimeInMillis` / 1000 → gun time in seconds
  - `divisions[]` where `type="overall"` → `rank` = overall place, `totalAthletes` = field size
  - `divisions[]` where `type="gender"` → `rank` = gender place
  - `divisions[]` where `type="other"` and name matches age group pattern (e.g. `"M30-39"`) → age group place
- `intervals[]` where `full=false` → splits (by `name`, e.g. `"Lap km 1"`):
  - `chipTimeInMillis` → split chip time
  - `distance.meters` → distance from start at split point

**From paged results (`/results`):**
- Top-level array elements group by race; `race.id` = raceId, `race.name` = course name
- `intervals[0].results[]` contains per-athlete rows with same fields as detail (minus splits):
  - `bib`, `displayName`, `gender`, `age`, `chipTimeInMillis`, `gunTimeInMillis`, `status`
  - `rankings.overall` → overall place
  - `rankings.gender` → gender place
  - `rankings.primary` → place within the division shown (not age group)
- Note: age group place not present in list — fetch detail for age group rank

**Time conversion:**
```
chipTimeInMillis / 1000  →  net seconds
gunTimeInMillis / 1000   →  gun seconds
```
Format for display: `seconds → H:MM:SS` (standard Go `time.Duration` formatting).

### Athlete (cross-event)

These two endpoints are served by `alaska.athlinks.com` (the legacy GraphQL-style API), not `reignite-api.athlinks.com`. Auth is optional here too — both return real data anonymously (verified live 2026-06-18); when sent, the token is the same Keycloak bearer token.

**1. Athlete search by name:**
```
GET https://alaska.athlinks.com/athletes/api/find?searchTerm={name}&limit={n}&skip={offset}
    &running=true&upTo5k=true&from5kTo15k=true&from15kToHalfMara=true
    &fromHalfMaraToMara=true&marathon=true&ultra=true&triathlon=true
    &sprint=true&olympic=true&halfIronman=true&ironmanAndUp=true
    &aquathlon=true&aquabike=true&duathlon=true&more=true
    &swim=true&mountainBike=true&cycling=true&snow=true
    &adventure=true&obstacle=true&other=true
    &gender=&fromAge=5&toAge=90&location=&withinRange=&sortBy=
Authorization: Bearer <token>   # optional — sent only when ATHLINKS_TOKEN is set
Origin: https://www.athlinks.com
```
- `searchTerm`: free-text name search (prefix match); `limit`/`skip` for pagination
- The sport/category boolean flags can all be `true` for an unrestricted search
- Returns `result.athletes[]` — each entry is an athlete (person) record, not a race entry
- `result.total`: total count across all pages

**racerId** is obtained from `result.athletes[i].racerId`. Athletes with `showPersonalData: false` have a null `profileUrl` — still searchable but profile page is private.

Fixture: `testdata/fixtures/athlinks/athlete-search.json`
- Captured: `searchTerm=Smith`, `limit=10`, `skip=0`
- `result.total`: 15074 | `result.athletes` has 10 items

Response → fields per athlete:
- `racerId` → stable athlete ID (use in profile URL and Races endpoint)
- `displayName` → full name
- `gender` ("M"/"F") → gender
- `age` → age
- `city`, `stateProv`, `country` → location
- `totalRaces` → count of claimed results across all events
- `profileUrl` → relative path e.g. `/athletes/{racerId}` (null if profile is private)
- `raceCategs` → array of sport category IDs the athlete has raced in
- `showPersonalData` → bool; if false, profile is private

**2. Athlete race history:**
```
GET https://alaska.athlinks.com/athletes/api/{racerId}/Races?start={offset}&limit={n}
Authorization: Bearer <token>   # optional — sent only when ATHLINKS_TOKEN is set
Origin: https://www.athlinks.com
```
- `racerId`: from athlete search above
- `start`/`limit`: pagination params (note: as of capture the API ignores limit and returns all entries — treat as a no-pagination endpoint for now)
- Returns all claimed race entries for the athlete, sorted newest-first

Fixture: `testdata/fixtures/athlinks/athlete-results.json`
- Captured: `racerId=43234281` (Sample Athlete, public profile, 118 races)
- Truncated to 3 entries; `Result.raceEntries.MasterCount` = 118

Response structure:
```
Result.raceEntries.MasterCount   — total number of race entries
Result.raceEntries.List[]        — array of race entry objects
```

Per-entry field mapping:
- `Race.RaceName` → event name (e.g. "Mohican 100 Trail Run")
- `Race.RaceDate` → ISO-8601 date (e.g. `"2023-06-03T04:00:00"`)
- `Race.MasterEventID` → masterEventId for the Athlinks event page URL
- `Race.Courses[0].CourseName` → sub-course/distance name (e.g. "Marathon", "5K")
- `Race.Courses[0].DistUnit` → distance in meters (e.g. `42164.81` = marathon)
- `Race.Courses[0].EventCourseID` → eventCourseId (= raceId for deep-link)
- `Race.City`, `Race.StateProvAbbrev`, `Race.CountryID` → event location
- `EventID` → reignite-api or alaska eventId
- `EventCourseID` → (string) raceId for deep-link to `reignite-api.athlinks.com/event/{eventId}/race/{raceId}/bib/{bib}/result`
- `BibNum` → bib number (string, may be empty for virtual events)
- `TicksString` → finish time formatted (e.g. `"4:28:11"`, `"24:57"`)
- `Ticks` → finish time in milliseconds (divide by 1000 for seconds)
- `RankO` / `CountO` → overall place / total finishers overall
- `RankG` / `CountG` → gender place / total finishers in gender
- `RankA` / `CountA` → age-group place / total finishers in age group
- `ClassName` → age group name (e.g. `"50 to 54"`)
- `ClaimStatus` → `"Claimed"` = athlete has claimed this result
- `RacerID` → same as `racerId` from search

**Deep-link to full result detail** (from race history entry):
```
https://www.athlinks.com/event/{Race.MasterEventID}/results/Event/{EventID}/Course/{EventCourseID}/Bib/{BibNum}
```

**`--me` shortcut:** The Keycloak JWT payload contains `"athlinks-racer-id": <int>` — decode the token's middle segment (base64url) to obtain the authenticated user's own `racerId` without an extra API call. This enables `athlinks athlete --me` that skips the search step entirely.

**Pagination note:** The `/Races` endpoint currently returns all entries regardless of `start`/`limit`. The `start=0&limit=N` params are accepted but may be ignored server-side. For large profiles (hundreds of races), expect a single large response.
