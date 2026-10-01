# OMAKASE CLI brief

## API Identity
OMAKASE byGMO, https://omakase.in/en. Public Rails-rendered restaurant catalogue and detail pages; no published public API identified. Only this provider and its Japanese /r variant are integrated. No API keys or account are required for public reads.

## Users and Workflows
Travel planners shortlist premium restaurants by name, region and cuisine, inspect courses and dining rules, distinguish release timing from bookable seats, compare public fees and cancellation exposure, and hand off canonical booking URLs. Pain points: confusing release with availability, minimum/variable course prices, and membership-only tools.

## Current access and economics
Verified anonymous browser and Go HTTP reads. Go GET with an identified User-Agent plus Accept returns 200 on catalogue, English/Japanese Amamoto, Quintessence, ume, and name search. Bare curl and bare Go receive Cloudflare 403; detection must fail explicitly on challenges. No browser runtime or paid transport is needed. Public detail displays Log in to check availability. Exact date/party seats are not publicly exposed and will return unknown with login_required; advanced search, seat labels and release calendars are Premium Green. Premium Green public plans show JPY 4980/month and 49980/year including tax; Gold invitation-only price undisclosed. Registration/payment is not implemented.

## Reachability Risk
Medium: undocumented HTML can change and Cloudflare can block requests. No arbitrary /api or reservation endpoints will be guessed. Fail closed on login/challenge/missing structure. robots.txt allows public pages, excludes APIs, user and reservation paths and indexed query crawling; commands are bounded user-directed reads. No background crawl.

## Table stakes and alternatives
Official site supplies area/cuisine/name search, paginated cards, courses, release rows, cancellation policy and fee text. TableCheck and Pocket Concierge offer similar planning but are not integrated. Focused searches for omakase.in CLI, MCP, npm and PyPI found no relevant maintained wrapper or documented API; generic Omakase projects and company-data MCPs do not cover this provider. No wrapper issues applicable.

## Data Layer
Restaurant slug is source ID. Local JSON inventory contains summary cards only, refreshed explicitly (default one page, maximum 25 pages). Detail is lazy; bounded normalized response cache has separate freshness and offline semantics. Japanese names retrieved only on detail, never guessed.

## Product Thesis
omakase-pp-cli: concise, source-grounded planning with honest reservation states and no bookings or payments.

## Build Priorities
1. restaurants find/show and source filter discovery, bounded pagination and compact JSON/projection.
2. courses, release and availability: exact source fields, JPY per guest, tax/floor flags, separate reservation and service fees, cancellation and eligibility text.
3. compare bounded IDs, explicit partial errors, inventory refresh/find/status, live membership/capability report.
4. deterministic state/price/release/parser tests, high-signal live relevance/source correctness and cold/warm resource measurements.

## Authorization
User preapproved ordinary gates and focused public scope, sole implementation. Novel brainstorming is performed by builder because user explicitly forbids all extra workers; exactly one fresh-context independent code reviewer will be spawned after implementation. Shared/global upgrades skipped per explicit unchanged-config constraint.
