# Drive Plaza CLI brief

Source: NEXCO East public English Drive Plaza and same-provider Japanese pages. No auth, purchases or accounts. Native browser discovery used isolated Chrome tabs after IAB was unavailable; rendered route search succeeds, and stdlib-compatible HTTP repeats it without cookies.

Product: source-backed Japan expressway planning with stable IC/road identities, route alternatives, base versus conditional ETC/ETC2.0 estimates, planned JST travel times, road-specific rest stops and traffic advisory handoffs.

Workflows: (1) resolve an ambiguous interchange by name/road/code and preserve Japanese name; (2) compare all source route alternatives for five vehicle classes, explicit departure/arrival JST date and 10-minute time; (3) discover roads and directional SA/PA with facility filters and hours detail; (4) read public NEXCO East traffic notices and planned closure links; (5) open canonical route, forecast and live-traffic handoffs.

Source contracts: /community/icsearch_api.php is XML with IcItem Code, Name, Yomi, Type, RoadNo, RoadName. English SearchQuickEN HTML GET accepts startPlaceKana/arrivePlaceKana, carType 0..4, priority 1 distance/2 time/3 toll, kind 1 departure/2 arrival, searchYear/Month/Day/Hour/Minute, up to five keiyuPlaceKana waypoints. SearchOutsideEN supports IC codes. Result HTML contains summary table, separate route panels and source estimates ignoring/considering traffic. No local discount formulas: preserve quoted prices and conditional caveats.

SAPA: SAPAServiceEN exposes road codes and facility ITEM enums, SAPAServResEN GET returns directional identities and icon availability classes. English lowercase hasuda and Japanese 蓮田 both returned none; road code 1040 succeeded. Uppercase canonical-name behavior has not been verified. Use road codes until verified. Detail /sapa/{road}/{area}/{1|2}/ contains facilities and weekday hours. Search note explicitly says non-NEXCO-East data dates to 2006-03-31 and facility hours vary on holidays.

Traffic: /traffic/ links /cms/news/traffic.xml RSS and /traffic/roadinfo/schedule/ for planned restrictions, with same-provider e-nexco detail links. This is notices, not comprehensive live road status. Current traffic map handoff is en-www.drivetraffic.jp/map.html. Predicted versus live timing must remain explicit.

Data: online bounded requests, no bulk sync needed. Offline metadata only vehicle codes and help. Limit/offset/fields separate summaries from detail. Each output includes retrieved_at, timezone JST, source URL and coverage caveats.

Competitive table stakes: interchange/route/toll comparison, facilities, forecast/closure links. NAVITIME-like broader street driving and generic live traffic are beyond this provider. No unofficial SDK or external data is needed. Highest priority parser correctness, assumptions, coverage limits and actionable upstream errors.

Reachability: default sandbox DNS failed; escalated public HTTPS succeeded 200. This is tool networking, not provider restriction. No WAF/login/challenge observed in native Chrome or HTTP. Preserve empty results as empty and distinguish malformed source responses as failures.

Build priorities: interchanges, route estimates, roads + facility catalog and discovery, detailed SA/PA, notices + schedule handoffs, compact JSON and MCP/read-only declarations.
