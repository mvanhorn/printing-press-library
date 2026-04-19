# Happenstance web-app sniff, 2026-04-19

Captured against Matt Van Horn's signed-in Chrome session via the claude-in-chrome MCP.
Every request below was observed in the DevTools Network panel or intercepted by a
monkey-patched `window.fetch`. All JWTs, session tokens, and cookie values are redacted.

## TL;DR — root causes of contact-goat's silent-empty bug

1. Clerk refresh endpoint is `POST /v1/client/sessions/{sess}/touch`, not `/tokens`.
   contact-goat's `internal/client/cookie_auth.go` calls `/tokens` and always gets HTTP 404.
   That is why `MaybeRefreshSession` dies after ~60 seconds.

2. The CLI's `coverage` uses `GET /api/friends/list`, which is the narrow "top connectors"
   widget and only returns the 3 users Matt follows on Happenstance. The real graph search
   lives at `POST /api/search` + `GET /api/dynamo?requestId=...`. That endpoint returned
   30 people for "Warner Bros" and 19 for "Weber Inc" on this same signed-in session.

3. contact-goat's `POST /api/research` attempt uses snake_case keys (`request_text`,
   `request_content`). The server expects camelCase (`requestText`, `requestContent`).
   That is why every prior POST returned 204 or 500 with empty body.

## Endpoint: Clerk session refresh

### Observed

```
POST https://clerk.happenstance.ai/v1/client/sessions/sess_3CaMtjozgFPsjhyE33et4msqTS4/touch
     ?__clerk_api_version=2025-11-10
     &_clerk_js_version=5.125.9
→ 200
```

Headers of interest on request: browser sends `__client_uat`, `__session`, `__refresh_*`
cookies automatically via the jar. Body is empty.

Headers of interest on response:
- `X-Clerk-Auth-Status: signed-in`  (when the refresh succeeded)
- `X-Clerk-Auth-Status: signed-out` with `X-Clerk-Auth-Reason: ...` (when the token is dead)
- Sets a fresh `__session` cookie on the `.happenstance.ai` domain.

### contact-goat bug

`internal/client/cookie_auth.go:219-220` builds:

```go
fmt.Sprintf("%s/v1/client/sessions/%s/tokens?__clerk_api_version=%s",
    clerkBaseURL, sessionID, clerkAPIVersion)
```

The path is wrong. Clerk's current frontend-API calls `/touch`, not `/tokens`. Endpoint
`/tokens` is a different (server-to-server) Clerk surface. Fix: replace `/tokens` with
`/touch` and append `&_clerk_js_version=5.125.9` to match the frontend contract.

## Endpoint: create a search

### Observed

```
POST /api/search
Content-Type: application/json

{
  "requestText": "engineers at Stripe",
  "requestContent": [{"children":[{"text":"engineers at Stripe"}], "type":"p"}],
  "requestGroups": ["your-connections", "your-friends"],
  "parentRequestId": null,
  "excludePersonUUIDs": [],
  "searchEveryone": false,
  "creditId": null
}

→ 200
{"status":"Request received","id":"34becdcd-c706-4dc0-ab60-8fbfe055e8e7"}
```

Field semantics:

| Field | Meaning |
|---|---|
| `requestText` | Free-text natural-language query. Happenstance runs its own parsing to pull out company / title / traits filters. |
| `requestContent` | Slate-style rich-text representation of the same query. The web app always sends both. The server accepts mismatches but the safe path is to send both in lockstep. |
| `requestGroups` | Array of tier selectors. `"your-connections"` = 1st-degree (your synced LinkedIn / Gmail / etc contacts). `"your-friends"` = 2nd-degree (friends-of-friends visible via your Happenstance-friend graph). Custom groups also go here by uuid. |
| `parentRequestId` | Previous request uuid when refining. `null` for a fresh search. |
| `excludePersonUUIDs` | Used to hide already-seen people on "find more". |
| `searchEveryone` | Boolean. `true` flips it to the 3rd-degree / public-everyone mode. |
| `creditId` | Required for some paid-credit-gated flows; `null` for free use. |

Returns a generated server-side uuid immediately. The actual search runs async.

### contact-goat bug

Prior attempts sent `request_text` / `request_content` (snake_case). Server silently 500s
without a body. Must send the camelCase keys above.

## Endpoint: poll for results

### Observed

```
GET /api/dynamo?requestId=34becdcd-c706-4dc0-ab60-8fbfe055e8e7
→ 200
[
  {
    "request_id": "34becdcd-c706-4dc0-ab60-8fbfe055e8e7",
    "request_text": "people at Weber Inc",
    "request_content": [ ... Slate ... ],
    "request_status": "Found 19 people",
    "completed": true,
    "include_my_connections": true,
    "include_my_friends": true,
    "search_everyone": false,
    "request_groups": [],
    "logs": [ { "type": "INFO", "message": "...", "timestamp": "..." }, ... ],
    "results": [ <person>, ... ],
    "s3_results_uri": "s3://hpn-search-results-prod/results/{uuid}.json",
    "user_id": "user_...",
    "user_display": { ... },
    "timestamp": "...",
    "version": 1.1
  }
]
```

