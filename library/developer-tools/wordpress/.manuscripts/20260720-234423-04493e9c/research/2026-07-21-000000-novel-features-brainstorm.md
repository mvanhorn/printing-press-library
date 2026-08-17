# WordPress CLI — Novel Features Brainstorm (Phase 1.5c.5 audit trail)

## Customer model

**Marisol — freelance agency maintainer, 31 client WordPress sites.**

*Today (without this CLI):* Marisol has no SSH on ~24 of her sites (cheap shared hosts,
cPanel only), so WP-CLI is off the table for most of the fleet. Her Monday morning is 31
browser tabs: log into `/wp-admin`, glance at the update nag, check Users →
Administrators for accounts she didn't create, note the WP core version, log out, next
site. She keeps a Google Sheet she updates by hand and that is stale by Wednesday. She
cannot answer "which of my sites are running an outdated core right now" or "which sites
have an admin account that isn't me or the client" without redoing the whole loop.

*Weekly ritual:* Monday fleet sweep — core/plugin/theme update status, admin user
inventory, and "is the site even up and is REST reachable." Then a batch of
update-and-pray on whichever sites look worst.

*Frustration:* There is no fleet-level question she can ask. Every answer requires N
logins, and by the time she reaches site 31 the answer for site 1 is already stale.

**Devin — headless front-end developer, Next.js reading `/wp-json/`.**

*Today (without this CLI):* Devin lives with `/wp-json/wp/v2/posts?_embed=1` open in one
tab and the theme's `functions.php` in another. When the client's editor adds a "Case
Study" custom post type or an ACF field and it doesn't appear in his Next.js build, he
starts the same manual forensics every time: curl `/wp-json/`, eyeball the giant route
table for a `case-study` route, curl `OPTIONS /wp/v2/posts` and squint at the
29-property schema, then curl an actual record to see whether the `meta` key is
present-but-empty or absent entirely. The Go clients he tried (`sogko`, `robbiet480`)
model post types as fixed structs and silently drop the CPT, so they cannot even tell him
it exists.

*Weekly ritual:* Every content-model change from the WP side means re-verifying what the
REST surface actually exposes before he writes a fetcher against it — and re-verifying
after every plugin update, because plugins add and remove namespaces.

*Frustration:* The gap between "registered in WordPress" and "actually visible in REST"
(`show_in_rest`) is invisible. He finds out at build time, from an `undefined` in a
template, not from any tool.

**Priya — content ops manager, one high-volume editorial site (1000+ posts).**

*Today (without this CLI):* Priya works the wp-admin Posts list with the status filter,
one status at a time: Drafts, then Pending, then Scheduled. The list view paginates at
20, sorts by date, and cannot tell her how *long* something has been sitting in Pending.
She keeps a separate mental list of "posts that went out without a featured image" and
finds them by scrolling the front end. Uncategorized posts she finds by clicking the
Uncategorized term link and hoping.

*Weekly ritual:* Friday queue review — what's scheduled for next week, what's rotting in
pending, what shipped this week with something missing (no featured image, no category,
no excerpt).

*Frustration:* Age. wp-admin shows a date, never a duration, and never across statuses at
once. "What has been stuck in pending for more than three weeks" is a question she
answers by hand, every week.

**Sam — automation builder wiring an agent/cron to publish into client sites.**

*Today (without this CLI):* Sam has an Application Password and a script. He finds out
what that credential can actually do by attempting a write and reading the 401/403 — in
production, on a client's site. He's been burned by hosts that strip the `Authorization`
header (the request looks unauthenticated, so WordPress returns a perfectly polite 401
that tells him nothing about the real cause) and by security plugins that restrict
`/wp-json` for some routes but not others. His fallback debugging tool is "try it and see
what breaks."

*Weekly ritual:* Onboard or re-verify credentials against a site before pointing an
unattended job at it, and triage the previous week's job failures back to a cause (bad
creds vs. stripped header vs. plugin block vs. missing capability).

