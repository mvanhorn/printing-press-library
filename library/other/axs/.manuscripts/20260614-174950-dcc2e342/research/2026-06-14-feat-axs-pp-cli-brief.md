# SRAM AXS CLI Brief

## API Identity
- **Domain:** SRAM AXS — electronic drivetrain / component ecosystem (eTap AXS, Eagle AXS, RED/Force/Rival, Reverb AXS dropper posts, batteries, blips). The web app at axs.sram.com lets riders register components, manage bikes, check firmware, link ride services, and view component usage telemetry.
- **Users:** Cyclists who own SRAM AXS electronic components; mechanics/shops registering components; data-minded riders who want their wear/shift/battery telemetry programmatically.
- **Data profile:** Per-user bikes & components (serials, firmware versions, battery state), a global device-type / model / product catalog, and ride-derived component usage stats (shift counts, distance, wear) + activities.

## Reachability Risk
- **None.** API is reachable. `nexus.quarqnet.com/api/v2/devicetypes/` returns 200 with real JSON unauthenticated; auth-gated endpoints return clean `401 {"detail":"Authentication credentials were not provided."}` (Django REST Framework backend).
- Probe-safe public endpoint used: `GET nexus.quarqnet.com/api/v2/devicetypes/`.

## Spec Source
- No public OpenAPI spec. Surface reverse-engineered from the official web client bundle (static analysis) — see `discovery/bundle-sniff-report.md`. High confidence: these are the exact call sites of the shipping app.

## Auth
- **Auth0 password-realm** via `sramid-auth.sram.com`. Public SPA clientID `zIvfleoh46jy4behzZdkFoUIiW70KX23`, realm `sramid-db`, audience `https://api.quarqnet.com`, scope `openid profile email offline_access`. Runtime: `Authorization: Bearer <access_token>`.
- CLI auth model: hand-coded `auth login` (email + password → `POST https://sramid-auth.sram.com/oauth/token`, stores access+refresh token) PLUS `auth set-token` paste + `SRAM_AXS_TOKEN` env var. Read-only commands against the public `devicetypes` catalog work with no auth.

## Top Workflows
1. **See my bikes & components with firmware + battery state** — `bikes list`, `components list`, the single most common reason to open the app.
2. **Check whether my firmware is current** — compare each component's installed firmware against the `models` catalog's latest version.
3. **Register a component by serial** — `componentregistrations create --serial ... ` (the onboarding flow).
4. **Review component usage telemetry** — shift counts, distance, wear from `componentsummaries` / `activitysummaries`.
5. **Browse the device-type / product catalog** — `devicetypes list`, `models list`, `products get` (works unauthenticated).

## Table Stakes
- List/get bikes, components, device types, models, products, notifications, linked accounts.
- Register a component; request a data export.
- Activity + component-usage telemetry.
- JSON/CSV/select output; offline local store + search for the user's own bikes/components/activities.

## Data Layer
- Primary entities: `bikes`, `components`, `componentregistrations`, `devicetypes`, `models`, `products`, `notifications`, `linkedids`, `activities`, `activitysummaries`, `componentsummaries`.
- Sync cursor: DRF pagination (`page`/`page_size`); activities are time-ordered.
- FTS/search over local bikes/components/activities (offline, no incumbent offers this).

## Product Thesis
- **Name:** SRAM AXS CLI (`axs`)
- **Why it should exist:** The only way to touch your AXS data today is the web app or phone app — point-and-click, no export, no scripting. A CLI gives riders and mechanics scriptable access to their own components, a real "is my firmware current?" check, offline-searchable history, and agent-native JSON output the web app can't provide.

## Build Priorities
1. Read surface over the two quarqnet hosts (account/device + telemetry), bearer auth, public `devicetypes` works without login.
2. Local SQLite store + sync + search for bikes/components/activities.
3. Transcendence: firmware-staleness check (installed vs catalog latest); component-wear/usage rollup across all bikes; battery-state dashboard.
4. `auth login` (Auth0 password-realm) + token paste; `componentregistrations create` with `--dry-run`.

## Notes / Constraints
- Live authenticated testing requires the user's SRAM token, unavailable in this run → Phase 5 auth-gated endpoints will be tested via dry-run + the public `devicetypes` endpoint only; full live dogfood is the user's follow-up once they paste a token.
