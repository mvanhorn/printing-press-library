# api.goose.pet admin endpoint catalog (sanitized)

> Captured via driven-Chrome browser-sniff. Bodies omitted; only shapes & query patterns recorded.
> All `<facility>` paths are scoped per facility (slug, e.g. `your-facility`).

## Auth
- **Type:** AWS Cognito JWT Bearer
- **Header:** `Authorization: Bearer <jwt-access-token>`
- **Issuer:** `https://cognito-idp.us-east-2.amazonaws.com/us-east-2_IqPUw1L4C`
- **Client ID:** `4qv4b8pvtsqigsontd3vfmf6kf` (public, embedded in app.goose.pet bundle)
- **Token TTL:** ~1 hour (`exp - iat = 3600s`)
- **Scope:** `aws.cognito.signin.user.admin`
- **Groups encoded in JWT:** `A:<facility>:AA` (admin), `L:<facility>:AA` (location), `USR:<userid>:SELF`, `USR:<email>:EMAIL`

## Origin & Headers
- **Request origin:** `https://app.goose.pet`
- **API host:** `https://api.goose.pet`
- **Required headers:** `Authorization`, `Origin: https://app.goose.pet`, `Referer: https://app.goose.pet/`, `Accept: application/json`
- **Server:** CloudFront → API Gateway → Lambda (`x-amz-apigw-id`, `x-amzn-trace-id`)
- **Compression:** gzip

## API conventions
- **Base path:** `/api/v1/admin/<facility>`
- **Includes (nested relations):** `?includes[]=order.pets.locationPetProfile.tags.tag` — bracketed-array, dot-notation deep relations
- **Filters:** plain query params + bracketed arrays for multi-value (`invoiceStatus[]=CONFIRMED`)
- **Pagination hint:** `limit=500` seen on dashboard/invoices
- **ID format:** CUIDs (`cmn4ykmqw2v3vpufqp850yhp9`)
- **Caching:** ETags + `304 Not Modified` honored

## Endpoints discovered

### Dashboard
- `GET /api/v1/admin/<facility>/dashboard/invoices` — today's bookings/visits
  - Query: `visitDate=YYYY-MM-DD`, `invoiceStatus[]=CONFIRMED|...`, `items.offerType=PRIMARY`, `items.offerTypeFilter=PRIMARY`, `limit=500`
  - Includes (observed): `order.invoices`, `order.pets.locationPetProfile.tags.tag`, `order.pets.locationPetProfile.vaccinations`, `order.locationUserProfile.agreements.contract`, `order.locationUserProfile.locationUserProfileMemberships`, `order.pets.locationPetProfile.petInstructions.consumables`, `instructions`
  - Response shape: `{ results: Invoice[] }` where Invoice = { id, createdAt, invoiceStatus, invoiceSubStatus, locationServiceType, order, period, invoiceItems[], instructions[], inStayInvoices[], visitStatus[], visitSubStatus[], allowedActions[], total, subtotal, tax, location }

### Customers (location-user-profiles)
- `POST /api/v1/admin/<facility>/location-user-profiles/outstanding-balance` — bulk balance lookup

### Conversations / Messaging
- `GET /api/v1/admin/<facility>/conversations` — Intercom-style conversation list (separate from real Intercom which is also embedded)

### Service catalog
- `GET /api/v1/admin/<facility>/location-service-types` — boarding/grooming/etc.
  - Query: `includes[]=serviceType`, `hasSharedCRM=false`, `includeShared=false`, `sortOrder=asc`

## Domain entities (inferred from invoice response)
- **invoice** — id, status (CONFIRMED, etc.), substatus (IN_PROGRESS, etc.), period {start/end date+time}, totals, allowedActions
- **order** — wraps invoice, has orderUser
- **orderUser / locationUserProfile** — customer: id, email, phone, firstName, lastName, agreements[], hasUnconfirmedMembershipPayment, memberships
- **agreement** — SERVICE_AGREEMENT with contract (id, type, name=revision number)
- **invoiceItem** — line item: displayName, invoicePets[]
- **invoicePet** — id, locationPetProfileId, sex, weight, altered, displayName, breed, color, dateOfBirth, status (ACTIVE), profileImage {key, bucket}, vaccinations[], tags[], petInstructions[], temperTestResults, careInstruction, locationSpecies (dog/cat), activities[] (current resourceUnit)
- **locationServiceType** — id, displayName, name, serviceType { id, name, overnight, isVisit }
- **petInstruction** — type (FEEDING|MEDICATION|BELONGINGS|...), description, scheduleAM/PM/Other, frequency, date
- **instruction** — type=BELONGINGS, body=free-text
- **activity** — id, activityStatus (IN_PROGRESS/...), startedAt, endedAt, resourceUnit (e.g. "Cozy Cottage - 3")
- **tag** — icon, color, priority, displayName (e.g. "Senior Dog", "Play Yard B", "Sensitive Stomach")
- **location** — id, name (slug), displayName, timezone
- **vaccination** — expirationDate, locationVaccineId

## Microservice hosts (4 total)
- `api.goose.pet` — main admin API (most resources)
- `search-api.goose.pet` — search service (Elasticsearch-backed full-text)
- `pawgress-report.goose.pet` — report cards / pet progress notes
- `soar-api.goose.pet` — Explo embed token broker for analytics dashboards

## Additional endpoints discovered

