# GreatClips Browser-Sniff Report

## Capture method
Claude-in-Chrome MCP driving the user's already-logged-in Chrome session
(`app.greatclips.com`, signed in as Matt Van Horn, favorited salon
"Island Square" salon #8991, Mercer Island WA).

Interception: `window.fetch` monkey-patch + `XMLHttpRequest.prototype.send`
hook, with response bodies tee'd to `window.__sniff`. Cross-referenced
URLs against `performance.getEntriesByType('resource')` and grep of the
Next.js JS bundles for path constants.

## Three host families

### 1. cid.greatclips.com - Auth0 customer identity
- `GET /authorize` (browser redirect, PKCE)
- `POST /oauth/token` (token exchange)
- Tenant marker in localStorage: `auth0.eq2A3lIn48Afym7azte124bPd7iSoaIZ.is.authenticated`
- Returns Auth0 JWT used as Bearer for the other two host families

### 2. webservices.greatclips.com - GreatClips backend
- `GET  /cmp2/profile/get` -- customer profile (name, email, phone, favorited salons, recent visits)
- `POST /customer/salon-search/term` -- body `{term, radius, limit, lat, lng}`; returns salon metadata WITHOUT wait time
- `POST /customer/salon-search/salon` -- body `{num, limit}`; single-salon detail
- `POST /customer/geo-names/postal-code` -- body `{query, limit}`; returns lat/lng/city for a zip
- `POST /customer/geo-names/term` -- city/term geocoding
- `GET  /customer/salon-hours/upcoming?salonNumber=X` -- 14-day hours forecast with special-hours support

### 3. www.stylewaretouch.net - ICS Net Check-In (third-party check-in vendor)
The footer of every page reads "Powered by ICS Net Check-In(TM)". This is
the actual wait-time and queue-management vendor; GreatClips proxies its
own brand on top.

- `POST /api/store/waitTime` -- body `[{storeNumber}, ...]`; returns `{stores: [{storeNumber, storeName, estimatedWaitMinutes, stateFlags, ociSalonStateCode, ociSalonStateDescription}]}` (e.g. "ActiveOci")
- `POST /api/customer/checkIn` -- body `{firstName, lastName, phoneNumber, salonNumber, guests}` -- "guests" is party size, 1-5
- `GET  /api/customer/status` -- current position in line for an active check-in
- `POST /api/customer/cancel` -- cancel an active check-in

## Confirmed sample responses

### Wait-time batch (request: array of `{storeNumber}`)
```
{"stores":[
  {"storeNumber":"2361","storeName":"Westgate on Pearl Street","estimatedWaitMinutes":0,"stateFlags":0,"ociSalonStateCode":0,"ociSalonStateDescription":"ActiveOci"},
  {"storeNumber":"2404","storeName":"Overlake Fashion Plaza","estimatedWaitMinutes":20,...},
  {"storeNumber":"8991","storeName":"Island Square","estimatedWaitMinutes":14,...}
]}
```

### Salon detail (request: `{num:"8991", limit:"1"}`)
Returns full record with `salonName`, `salonNumber`, `primaryAddress`,
`phoneNumber`, `longitude`/`latitude`, `proximity`, `hoursMondayToFridayOpen`,
`todayOpenStatus`, `todayOpenTimeDT` (`/Date(unixmillis)/` format).

### Salon hours (`?salonNumber=8991`)
14 forward days, each:
```
{"salonHoursId":17669201,"salonHoursDate":"2026-05-11T00:00:00","dayOfWeekCode":2,"dayOfWeek":"Monday","salonOpenStatusCode":1,"salonOpenStatus":"Open","specialHoursReason":null,"specialHoursTitle":null,"salonOpenTime":"09:00 AM","salonCloseTime":"07:00 PM"}
```

### Geo-names (request: `{query:"98040", limit:"20"}`)
```
{"results":[{"city":"Mercer Island","state":"WA","lat":47.56025,"lng":-122.228083,"population":null,"postalCode":"98040"}]}
```

## Important findings

- **Wait time is on the ICS service, salon metadata is on the GreatClips
  service.** A useful list view JOINs the two by `salonNumber`.
- **Party size cap is 5.** Form dropdown was "1 person ... 5 people".
- **Auth is one OAuth flow that unlocks both webservices and stylewaretouch.** A
  single Bearer token is reused across host families.
- **No public Swagger / OpenAPI.** No community wrappers. No competing CLI.
- **No CAPTCHA, no bot-protection signals** observed on the captured calls.
  Direct HTTP with Bearer is expected to work.

## What is NOT captured

The check-in submit (`POST /api/customer/checkIn`) was NOT exercised here -
clicking that button would have added the user to a real waitlist. The
request body shape is recovered from the JS bundle constants
(`{firstName, lastName, phoneNumber, salonNumber, guests}`). Live test
deferred to Phase 5 dogfood with the user's explicit approval.