Response is an array with exactly one object (the search record). `completed: true` signals
the poll can stop. `results` is inline — no separate S3 fetch required.

### Result shape (per person)

Keys:

```
author_name
person_uuid
score
linkedin_url
twitter_url
instagram_url
quotes            — HTML-bold-highlighted prose describing the person
quotes_cited      — [{text, url}, ...] source citations for each quote
current_title
current_company
summary
referrers         — {is_yc: bool, referrers: [<referrer>]}
traits
```

### Referrer shape (this is the 1st/2nd-degree evidence)

```
{
  "id": "a73cbe02-c7fc-47b1-8a5e-d6ff3b7a205f",
  "name": "Matt Van Horn",
  "source": ["LinkedIn"],
  "image_url": "https://img.clerk.com/...",
  "affinity_score": 49.99,
  "affinity_level": "medium",
  "is_directory_user": false
}
```

When `referrers.referrers[0].id == current_user_uuid`, the result is 1st-degree.
When it's another user's uuid, the result is 2nd-degree via that friend.

### Polling cadence observed

Web app polls `/api/dynamo?requestId=X` about every 1-2 seconds while the search runs.
Stops when `completed: true`. Warner Bros search completed in 7 seconds (30 results),
Weber Inc in 6 seconds (19 results).

## Endpoint: uploads status (for doctor coverage line)

```
GET /api/uploads/status/user
→ 200
{
  "statuses": {
    "linkedin_ext": [
      {
        "id": "...",
        "status": "COMPLETED",
        "isActive": true,
        "isError": false,
        "isDeleting": false,
        "timestamp": "2025-03-18T17:06:05.000Z",
        "last_refreshed": "2025-07-09T21:36:00.371Z",
        "s3_uri": "s3://connections-uploads/linkedin_ext/...csv",
        "source_identifier": null
      }
    ]
  },
  "hasActiveUploads": true,
  "hasErrorUploads": false,
  "hasProcessingUploads": false,
  "hasDeletingUploads": false,
  "hasAnyUploads": true,
  "activeAccountsCount": 1
}
```

`statuses.linkedin_ext[0].last_refreshed` is what `doctor` should show as the contact-graph
freshness line. No record count on this endpoint; that comes from the dynamo response.

## Other endpoints observed (not in scope for this plan)

| Path | Verb | Purpose |
|---|---|---|
| `/api/dynamo/recent` | GET | Recent searches list, for the sidebar History UI |
| `/api/research/recent` | GET | Alternate recent-list shape |
| `/api/search/{id}/suggested-posts` | GET | Related posts for a completed search |
| `/api/friends/list` | GET | Top-connectors (the narrow endpoint coverage currently uses) |
| `/api/clerk/referrer?referrerId={uuid}` | GET | Resolve a referrer uuid to a user record |
| `/api/notifications?page=1&limit=10` | GET | Bell icon |
| `/api/user`, `/api/user/limits` | GET | Current user, rate limits |

## Concrete samples from today's session (redacted)

Real search uuids captured this session — useful for replay tests once the real client lands:

- Warner Bros: `cd5a3e1a-e8ff-42e5-a7e0-63de0c6faf4f` (30 results)
- Weber Inc: `e148718b-492e-433f-aa9a-d7b31ae7332b` (per-page), `34becdcd-c706-4dc0-ab60-8fbfe055e8e7` (programmatic)  (19 results)
- Stripe engineers: `49722417-468c-47fd-bd07-03fa5ba99412`
- Sequoia investors: `59554597-9f7d-45b3-b70d-551c9f0512f4`

Sample 1st-degree result from Weber Inc search: Gabriel Risk, Director of Engineering at
Weber Inc., person_uuid `fff0ae8e-9940-4858-8d8a-6e60a84f5e9f`, referrer = Matt Van Horn
(LinkedIn, affinity_level medium).

## Deltas vs. the current contact-goat implementation

| Area | Current | Correct |
|---|---|---|
| Clerk refresh path | `/v1/client/sessions/{sess}/tokens` | `/v1/client/sessions/{sess}/touch` |
| Clerk refresh query | `?__clerk_api_version=2025-11-10` | `?__clerk_api_version=2025-11-10&_clerk_js_version=5.125.9` |
| Create search body | `{request_text, request_content}` (snake_case) | `{requestText, requestContent, requestGroups, parentRequestId, excludePersonUUIDs, searchEveryone, creditId}` (camelCase) |
| Create search path | `POST /api/research` (attempted) | `POST /api/search` |
| Result retrieval | Not wired | `GET /api/dynamo?requestId={id}` poll until `completed: true` |
| 1st vs 2nd degree | No distinction | `referrers.referrers[0].id == current_user_uuid` → 1st-degree |

## Notes on cookies

The claude-in-chrome session showed 14 happenstance cookies. Only `__session`, `__refresh_K8Qez0yT`,
`__client`, `__client_uat_K8Qez0yT`, and `clerk_active_context` are needed for auth. Intercom and
PostHog cookies are noise and can be excluded when exporting from Chrome.