### Bookings/Reservations (`invoices` resource)
- `GET /api/v1/admin/<facility>/invoices` — universal booking list, accepts:
  - Filters: `items.locationServiceTypeId=<id>`, `locationServiceType=<id>`, `invoiceStatus[]=CONFIRMED|...`, `invoiceSubStatus[]=REQUESTED|PENDING_SCHEDULED|IN_PROGRESS|...`, `order.locationUserProfileId=<id>`, `period.startDate=gte_YYYY-MM-DD`, `visitDate=YYYY-MM-DD`
  - `distinguishes[]=orderId` — collapse to one row per order
  - `limit`, `sortOrder=desc|asc`
  - Filter convention: **`gte_`, `gt_`, `lte_`, `lt_`, `eq_` prefix on date params** (e.g., `period.startDate=gte_2026-05-10`)

### Scheduler (grooming)
- `GET /api/v1/admin/<facility>/resources?type=PERSON&subtype=GROOMING_STAFF&status=ACTIVE` — staff list
- `GET /api/v1/admin/<facility>/resource/availability-v2?type=&subtype=&start=YYYY-MM-DD&end=YYYY-MM-DD` — availability
- `GET /api/v1/admin/<facility>/location-booking-confs?locationServiceType=<id>` — booking configuration

### Customers (`location-user-profiles`)
- `GET /api/v1/admin/<facility>/location-user-profiles/<userId>?includes[]=...` — customer detail with: locationPetProfiles, tagRelations.tag, agreements, attachments, vouchers, locationUserProfileMemberships, associatedContacts
- `POST /api/v1/admin/<facility>/location-user-profiles/outstanding-balance` — bulk balance lookup
- Search: `GET https://search-api.goose.pet/api/v1/admin/account/<facility>/location-user-profile/search?query=&location=&include=petProfiles&from=&size=`

### Customer related
- `GET /api/v1/admin/<facility>/vouchers?status=&type=OFFER|CASH&locationUserProfile=<id>&isUsed=false&expired=false&atLeastOneAvailable=true` — credits/packages
- `GET /api/v1/admin/<facility>/stored-payment-methods-v2?locationUserProfile=<id>`
- `GET /api/v1/admin/<facility>/stored-payment-methods?locationUserProfile=<id>`
- `GET /api/v1/admin/<facility>/notes?locationUserProfile=<id>&sortOrder=desc&limit=N` — customer/pet notes
- `GET /api/v1/admin/<facility>/contracts?type=SERVICE_AGREEMENT&status=ACTIVE`

### Species/breeds catalog
- `GET /api/v1/admin/<facility>/location-species?includes[]=species&includes[]=species.breeds` — species + breeds

### Reports
- `GET /api/v1/admin/<facility>/report-types?includes[]=reportServiceTypes.serviceType&limit=99&status=ACTIVE` — catalog of reports
- `GET /api/v1/admin/<facility>/report-types?includes[]=...&name=<report-slug>` — single report metadata
- `GET /api/v1/admin/<facility>/reports/<export-slug>?<params>` — direct CSV/data export (e.g. `reports/feeding-medication-export?date=YYYY-MM-DD`)
- `GET https://soar-api.goose.pet/api/v1/admin/<facility>/token/explo?embedId=<id>` — Explo embed token (for dashboard reports; CLI can't run the SQL but can mint+open the URL)

### Report cards
- `GET https://pawgress-report.goose.pet/api/v1/admin/<facility>/reportcard?localDate=YYYY-MM-DD&timeZoneOffsetMinutes=N` — daily report cards

### Messaging
- `GET /api/v1/admin/<facility>/conversations` — message threads (custom Goose conversations, separate from embedded Intercom)

### Revenue management
- `GET /api/v1/admin/<facility>/restrictions?type=PRICE|OCCUPANCY&targetType=OFFER|PRICE_ADJUSTMENT&includes[]=periods,restrictions,restrictionRelations.offer`

### Service catalog
- `GET /api/v1/admin/<facility>/location-service-types?includes[]=serviceType,restrictionRelations.restriction.periods,inStays&status=ACTIVE&sortOrder=asc` — services offered (boarding, daycare, grooming, etc.)
  - `hasSharedCRM`, `includeShared` filters

## Auth localStorage layout (app.goose.pet)
- `CognitoIdentityServiceProvider.4qv4b8pvtsqigsontd3vfmf6kf.LastAuthUser` → email
- `CognitoIdentityServiceProvider.4qv4b8pvtsqigsontd3vfmf6kf.<email>.accessToken` → JWT (1hr TTL)
- `CognitoIdentityServiceProvider.4qv4b8pvtsqigsontd3vfmf6kf.<email>.refreshToken` → opaque, long-lived
- `CognitoIdentityServiceProvider.4qv4b8pvtsqigsontd3vfmf6kf.<email>.idToken` → JWT id
- `CognitoIdentityServiceProvider.4qv4b8pvtsqigsontd3vfmf6kf.<email>.clockDrift` → seconds
- `CognitoIdentityServiceProvider.4qv4b8pvtsqigsontd3vfmf6kf.<email>.userData` → JSON blob

## Cognito refresh flow
- Endpoint: `POST https://cognito-idp.us-east-2.amazonaws.com/`
- Headers: `Content-Type: application/x-amz-json-1.1`, `X-Amz-Target: AWSCognitoIdentityProviderService.InitiateAuth`
- Body: `{ "AuthFlow": "REFRESH_TOKEN_AUTH", "ClientId": "4qv4b8pvtsqigsontd3vfmf6kf", "AuthParameters": { "REFRESH_TOKEN": "<token>" } }`
- Response: `{ "AuthenticationResult": { "AccessToken": "...", "IdToken": "...", "ExpiresIn": 3600, "TokenType": "Bearer" } }`
