# Function Health Browser-Sniff Summary

## API Base (verified live, 2026-05-28)
- **Host:** `https://member-app-mid.functionhealth.com`
- **API root:** `/api/v1`
- **Previous host (daveremy/function-health-mcp docs, NOW STALE):** `https://production-member-app-mid-lhuqotpy2a-ue.a.run.app/api/v1` — returns 404
- **SPA host:** `https://my.functionhealth.com` (Nginx 1.29.1 serving the SPA shell for unknown GET paths; `/api/v1/login` POST returns 405)

## Verified Endpoints (HTTP 401 = exists, requires auth)
All return `{"detail":"Not allowed to perform this operation."}` when called without proper auth:

| Method | Path | Notes |
|---|---|---|
| GET | `/api/v1/user` | User profile |
| GET | `/api/v1/biomarkers` | Biomarker catalog |
| GET | `/api/v1/categories` | ~13 medical categories |
| GET | `/api/v1/results` | (per daveremy) PDF requisition data |
| GET | `/api/v1/results-report` | Structured lab results (the big one) |
| GET | `/api/v1/recommendations` | Personalized recommendations |
| GET | `/api/v1/biological-calculations/biological-age` | Bio age vs chronological |
| GET | `/api/v1/biological-calculations/bmi` | BMI with weight/height |
| GET | `/api/v1/notes` | Clinician notes |
| GET | `/api/v1/requisitions?pending=true` | In-progress draws |
| GET | `/api/v1/requisitions?pending=false` | Completed draws |
| GET | `/api/v1/pending-schedules` | Upcoming scheduled visits |
| GET | `/api/v1/notifications` | Change notifications |
| GET | `/api/v1/visits` | Individual visit/collection events |
| GET | `/api/v1/wearables/supported-apps` | NEW since daveremy — wearables integration list |
| GET | `/api/v1/biomarker-data/<biomarker-uuid>` | Single-biomarker detail; verified via real captured XHR (`f39fd8bc-e55a-4399-931e-5951ee694d61` = ApoB) |

## Confirmed-missing Endpoints (HTTP 404, were not documented but tested for completeness)
- `/api/v1/documents`, `/api/v1/test-rounds`, `/api/v1/wearables`, `/api/v1/reports`, `/api/v1/orders`, `/api/v1/protocols`

## Auth Model
- **Type:** Firebase Authentication (email/password)
- **Firebase apiKey** (public, embedded in SPA bundle): `AIza<redacted — public key, recoverable from the SPA bundle>`
- **Sign-in endpoint** (Google Identity Platform): `POST https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=<apiKey>` with `{email, password, returnSecureToken:true}` → returns `{idToken, refreshToken, expiresIn, localId, email}`
- **Refresh endpoint**: `POST https://securetoken.googleapis.com/v1/token?key=<apiKey>` with `{grant_type:"refresh_token", refresh_token}` → returns new `{access_token, refresh_token, expires_in}`
- **Bearer header**: `Authorization: Bearer <idToken>`
- **Token TTL:** 3600s (1 hour) — refresh handling is the active bug in daveremy/#22

## Required Request Headers (captured from real XHR)
```
Authorization: Bearer <Firebase idToken>
Accept: application/json, text/plain, */*
fe-app-version: 0.84.0   (current SPA version)
X-Backend-Skip-Cache: true
x-request-id: <uuid>
x-session-id: <uuid>
x-experiment-state: <opaque>
traceparent: <W3C trace>
referer: https://my.functionhealth.com/
```

## Reachability
- `printing-press probe-reachability` against the API host: `standard_http`, confidence 0.95 (no Cloudflare/WAF)
- Both stdlib HTTP and Surf-Chrome got responses

## Replayability Verdict
- All 14 verified endpoints are plain REST GET — fully replayable from any HTTP client with the Firebase Bearer token
- No GraphQL, no proxy envelope, no live page-context execution required
- The printed CLI will ship `auth login` (Firebase email/password flow) + `Authorization: Bearer` HTTP transport; **no resident browser, no cookie clearance.**

## Observed User Data (in-page evidence of authenticated session)
- Hemoglobin: 16.3 g/dL (In Range)
- ApoB: 72 mg/dL (In Range)
- Biological Age: "Younger -15.1 Years"
- Out-of-range biomarkers: Hematocrit, Lipoprotein(a), LDL Particle Number, LDL Small (+3 more)
- 13+ categories visible (Autoimmunity, Blood, Heart, Biological Age, etc.)

## Telemetry/Analytics Endpoints (NOT to be included in spec)
- `statsig.functionhealth.com/v1/rgstr` (Statsig feature flags)
- `api-js.mixpanel.com/track/` (Mixpanel events)
- `otel.services.functionhealth.com/v1/traces` (OpenTelemetry traces)
