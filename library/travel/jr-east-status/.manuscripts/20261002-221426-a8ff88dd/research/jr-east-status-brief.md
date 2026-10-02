# JR East Status CLI brief

## API identity and user vision
The authorized product is a focused, anonymous, read-only Japan travel status CLI. JR East's English and Japanese public pages are the primary sources, not an officially documented API. Category: travel. Native Chrome exposed five area/service summaries, detailed status rows, planned construction notices and published certificate links.

## Customer model and workflows
A traveler checks a Shinkansen and connecting local line before leaving; a commuter needs the affected direction and section; an agent compares an itinerary's component lines while preserving unknowns. Today users must open separate regions and reconcile machine-translated notices. The English Koumi planned-closure sentence visibly scrambles dates and station names, so Japanese source facts must remain available.
1. Discover regions and source-native line IDs with Japanese and English names.
2. Inspect multiple disruption rows, directions, sections and source timestamps for one line.
3. Compare up to eight itinerary lines, grouping requests by region and preserving stale/error/outside-hours states.
4. Find construction notices and official handoffs, retaining source dates rather than inventing schedule guarantees.
5. Hand off to a currently published delay certificate for a covered line/time slot, distinguishing route maximum from an individual train's delay.

## Source contract and reachability
GET /train_info/e/{kanto,tohoku,shinetsu,chyokyori,shinkansen}.aspx: English server-rendered .rosenBox rows; .current_time timestamps. GET /train_info/{region}.aspx: Japanese server-rendered .traininfo-routes__table__item rows, source line.aspx?gid=&lineid= links and status CSS classes. Express summaries link to express.aspx?group= service details. Shinkansen identity comes from source CSS line identifiers when there is no detail link.
GET /train_info/e/infotop.aspx: public document.write HTML fragments for five area summaries, no page update timestamp. Parse without JavaScript execution.
GET /delay_certificate/ and /delay_certificate/e/: current certificate tables, published pop.aspx?D=YYYYMMDD&R=route&T=slot links. Japanese rows expose source line IDs. GET /delay_certificate/e/rosen.html gives section coverage and through-service routing.
GET https://www.jreast.co.jp/suspend/: planned-work sections and date/section tables; direct handoff for complex schedules.
The browser-observed /traininfomulti/kanto.json feed contains a dummy notice record and is not operational line data. Do not mistake its HTTP 200 for a status-data proof.
Raw fetch-docs/curl and default Go User-Agent received Akamai 403 Access Denied for some status paths. Press probe-reachability (stdlib and Surf) returned 200. Standard Go HTTP with identifiable jr-east-status-pp-cli User-Agent plus normal Accept headers then returned real HTML, without credentials, cookies, TLS impersonation or a running browser. All selected public pages returned 200 in actual replay. Network sandbox DNS failures were tool restrictions, not provider behavior.

## Coverage and consequential semantics
English: 04:00 through 02:00 following day JST, anticipated/actual delays in excess of 30 minutes. Japanese says 30 minutes or more; retain that language discrepancy. At 02:00–04:00 reporting is closed. A normal label/no notice never proves zero delay. BRT reports long suspensions only. Conventional limited expresses belong to the express/night-train service region.
Current-as-of is a source page timestamp, not necessarily the event update or a real-time train position. Compare source timestamp to observation; missing timestamp, future timestamps and stale snapshots are explicit. A fresh page with an old notice can describe a continuing disruption. Unknown affected direction/section remains unknown.
Certificates cover local conventional services, approximately five-minute-plus delays, rounded maximum per route/time slot; they neither establish an individual train delay nor prove boarding. Publication is confirmation-based near 07:00/10:00/16:00/21:00 and 02:00 following day. Dash or absent link is not a zero-delay proof. Current published links only in v1; no invented historical/date links. Sagami and Ome beyond Ome hand off to DOKOTORE. Through-services require choosing the actual segment's certificate.

## Ecosystem and table stakes
Focused searches of GitHub CLI/MCP/plugins, npm/PyPI SDKs and traininfomulti code found no relevant maintained integration to absorb. No top-wrapper issue check applies when no wrapper exists. First-party JR East web/app and DOKOTORE provide statuses/official handoffs; exact train positions, timetables and their other features are outside the approved focused product. No competitive superiority claims.
Table stakes: regional discovery, native IDs, multiple notices per line, normal/suspension/cancellation labels, bilingual evidence, current certificate handoffs, concise structured output, source links.

## Data layer and boundedness
Operational reads are live read-through, with no persistent status cache masquerading as current. Generated local learning is available for reusable line vocabulary only. Each command caps inputs, request count, body size, deadline and result rows; no notifications, subscriptions, ticket/account changes or paid service dependencies. Output derives short facts instead of republishing articles or entire status messages.

## Product thesis and build priorities
Name: JR East Status. Compare JR East line impacts with source freshness, bilingual identity and explicit reporting limits.
Build areas, lines, status, impact, planned, certificates and coverage; retain a source-guide link command. Native browser discovery is temporary; runtime is anonymous standard HTTP. Deterministic tests focus on language identity, duplicate groups/rows, reporting midnight rollover, stale/malformed/error pages, planned-note extraction and certificate link semantics. Live tests must prove real parser behavior on all service regions.

## Approval record
The user preauthorized routine research, scope/category decisions and all build/local-promotion work in the batch brief. The user limited delegation to exactly one fresh-context MAX reviewer per CLI; this overrides the skill's extra novel-feature agent step. Novel candidate/adversarial evaluation is performed directly and documented in the manifest. No additional gate question is required.
