# Bookclicker CLI Brief

## API Identity

- **Domain:** Author newsletter-promotion marketplace. Authors swap, sell, and buy newsletter spots to promote books.
- **Users:** Indie authors (heavily romance/genre fiction), pen-name operators, and promo services running one or more mailing lists.
- **Data profile:** Transactional marketplace over a calendar. Reservations (swap or paid) against dated inventory on mailing lists, scoped to pen names and books. Newsletter performance metrics (platform, open rate, click rate) are surfaced per list.
- **Stack:** Ruby on Rails (Phusion Passenger), jQuery + Bootstrap 3, Sprockets asset pipeline, Cloudflare in front, Stripe for payments. Session auth via `_bookclicker_session` cookie plus Rails CSRF `authenticity_token`. No SPA framework.
- **Surface:** ~35 internal JSON endpoints under `/api/` discovered in the public asset bundle (`application-<hash>.js`). No published docs, no OpenAPI, no developer portal.

## Reachability Risk

- **Low-to-moderate.** Public pages return HTTP 200 through Cloudflare with no challenge on plain curl. The `/api/` surface is session-gated, so reachability for real work depends on an authenticated cookie, not on bot mitigation.
- No community wrappers exist to inspect for known breakage (see Ecosystem below), so there is no issue history to mine.
- **Contract risk:** these are internal endpoints with no compatibility guarantee. A Bookclicker redesign can change them without notice. Regeneration is the remedy.

## Terms-of-Use Constraint (flagged, not resolved)

Bookclicker's Acceptable Use policy contains no anti-scraping, anti-crawler, or anti-automation clause. It does contain, in a list of prohibited conduct:

> "Decipher, decompile, disassemble, or reverse engineer any of the software on our Website, or in any way used or downloaded from the Website."
> "Use any of the software on our Website, or downloaded from the Website, to create a competing product."

Reading the site's own network traffic from the operator's authenticated session to drive the operator's own account is not obviously "decompiling software," and this CLI is not a competing product. But the clause is broad and explicit, and a strict reading covers reverse-engineering the internal API. **This is the user's call to make; it has been surfaced.** The build proceeds under the user's direction, single-account, at human pace, with no redistribution of captured endpoints implied.

Separately: signup requires the user's **third-party newsletter-provider API key** (MailerLite, etc.), which Bookclicker stores encrypted. That credential must never be captured into any artifact during discovery.

## Top Workflows

1. **Plan and fill a launch.** Create a book launch, filter mailing lists by genre, open a list's calendar, pick an open (green) date, choose promo type, send offer. Repeat across many lists until the launch window is covered. This is the highest-volume, highest-tedium workflow in the product.
2. **Confirm sent promos.** After a promo runs, Bookclicker registers it a few days later and the user must select which newsletter was sent to confirm it. Explicitly called out in the FAQ as a recurring chore on the home page.
3. **Handle inbound offers.** Accept or decline incoming swap/paid requests against the user's own list inventory.
4. **Maintain sellable inventory.** Mark lists For Sale, set promo types and prices, manage per-date availability.
5. **Maintain identity graph.** Pen names (Amazon-verified / non-verified / promo service), books under each pen name, mailing-list integrations and their health.

## Table Stakes

There is no competing tool, so table stakes are defined by the Bookclicker web UI itself:

- Browse and filter mailing lists by genre
- Per-list calendar availability (open vs booked dates)
- Send offer (swap or paid), with promo type Solo / Feature / Mention
- Accept / decline / cancel reservations (buyer-side and seller-side bulk cancel both exist as endpoints)
- Pen name CRUD, book CRUD under a pen name
- Mailing list listing, For Sale toggling, subscription counts
- Integration health ("whether they are working")
- Confirm promos queue
- Conversations / messaging with counterparties
- Payment method management via Stripe
- Assistant accounts (`users_assistants`, invite) — delegate access to a VA

## Promotion Model (domain rules that must be encoded)

- **Solo** — one book only, nothing else in that send. Max **1** per newsletter.
- **Feature** — book featured alongside the author's own or unpaid books. Max **1** per newsletter.
- **Mention** — hyperlink mention. Up to **9** per newsletter.
- **Swap** = reciprocal, free, two dates. **Paid** = one-directional, Stripe-settled.

These caps are inventory constraints. A CLI that understands them can compute remaining capacity per newsletter per date; the UI makes the user reason it out per calendar screen.

## Data Layer

- **Primary entities:** `reservations` (the core transaction), `lists` (with platform / open rate / click rate / genre), `pen_names`, `books`, `one_day_inventories` (dated availability), `confirm_promos` (pending confirmations), `conversations`, `integrations`.
- **High gravity:** `reservations` — every row is a dated, typed, priced, counterparty-linked event. This is the table everything else joins to.
- **Sync cursor:** reservation updated/created timestamp; calendar availability by date window.
- **FTS/search:** books (title), lists (name, genre), pen names.

## Ecosystem (absorb input)

**Nothing exists.** Verified across:
- GitHub repo search `bookclicker` — 0 results
- GitHub code search `bookclicker.com/api` — 0 results
- npm search `bookclicker` — 0 results
- Web search for MCP servers / Claude skills / CLIs / automation scripts — 0 results

The absorb manifest's Layer 1 is therefore empty of third-party tools. Absorbed features are drawn from the web UI's own surface. Nearly all differentiation comes from Layer 2 (transcendence).

Adjacent products (not integrations, context only): StoryOrigin and BookFunnel both run newsletter swaps and group promos for indie authors. Neither exposes Bookclicker's paid-inventory-on-a-calendar model, and neither offers a CLI.

## Non-Obvious Insight

> **Bookclicker isn't a newsletter swap marketplace. It's a promotional reach ledger.**
> Every reservation is a dated bet on a partner's list, and every open rate, click rate, and confirmed send is a signal about which partners actually move books — and which ones quietly consume launch slots and give nothing back.

The web UI is built for booking one promo at a time. It has no memory across launches. It cannot answer "which partners have I swapped with three times who have never reciprocated," or "which genre lists actually converted for my last four releases," because those questions require history joined across reservations, lists, and books. Once that history is in local SQLite, they become one query.

## Product Thesis

- **Name:** `bookclicker-pp-cli`
- **Why it should exist:** Filling a launch on Bookclicker means opening one calendar at a time across dozens of lists, eyeballing green squares, and sending offers one by one — then remembering to come back days later to confirm each send. The CLI collapses the search-and-book loop into a single filtered query across every candidate list, keeps a local mirror so partner performance accumulates across launches, and turns the confirm-promos chore into one command. For an agent, it turns a click-heavy marketplace into a scriptable one.

## Build Priorities

1. **Data layer + sync** for reservations, lists, pen names, books, inventories. Nothing else works without it.
2. **Discovery and booking**: cross-list availability search filtered by genre/date/price, then send offer.
3. **Confirm-promos queue** — highest-frequency recurring chore, trivially automatable.
4. **Reciprocity and partner performance** — the transcendence layer the UI cannot do.
5. **Inventory management** — For Sale toggles, pricing, per-date capacity against Solo/Feature/Mention caps.
