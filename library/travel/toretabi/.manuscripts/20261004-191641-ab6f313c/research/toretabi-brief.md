# Toretabi ticket CLI brief

## API identity and users
Primary discovery source: https://www.toretabi.jp/ticket/. Public Japanese HTML operated by 交通新聞社. This is an editorial ticket directory, not an official inventory API. Main users compare a small regional rail-ticket shortlist with bounded linked official operator rule inspection. Stable ticket leaf IDs, native area/type filters, numbered listings, detail period tables and operator links form the contract. No credential or browser runtime is required.

## Reachability and source evidence
Ordinary anonymous GET listing repeated HTTP200; tokai_043, east_027, hokkaido_028 and page/2 plus area=1&class=4 returned full HTML HTTP200 on 2026-10-04. Sandbox DNS first refused network; ordinary escalated public HTTP worked. There was no provider challenge or access bypass. The filtered listing gave six Hokkaido free-ticket candidates. Five numbered pages advertised on the unfiltered listing. Exact limits and source HTML can change. Source clocks are retrieval only; no publisher update timestamp found.

## Customer model and pains
Domestic and visiting travelers need the Japanese ticket name, date rules and purchase prerequisites more than a generic page dump. They must distinguish sales from use deadlines, inspect a fare supplement before boarding a tourist/limited-express train, and confirm age/airline/weekday restrictions. An operator handoff is necessary because listed prices often are absent and static coverage does not prove current sale or seat stock. Travelers also need a saved bounded shortlist for offline re-reading without losing observation times.
Concrete cases: tokai_043 excludes のぞみ and some private-railway tourist/event trains, requires separately bought supplements, and names JR sales channels. east_027 has a 2027 sale/use deadline, Friday/Saturday/holiday rules and a purchase-channel exception that ended 2026-09-30. hokkaido_028 has U25 document requirements, blackout month/day spans and airline vouchers worth 1,000 JPY; the voucher value is not the ticket price.

## Top workflows and table stakes
1. Bounded native area/type and Japanese keyword discovery with explicit listing routes/counts/continuation.
2. Inspect sale/use/validity, eligibility, channels, supplements/exceptions and linked operator URLs for one ID.
3. Compare at most four detail-inspected tickets at a travel date and sale-check date; proven exclusions, conditional rules and unknowns stay distinct.
4. Re-read normalized cached observations offline with their original retrieval clocks.

## Ecosystem and incumbent
Exact Toretabi CLI/MCP/plugin/SDK/wrapper searches on GitHub/npm/PyPI/community MCP directories found no relevant integration. No wrapper issue tracker exists to check. NAVITIME passes list advertises pass IDs and verification for route queries; it does not model Toretabi sale/use windows or source purchase/channel exceptions. Extending NAVITIME would mix editorial ticket evidence with route transport and its pass IDs, so use a separate small source CLI. JR operator ticket pages are inspected through links already supplied by Toretabi details, within get/compare; no separate operator integration or command surface is added.

## Data layer
Bounded normalized SQLite observations, maximum 100 tickets, each payload <=32 KiB, original publisher/operator observed_at clocks retained separately. No full-site copy or raw HTML stored in user cache. Ordinary SQLite transactions; explicit offline reads never refresh or migrate. Refresh is requested through live reads; provider operations GET only. No unrelated framework or adversarial filesystem redesign.

## Product thesis and scope
Toretabi: compare published regional ticket conditions while keeping source uncertainty visible. Four domain commands: tickets list, tickets get, tickets compare, tickets cached. Listing output includes native area/type facets. Five additional behaviors in these paths: bounded coverage auditing, date evidence comparison, supplement/exception grouping, eligibility/channel evidence, and timestamped offline shortlists. Japanese names/evidence authoritative; evidence snippets bounded. Publisher and operator prices stay separate; operator fare tables retain passenger/category/unit/currency-basis evidence; voucher values never become fares. Dates normalize only explicit year-qualified intervals/endpoints. Holiday, blackout and compound calendar rules remain conditional unless a date is excluded by an explicit year-qualified range.
Tourist-train operating timetables, fares absent from both sources, seats, payments, bookings, messages, pass-holder totals and article archives outside /ticket/ are unsupported. Closed sales are distinct from unavailable seats, sold-out, unknown and unsupported; source omission cannot invent any state. Expired year-qualified use windows are labelled archival evidence relative to the requested as-of date.

## Build priorities
Parse details and bounded linked official operator evidence first; source bounds and errors second; conservative comparison third; offline normalized cache fourth. HTTP timeout 20s, maximum 512 KiB per page, at most 5 listing pages/4 details, no arbitrary user URL input. Stdout compact JSON with projection and stderr diagnostics. Go/SQLite dependencies reuse shared proven pin, Go 1.27.1, modernc/sqlite 1.60.1 with libc 1.77.1.

## User vision and authorization
Detailed source scope, read-only implementation, isolated paths, install and fork PR publication are authorized by the root user briefing. No repeated setup/menu, nested exec/coordinators or subagent spawning. Root assigns one fresh reviewer for phases14–17. No publication for blocked or zero-success work.

## Restored current-rule verification
Live get/compare follow one publisher-linked official JR document, with supported hosts, GET-only bounds and separate observations. Real JR Central/JR Hokkaido HTML proves current edition, explicit periods, fare categories, purchase/channel/U25 rules and exclusions; no fetch/title result confirms all rules. Date/validity checks corroborate or conflict; unknowns remain unknown. PDF text extraction, inaccessible/mismatched/generic HTML and unsupported hosts are disclosed. Repeated ordinary public reads and bounded original evidence are recorded in proofs/operator-live-matrix.json.
