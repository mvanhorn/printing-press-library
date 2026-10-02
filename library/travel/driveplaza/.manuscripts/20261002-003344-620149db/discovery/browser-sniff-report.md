# Drive Plaza browser discovery provenance

## User Goal Flow
Resolve an interchange and compare a planned expressway journey with the provider's prices and timing assumptions. Previous builder used native Chrome to submit the English route form (nerima → sendai-minami), browse road-based directional SA/PA lists and HASUDA-SA UP detail, and browse the Japanese traffic portal. This resumed builder opened the official planned-restriction page and verified its construction and ETC-lane handoffs. All planned read-only flows completed; network event capture for the earlier route flow yielded no events and was not fabricated.

## Pages & Interactions
1. https://en.driveplaza.com/dp/SearchTopEN — inspected vehicle, priority, departure/arrival, explicit date/time, waypoint fields and submitted nerima → sendai-minami.
2. https://en.driveplaza.com/dp/SearchQuickEN — actual returned route alternatives, toll columns, distance and distinct traffic timing.
3. https://en.driveplaza.com/dp/SAPAServiceEN — inspected road and facility filter controls; lowercase hasuda and Japanese 蓮田 produced no English name results.
4. https://en.driveplaza.com/dp/SAPAServResEN?HIGHWAY=1040&AREA= — selected Tohoku road; directional results succeeded.
5. https://en.driveplaza.com/sapa/1040/1040021/1/ — HASUDA-SA UP facility sections and weekday hours.
6. https://www.driveplaza.com/traffic/ — traffic news, RSS and scheduled-work links.
7. https://www.driveplaza.com/traffic/roadinfo/schedule/ — resumed native Chrome navigation; actual rendered headings 東日本/中日本/西日本 and official construction/lane handoffs verified. CDP document GET and response HTTP 200/text-html captured on reload (sequence145/149/208). Browser automation was unusually slow during that reload; further runtime checks use HTTP.

## Browser-Sniff Configuration
Native built-in browser API through Chrome extension, isolated agent tabs. IAB was unavailable, so Chrome was the fallback. No desktop control, Playwright or authenticated session. Source inspection is temporary; runtime uses plain Go HTTP. No proxy envelopes. No artificial pacing or concurrent browser controls.

## Endpoints Discovered
| Method | Public source | Status | Content |
|---|---|---|---|
| GET | en.driveplaza.com/dp/SearchTopEN | 200 | HTML route form |
| GET | en.driveplaza.com/dp/SearchQuickEN | 200 | HTML quote with alternative panels |
| GET | en.driveplaza.com/community/icsearch_api.php | 200 | XML IcItem Code/Name/RoadNo/RoadName |
| GET | www.driveplaza.com/community/icsearch_api.php | 200 | XML Japanese names/roads |
| GET | www.driveplaza.com/community/icsearch_fromcode_api.php | 200 | XML Japanese name by code |
| GET | en.driveplaza.com/dp/SAPAServiceEN | 200 | HTML road/facility catalog |
| GET | en.driveplaza.com/dp/SAPAServResEN | 200 | HTML directional SA/PA list |
| GET | en.driveplaza.com/sapa/1040/1040021/1/ | 200 | HTML sections and hours |
| GET | www.driveplaza.com/cms/news/traffic.xml | 200 | RSS advisory notices |
| GET | www.driveplaza.com/traffic/roadinfo/schedule/ | 200 | HTML official planned-work handoffs |
| GET | en.driveplaza.com/community/icsearch_fromcode_api.php | 404 | Unsupported English code lookup |
| GET | en.driveplaza.com/dp/SearchOutsideEN | 404 | Unsupported code-entry endpoint despite source-script mention |

## Traffic Analysis
Only the schedule document's network events were captured in this resumed context. Earlier endpoint evidence is rendered forms and original builder's saved sanitized HTTP replay captures. traffic-analysis.json records this limited provenance explicitly; it is not a HAR. Response shapes are XML, RSS and structured server-rendered HTML. GET parameter meaning comes from actual form labels: carType0..4, priority1 distance/2 time/3 toll, kind1 departure/2 arrival, searchYear/Month/Day/Hour/Minute, five keiyuPlaceKana inputs. No login, WAF or CAPTCHA observed. Default sandbox DNS failure was an execution-environment limitation; escalated public HTTP succeeded without cookies. Generation seeds the first-party page commands; focused domain parsers are hand-coded.

## Coverage Analysis
Covered IC identity, road catalog, source quote alternatives, directional stop list/detail, advisory feed and planned-work handoffs. Unsupported English code route endpoint is excluded; quote entry uses exact English interchange names from the lookup. No comprehensive active-closure feed or live-current-traffic extraction is claimed. Non-East SA/PA records have an explicit provider warning dated 2006-03-31; facility weekday hours can vary on holidays. Route considering-traffic times are provider estimates for the requested date, not a claim of live-now traffic.

## Response Samples
XML: `<NexcoIC><IcItem><Code>1800001</Code><Name>NERIMA</Name><RoadNo>1800</RoadNo><RoadName>【E17】Kan-Etsu Expressway</RoadName></IcItem></NexcoIC>` (representative fields; see saved ic-en.xml).
HTML route: source summary table columns Standard toll, ETC toll, ETC2.0 toll, ignoring traffic, considering traffic, Distance. 2026-10-10 08:00 standard sample route1:8490/7970/7970 JPY,213/273 minutes,345.2km (see route.html). Rendered 2026-10-02 01:40 example had different ETC prices5580: do not cache or recompute locally.
RSS: see notices.xml, which includes解除 and延期 notices; title alone does not establish an active restriction.

## Rate Limiting Events
No429 observed. Discovery requests were serial or bounded read-only fetches. Runtime has bounded body size, request timeout and explicit request-count metadata.

## Authentication Context
No authenticated session used. Cookies and credential header values excluded. Public embed keys and jsessionids were stripped from saved HTML.

## Bundle Extraction
Saved public suggestIC.js/route-search.js establish XML/form wire names. English from-code and SearchOutsideEN paths mentioned in the bundle returned404 during direct verification and are not exposed as working quote endpoints.
