# Mariana Tek CLI Brief

## API Identity
- **Domain**: Boutique-fitness / wellness studio class booking SaaS (Xplor-owned). Powers studios, gyms, yoga/pilates/sauna brands. Each tenant is a brand subdomain (`{tenant}.marianatek.com`).
- **Users**: Studio members (booking classes), studio admins (back office), integration developers building custom booking experiences.
- **Data profile**: ClassSession-heavy (each session = a bookable slot); reservation-per-user-per-session; per-tenant catalogs of locations, instructors, packages, credits, memberships.

## Reachability Risk
- **None / Low.** Official OpenAPI 3.0.3 spec is hosted at `https://docs.marianatek.com/api/customer/v1/schema/` (292KB, 81 endpoints). OAuth2 Authorization Code flow. Live tenant API at `https://{tenant}.marianatek.com/api/`. Existing community client `bigkraig/marianatek` (Go) targets the same surface for Barry's Bootcamp.

## Top Workflows
1. **Discover and book** — list upcoming classes at one or more tenants, filter by date/location/instructor/duration/intensity, book the slot.
2. **Manage existing reservations** — view my upcoming bookings, swap to a different class, cancel (with cancel-penalty awareness), check in.
3. **Watch for cancellations** — a sold-out class refreshes availability over time. Poll `/classes/{id}` and grab the slot the moment it opens.
4. **Account / package management** — view credit balance, membership status, payment methods, achievements, metrics.
5. **Multi-tenant browsing** — a single user can be a member of multiple Mariana Tek tenants (boutique-fitness consumers often are); aggregate schedule across tenants.

## Table Stakes
From `bigkraig/marianatek` (Go community client — ClassSessions, Locations, PaymentOptions, Reservations, Users) and the documented Customer API:
- Class list + filter + single get
- Locations + regions
- Reservations CRUD (create / list mine / single / cancel)
- Payment options (per-class, credits, memberships)
- User account (`/me/account`, achievements, metrics)
- Credit cards (CRUD)
- Cart + checkout flows (regular cart + add-ons cart)
- Orders (history + cancel)
- Buy pages (purchase packages / credits / memberships)
- Appointments (1:1 services — separate from group classes)
- Brand config / theme / legal / app version metadata

## Data Layer
- **Primary entities**: `Tenant`, `ClassSession`, `Reservation`, `Location`, `Region`, `Instructor`, `Credit`, `Membership`, `PaymentOption`, `Cart`, `Order`, `Appointment`.
- **Sync cursor**: ClassSession is most useful with date-windowed sync (`start_date_after`, `start_date_before`).
- **FTS5 search**: across class names, instructors, locations, session types — power-user "earliest barre class at any location" queries.
- **Multi-tenant store**: per-tenant token rows + per-tenant cached schedules; one binary supports any tenant via `--tenant` flag.

## Codebase Intelligence
- **Source**: `bigkraig/marianatek` (Go, ~5-year-old reference implementation against Barry's Bootcamp tenant)
- **Auth**: `Authorization: Bearer {access-token}` via OAuth2 Authorization Code (or PKCE for public apps). Iframe widget at `{tenant}.marianaiframes.com/` uses a password-grant variant for direct login — to be confirmed by HAR.
- **Data model**: JSON:API spec (`application/vnd.api+json`) with `data`/`included`/`meta` envelope. Page-based pagination (`meta.pagination.{count,pages,page}`).
- **Rate limiting**: not documented; assume reasonable defaults, use `cliutil.AdaptiveLimiter` per-tenant.
- **Architecture insight**: Tenant subdomain IS the namespace — every API path is scoped to a single brand. The iframe widget is a UI shim; the API beneath is the documented Customer API.

## User Vision
- One CLI for Mariana Tek booking that works for any studio (kolmkontrast is the default tenant the user cares about, but Barry's, Y7, CycleBar, Solidcore, etc. all share the platform).
- Watch-for-cancellation is the killer feature — sold-out sessions in popular slots are the everyday pain point.
- Refresh-token to disk via `marianatek login` (not 1Password Environment — the CLI is going public; per-user auth flow is the standard).
- Public CLI: no personal notification hooks. Watch emits structured stdout for users to pipe into their own flow.

## Product Thesis
- **Name**: `marianatek` (slug). Binary: `marianatek-pp-cli`. MCP: `marianatek-pp-mcp`.
- **Headline**: "Every Mariana Tek booking feature, plus multi-tenant search, watch-for-cancellation, and an offline class catalog no other tool has."
- **Why it should exist**: Mariana Tek powers hundreds of boutique-fitness/wellness studios but has zero public CLIs and no MCP server. Members repeatedly hit the same sold-out class problem (no waitlist signal) and book the same regulars (no shortcut). One typed `--json`-emitting CLI with a local SQLite cache and cancellation watcher closes both gaps and unlocks agentic use cases (Claude scheduling around an existing calendar).

## Build Priorities
1. **Priority 0 — foundation**: OAuth login flow (interactive `marianatek login --tenant <slug>` opens browser, captures auth code, exchanges for tokens, writes refresh token to `~/.config/marianatek/<tenant>.token.json` with 0600). Multi-tenant config. SQLite store schema covering all primary entities. Sync command (schedule + reservations + account).
2. **Priority 1 — absorb**: every Customer API endpoint mapped to a typed Cobra command. Cart + checkout. Buy pages. Appointments. Metrics. Account management. Credit card CRUD. Order history.
3. **Priority 2 — transcend** (these only work because of the SQLite store + multi-tenant model):
   - `marianatek watch <class-id|filter> --interval 60s` — poll until a sold-out class opens, auto-book or alert via structured stdout.
   - `marianatek schedule --any-tenant --type "vinyasa" --before "07:00"` — cross-tenant filtering (only works if you sync multiple tenants).
   - `marianatek regulars` — "instructors / class types / time-of-day I usually book" derived from local reservation history (no API offers this aggregation).
   - `marianatek expiring --within 30d` — credits / memberships about to expire that the API surfaces as raw dates but never aggregates.
   - `marianatek conflicts <date>` — flag overlapping reservations + buffer-time conflicts across tenants.
4. **Priority 3 — polish**: enrich generated flag descriptions (Mariana Tek's spec uses terse fields like `start_after`; rewrite as "earliest class start time (ISO8601)"), MCP read-only annotations on every list/get command, MCP intents for the top compound workflows.
