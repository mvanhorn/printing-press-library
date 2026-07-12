# Jane App CLI Brief

## API Identity
- Domain: Practice-management + online booking for health/wellness clinics (physio, massage, chiro, acupuncture, mental health).
- Multi-tenant: every clinic is its own subdomain, e.g. `embophysio.janeapp.com`, `leahkangas.janeapp.com`. Each tenant has its own data AND its own patient login. A patient at two clinics has two independent accounts.
- Users: patients/clients booking and managing their own appointments.
- Data profile: locations, disciplines, staff_members (practitioners), treatments (services w/ price+duration+book_online flag), openings (availability), appointments (auth-gated, the patient's own bookings).

## Reachability Risk
- None. Public read endpoints return 200 unauthenticated; the auth-gated appointment endpoint returns a clean 401 JSON. No bot protection, no Cloudflare challenge. Rails app (`x-jane-version` header, `_front_desk_session` cookie).

## API Surface (probed live, identical across both clinics)
Public (no auth):
- `GET /api/v2/locations`      → clinic locations (id, name, address, booking_url, slug)
- `GET /api/v2/disciplines`    → disciplines (id, name, description)
- `GET /api/v2/treatments`     → { treatments: [ {id, name, price, scheduled_duration, book_online, discipline_id, staff_member_id, location_ids, capacity, online_only} ] }
- `GET /api/v2/staff_members`  → practitioners (id, name via description, all_treatment_ids, allow_online_booking)
- `GET /api/v2/openings?location_id=&treatment_id=&staff_member_id=&start_date=YYYY-MM-DD&num_days=1..7`
      → [ { id, full_name, first_date, openings:[{staff_member_id, location_id, treatment_id, duration, start_at, end_at, status}], shifts:[...] } ]
      NOTE: num_days hard-capped 1..7 (422 otherwise). staff_member_id required (404 without).

Auth-gated (session cookie required):
- `GET /api/v2/appointments`   → 401 { "message": "...don't have access..." } when unauthenticated → the patient's own appointments (view)
- login  (form POST, exact route TBD via sniff; `/login` 302→/cookies_test)
- create appointment (book), reschedule, cancel → TBD via sniff

## Auth model
- Rails session cookie `_front_desk_session` (HttpOnly, Secure). CSRF via `<meta name="csrf-token">` + param `authenticity_token` on form POSTs.
- CLI auth mode: cookie-based. Capture the session cookie via browser login per provider, store per-profile, replay on `/api/v2/appointments` + booking writes.
- Multi-tenant → auth is PER PROFILE. Each profile = { base_url (subdomain), cookie/session }.

## Top Workflows
1. Book an appointment: pick clinic profile → discipline → practitioner → treatment → availability window → confirm slot.
2. View my appointments: upcoming + past, across one or all profiles.
3. Reschedule / cancel an existing appointment.
4. Find the next available opening for a given practitioner+treatment (availability search across the 7-day-window limit, auto-paged).

## Table Stakes (from Jane's own patient portal "My Account")
- View upcoming appointments, review history (past), begin telehealth.
- Book new appointments (incl. under packages/memberships).
- Cancel or reschedule (clinic-permitting; always cancellable within 3 min of booking).
- (Out of initial scope but portal offers: secure messaging, intake forms, documents, invoices/receipts, profile/contact update, payment methods.)

## Data Layer
- Primary entities: profiles(providers), locations, disciplines, staff_members, treatments, appointments (cached), openings (ephemeral, not persisted).
- Sync cursor: appointments per profile (updated_at). Reference data (treatments/staff/locations) refreshed on demand.
- FTS/search: treatments + staff_members + appointments by name/description.

## Product Thesis
- Name: JaneApp CLI (`janeapp-pp-cli`)
- Why it should exist: Jane has no public patient API and no CLI. A patient at multiple clinics must log into each subdomain separately in a browser to see or book anything. This CLI unifies every Jane clinic behind one tool: one `appointments upcoming --all-profiles` shows every booking across every clinic; availability search auto-pages past the 7-day window cap; booking is scriptable and agent-native.

## Build Priorities
1. Profile system: add/list/select providers, each with its own subdomain base URL + cookie session. Cookie login (`auth login`).
2. Public discovery commands: locations, disciplines, staff, treatments, openings (availability), with auto-paging over the 7-day cap.
3. View appointments (upcoming/past, per-profile and --all-profiles), backed by local SQLite cache.
4. Book / reschedule / cancel (write path, --dry-run, confirmation).
5. Transcendence: cross-clinic unified view, next-opening finder, availability watch, conflict-aware booking.
