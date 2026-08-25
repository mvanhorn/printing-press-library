# amazon.jobs backing API — confirmed contract (direct HTTP, no auth)

Base URL: https://www.amazon.jobs

## Endpoint: GET /en/search.json
Server-side params (CONFIRMED working):
- base_query        keyword; ALSO matches an id_icims exactly (get-by-id path)
- normalized_country_code[]  ISO alpha-3 (USA, GBR, IND, ...) — hard filter
- normalized_state_name[]    full state/region name (Washington) — hard filter
- normalized_city_name[]     city name (Seattle) — hard filter (matches jobs where city is in locations[])
- offset            pagination offset (int)
- result_limit      page size (int; 0 = counts only)
- sort              recent | relevant

NOT working via .json (fall back to CLIENT-SIDE filter over returned records):
- category[] / business_category[] slugs  -> all returned hits=0
- schedule_type_id[] / is_intern[] / is_manager[] / job_schedule_type[] -> all hits=0

Response top-level: { error, hits, facets(empty), content, job_posting_search_request, jobs[] }
- hits: total matches (capped at 10000 for very broad queries; accurate for filtered)
- facets: returned EMPTY even when facets[] requested — no server aggregation available

## Job record fields (full record present in search results — no separate detail endpoint):
id (uuid), id_icims (canonical numeric id), job_path,
title, description, description_short, basic_qualifications, preferred_qualifications,
business_category, job_category, job_family, job_function_id, department_cost_center,
city, state, country_code, location, locations, normalized_location, display_distance,
company_name, team{...,label}, is_intern, is_manager, university_job, job_schedule_type,
posted_date (e.g. "July 24, 2026"), updated_time, primary_search_label, optional_search_labels,
source_system, url_next_step

## Get one job by id:
GET /en/search.json?base_query=<id_icims>&result_limit=1  -> hits=1, full record.
(/en/jobs/{id}.json 302-redirects to the HTML detail page; no clean JSON detail endpoint.)

## Text-cleaning note:
description / *_qualifications contain literal <br/> tags and HTML entities.
Go encoding/json parses responses fine (control chars are not a blocker).
Human output should strip HTML via cliutil.CleanText.

## CRITICAL: result_limit >= 1 required for filtered queries
- result_limit=0 (counts-only) + any normalized_* filter -> hits=0 (false zero).
- result_limit>=1 + filter -> correct hits AND matching jobs (verified: Seattle=3418 all-match, IND=2650 all-match).
- => generated/hand-written commands MUST default result_limit>=1; never send 0 with filters.
- For a counts-only need, fetch result_limit=1 and read .hits.

## CONFIRMED: normalized_* filter wire keys REQUIRE trailing []
- normalized_city_name[]=Seattle + result_limit>=1 -> filtered (3418, all-match).
- normalized_city_name=Seattle (no brackets) -> ignored (10000).
- => spec param name MUST be "normalized_country_code[]" / "normalized_state_name[]" / "normalized_city_name[]".
- Verify generated client serializes the bracket key verbatim; fallback = url_name override.