*Frustration:* There is no way to ask "what may this credential do here" without mutating
something. Discovery of permissions is only possible by causing side effects.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline verdict |
|---|------|---------|-------------|---------|--------|----------------|
| 1 | Fleet rollup | `fleet` | One table across every site in the local store: core version, pending plugin/theme updates, admin-account count, last sync age, last known REST status. | Marisol | (a) | **KEEP** — pure local cross-site join over per-site-keyed stores; no LLM, no external service. |
| 2 | REST reachability diagnosis | `diagnose` | Classify why a site's REST API misbehaves → one named cause. | Marisol, Sam | (a) | **KEEP** — every probe is in the spec; classification is a mechanical decision table. |
| 3 | Credential capability audit | `caps` | Walk the live route table and report, per route, what the current credentials may do — from the `Allow` header, zero mutations. | Sam, Marisol | (a) | **KEEP** — only non-destructive permission discovery that exists. |
| 4 | Editorial queue with age | `queue` | Drafts/pending/future bucketed by age in state, plus the coming week's schedule. | Priya | (a) | **KEEP** — local SQLite gives the duration math wp-admin refuses to compute. |
| 5 | Content hygiene audit | `audit` | Posts with no featured image, no category, no excerpt, no tags. | Priya, Marisol | (c) | **KEEP** — local join; named verbatim in brief Top Workflow #2. |
| 6 | Orphaned media | `orphans` | Media referenced by no content and set as no post's featured image. | Priya, Marisol | (c) | **KEEP** — impossible in any single API call. |
| 7 | REST visibility gap | `schema <type>` | Reconcile types registry × route table × OPTIONS schema × synced rows. | Devin | (a) | **KEEP** — the `show_in_rest` blind spot made visible. |
| 8 | Route-table drift | `drift <siteA> <siteB>` | Diff two sites' route tables. | Devin, Marisol | (b) | KEEP for Pass 3 — weekly-use doubtful. |
| 9 | Taxonomy hygiene | `terms audit` | Zero-post terms, near-duplicate slugs. | Priya | (b) | KEEP for Pass 3 — largely a subset of #5. |
| 10 | Internal link audit | `links` | Same-host hrefs resolved against local slugs. | Priya, Devin | (c) | KEEP for Pass 3 — second regex-over-content command. |
| 11 | Revision churn | `revisions churn` | Most-revised posts, per-author revision counts. | Priya | (b) | KEEP for Pass 3 — expensive fan-out sync. |
| 12 | Block usage census | `blocks usage` | Block-type frequency from `<!-- wp:name -->` delimiters. | Devin, Priya | (b) | KEEP for Pass 3 — one-time migration question. |
| 13 | Moderation triage | `moderate` | Hold/spam queue joined with per-author history. | Priya | (b) | KEEP for Pass 3 — many sites have comments off. |
| 14 | Author output stats | `authors` | Per-author counts, cadence, draft→publish lag. | Priya | (c) | **CUT NOW** — framework already emits `analytics --type posts --group-by author`. |
| 15 | Revision restore | `restore <post> --revision <id>` | PUT a revision's content back onto the parent. | Priya | (b) | **CUT NOW** — thin compound wrapper, no local leverage. |
| 16 | Stale content sweep | `stale` | Published posts not modified in N months. | Priya | (c) | **CUT NOW** — needs traffic data outside the spec. |

**Rubric checks applied to the whole set:** no candidate requires an LLM. No candidate
calls a service outside the spec (#16 died on exactly that). All read-side candidates use
the same Application Password as `sync`; `caps` and `diagnose` are read-only by
construction and work *better* with degraded credentials, which is the point. No candidate
is a dashboard or daemon. Every candidate is verifiable in dogfood against a real site.

## Survivors and kills

### Pass 3 force-answers

**1 `fleet`** — Weekly: yes, this *is* Marisol's Monday. Wrapper: no — WordPress has no
concept of a fleet. Transcendence: cross-site join over per-site-keyed SQLite. Sibling
kill: `drift`, a two-site quarterly question vs. an N-site weekly one. Buildability:
`hand-code`.

**2 `diagnose`** — Weekly: yes; at 31 sites something breaks weekly. Wrapper: no — a
decision table over four probes plus a transport fallback. Transcendence: service-specific
failure pattern that exists nowhere else. Sibling kill: `terms audit`. Buildability:
`hand-code`.

**3 `caps`** — Weekly: yes, on every onboarding and failure triage. Wrapper: no — the
feature is the walk across the live route table plus the auth/no-auth delta.
Transcendence: WordPress's `Allow` header semantics make zero-mutation permission
discovery possible, and nothing on the market uses it. Sibling kill: `restore`.
Buildability: `hand-code`.

**4 `queue`** — Weekly: yes, Friday review. Wrapper: no — the API returns rows, not
*how long* they have been pending, bucketed, across three statuses. Transcendence: local
duration math. Sibling kill: `revisions churn`. Buildability: `hand-code`.

**5 `audit`** — Weekly: yes; brief Top Workflow #2 verbatim. Wrapper: no — four checks,
three requiring a join the API cannot express. Sibling kill: `terms audit` and `links`.
Buildability: `hand-code`.

**6 `orphans`** — Weekly: borderline-but-yes for Marisol (billing clients for storage,
hunting bloat on shared hosting). Wrapper: no — WordPress has no "unused media" endpoint.
Sibling kill: `links`. Buildability: `hand-code`.

**7 `schema`** — Weekly: yes, on every content-model or plugin change. Wrapper: no — a
three-way set difference. Transcendence: exploits the self-describing route table, the
defining structural fact of this API. Sibling kill: `blocks usage`. Buildability:
`hand-code`.

### Survivors

See the transcendence table in the absorb manifest.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Route-table drift (`drift`) | Genuinely unique but a per-migration question, not weekly — soft kill on frequency. | `fleet`, `schema` |
| Taxonomy hygiene (`terms audit`) | Its two useful checks are a subset of the join `audit` already performs. | `audit` |
| Internal link audit (`links`) | Reuses `orphans`' machinery for a lower-value question. | `orphans` |
| Revision churn (`revisions churn`) | Expensive per-parent revision fan-out for a report no persona runs weekly. | `queue` |
| Block usage census (`blocks usage`) | One-time migration/theme-audit question rather than a ritual. | `schema` |
| Moderation triage (`moderate`) | Value collapses on the many editorial and headless sites with comments disabled. | `queue` |
| Author output stats (`authors`) | Reimplements `analytics --type posts --group-by author`. | `queue` |
| Revision restore (`restore`) | Thin two-call wrapper over spec endpoints. | `queue` |
| Stale content sweep (`stale`) | "Old" is not "stale" without traffic data, which is outside the spec. | `audit` |
