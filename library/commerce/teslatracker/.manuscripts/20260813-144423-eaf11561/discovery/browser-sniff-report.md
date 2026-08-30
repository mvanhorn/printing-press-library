# TeslaTracker browser-sniff report

Backend: browser-use 0.1.8 attached to the user's running Chrome via CDP.
Method: loaded `/inventory`, installed fetch/XHR interceptors, SPA-clicked a vehicle
link (no reload, per cardinal rule 3), captured 27 calls.
Auth: page reported no signed-in markers; **every endpoint below answered anonymously**.

## Replayable endpoints found

| Endpoint | Method | Status | Notes |
|---|---|---|---|
| `/api/inventory/{vin}` | GET | 200 | full vehicle record |
| `/api/inventory/{vin}/report` | GET | 200 | vehicle history report, superset of detail |
| `/api/advisor/conversations?vin={vin}` | GET | 200 | advisor thread |
| `/inventory?_rsc=<hash>` | GET | 200 | Next.js RSC payload for list |

Non-product traffic (analytics/ads) ignored: clarity.ms, doubleclick, mediavine.

## Field inventory — `/api/inventory/{vin}`
vin, model, trim, year, condition, exteriorColor(+Long), interiorColor(+Long),
hardwareVersion, wheels, hasFsd, mileage, locationCity, locationState,
**latitude, longitude**, teslaUrl, photoUrl, driveType, range, acceleration, topSpeed,
**warrantyBatteryExpDate, warrantyVehicleExpDate, warrantyBatteryMile, warrantyVehicleMile**,
**transportFee**, purchasePrice, vehiclePhotos[]

## Extra fields in `/api/inventory/{vin}/report`
optionCodeList, factoryCode, **factoryGatedDate**, **actualRange** (vs rated `range`),
category, currentPrice, totalPrice, discount, leaseMonthly,
finplatDetails (term, interestRate, monthlyPayment, cashDownPayment),
autopilot[], isAvailable, **firstSeenAt, lastSeenAt**

## Units
Money is in **cents**: purchasePrice 2060000 = $20,600; transportFee 100000 = $1,000.

## Replayability verdict
PASS — plain HTTPS GET, JSON, no auth, no clearance cookie, no page-context execution.
Runtime mode: `standard_http`.

## Why this matters beyond the CLI
These records are Tesla's own inventory data. tesla.com/inventory returns HTTP 403 to any
cold request (verified same day); teslatracker.com serves the same underlying fields over
unauthenticated HTTP. Fields that are estimates elsewhere are exact here:
`warrantyBatteryExpDate` (real date, not a model-year guess), `transportFee` (Tesla's own
shipping quote, not a distance band), `actualRange` vs `range` (measurable battery
degradation), `firstSeenAt`/`lastSeenAt` (true days-on-market), lat/lon (exact, not a ZIP centroid).
