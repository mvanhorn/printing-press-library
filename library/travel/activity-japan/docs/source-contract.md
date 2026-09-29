# Activity Japan source contract

Observed 2026-09-28 through an existing Firefox session and anonymous replay of public GETs on `gd.activityjapan.com`. This is an undocumented website surface, not the contracted partner API. The browser's search HTML is separately WAF challenged from the terminal.

## Identities

`plan_data.plan_id` is the plan ID; `partner_id` is the operator ID; `price_items[].plan_price_id` is a price option ID; `/select_plan_course.course_name` values are course/session IDs. Preserve IDs as strings in CLI JSON even when the source transports JSON numbers. A translated page may use the same plan and operator IDs but different names and availability of that locale's listing. Keep `plan_name` from both `lang_flag=ja` and `lang_flag=en` when fetched; never overwrite the original Japanese name with the English translation. Canonical booking URL is `https://activityjapan.com/publish/plan/<id>` or `https://en.activityjapan.com/publish/plan/<id>` for that language only when the plan is actually indexed there.

The detail endpoint takes a constructed plan URL as a query parameter even when the page is absent from that locale's index. Therefore detail output keeps `canonical_url` null and labels its `source_url` as constructed. Brief, compare and handoff may label a URL canonical only after checking the selected locale's plan sitemap. An unavailable sitemap or absent listing leaves that URL null with a partial error.

## Price

`/get_plan_price_info` returns `plan_data`, `plan_price`, and `price_items`. Each price item has its own `base_price`, `discount_price`, `plan_price_id`, suffix, prefix and participant bounds. These are headline/source option fields, not a selected-date party total. `/plan/get_calendar` returns a date-level headline `price` and source status. `/plan/get_plan_price?plan_id=&date=` returns selected-date option prices under `planPriceList`, with `price_id`, `price`, `base_price` and `depend_id`; these source integers take precedence over recomputing a discount. `taxIn=1.1` was observed, while the plan page says list prices include consumption tax. Do not apply the multiplier to a displayed source amount. Option suffixes vary (`person`, `people`, `pair`, Japanese `人`); an amount whose basis cannot be proven remains basis `unknown`. Mandatory extras and optional upgrades remain separate.

For an adult subtotal, honor both the dated option's `price_item_min/max` and the detail option's `item_people_min/max`; the stricter known unit bounds apply. Original Japanese age wording governs adult/child/infant interpretation. A translated English label alone cannot establish adult applicability for an otherwise generic Japanese option; keep that age class and subtotal unknown in both locales.

## Availability

`/plan/get_calendar?plan_id=&year=&month=` returns a month grid containing objects for days plus blank arrays for padding. Blank cells are not sold-out evidence. Day status is only a date-level observation. `/select_plan_course?plan_id=&selected_date=` returns a map from displayed time to course ID and a map from course ID to status. The site's calendar JS and page legend support: `1` immediate booking OK, `3` request booking, `4` reception closed, `5` not accepted. The site renders 4 and 5 unselectable. Status `2` has no verified label; report unknown with raw code. `3` is a request requiring provider acceptance, never a confirmed booking.

`/plan/check_calendar_data` is a read-only GET recheck of plan/course/date/status/count/type=2. The site's script maps result `1` to no change; `2` to request→instant; `3` to instant→request; `4` to no longer bookable; `5` to stock insufficient for count. The `stock` field is a source observation; it is not a reservation. Enforce plan `people_min`/`people_max` first: an over-limit request can receive result `3`, which does not make the traveler eligible. Recheck date and course IDs echoed by the CLI request. Do not claim availability based only on operating period or session name list.

The generated `source-plan stock-check` CLI and `source_plan_stock_check` MCP tool expose that endpoint's raw result for investigation. They do not check the plan's party or age limits. Use `experience check` for the bounded, party-aware observation; a raw source result alone is not evidence that the traveler is eligible.

## Conditions and language

The detail envelope carries `age_start/end`, `people_min/max`, `basic_min_passenger_count`, `necessary_time_comment`, `basic_meeting_time`, `basic_meeting_place`, address, inclusions, exclusions, rental equipment, attention text, payment and cancellation text. Venue address, meeting place and pickup remain separate; do not infer pickup from an access map. `period_start/end` can be `0000-00-00 00:00:00`, a placeholder rather than a real date. `support_language` is a Boolean whose value changes by `lang_flag`; it is locale-specific source evidence, not a complete instructor language list. The plan page may show a specific Supported language label, which takes precedence when captured. Site locale alone never proves instructor language.

Preserve `other_attention`, `other_comment`, and `default_message_note` as separate source text fields when present. These can contain participation or preparation advice; do not turn them into inferred weather or equipment rules.

## Transport and failure

The direct data host returned JSON 200 for verified valid plan IDs across Kyoto culture, Okinawa outdoors, Osaka guided experience and Fukuoka group kayak. An invalid plan ID returned an HTML fallback with HTTP 200. Verify content type, JSON schema, returned plan ID and requested date before treating a response as success. Preserve typed transport, rate-limit, denial and schema errors. Cap body size, redirects, retries, concurrency, date scans and cache lifetime. Source denial is not an empty result. Raw source strings and observation times are retained.

## Implementation clarification from cross-plan live checks

The source's `basic_min_passenger_count` was 1 for Kyoto chopsticks, 2 for Fukuoka's pair option, and 50 for an Osaka sumo plan whose `people_min` is 1. Its meaning is not verified as a minimum traveler party size, so the CLI preserves it as a raw source field and enforces `people_min`/positive `people_max` only. Source zero `people_max` is an unknown upper bound, not a literal capacity of zero. For group-priced options, the stock endpoint's `count` unit is unverified; the CLI refuses those checks rather than treating pairs as people.

An Osaka sushi plan exposed separate adult (9,000 JPY), child (7,000 JPY), and infant (0 JPY) options. The plan headline field was 9,000 JPY. Option unit amounts and age bands are separate from that headline and the dated quote; an adult subtotal cannot use the child or infant option. A duration string such as “90 inches.” or mixed total/activity prose remains raw source text and yields unknown derived total minutes.
