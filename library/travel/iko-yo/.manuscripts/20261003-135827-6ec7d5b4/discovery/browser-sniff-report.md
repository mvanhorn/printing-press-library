# Iko-yo Trip browser discovery

## Goal and interactions
Goal: discover parent/child local trips and compare published conditions. Owned native Chrome tab 448471767 first verified core homepage 403, then visited official Trip FAQ. Clicked Spots; opened Mooovi spot 8220; clicked Events; opened application-based event 8412; returned home; opened Kanto event region 6; clicked Saitama prefecture 11; inspected the observed page=2 href; clicked page 2 and verified different archived events. All intended public discovery, detail, geography and pagination flows worked on Trip; core catalog did not.

## Backend and replay
Native Browser Use was selected explicitly by the batch brief. DOM/accessibility observations were cross-checked with ordinary HTTP raw responses. No page-context fetch interceptor or XHR API was needed: these are server-rendered HTML document routes. The enriched capture contains six observed HTML targets with minimized factual response samples (first two listing cards or basic-information table plus selected relevant factual paragraphs), not a full network HAR. Head scripts/CSRF, provider keys, author profiles, ads and cookies are excluded before writing. No authenticated session, proxy-envelope or challenge solving used. Existing browser-use/agent-browser tooling was not modified.

## Discovered contracts
| Method | Path | HTTP | Content type | Auth |
|---|---|---:|---|---|
| GET | /spots | 200 | text/html | public |
| GET | /events | 200 | text/html | public |
| GET | /spots/8220 | 200 | text/html | public |
| GET | /events/8412 | 200 | text/html | public |
| GET | /events/regions/6 | 200 | text/html | public |
| GET | /events/regions/6/prefectures/11 | 200 | text/html | public |
Observed page links carry page=N. Region navigation supplies ids 1–11 and prefecture links. The source has no keyword/date/age search form on these pages; those filters will be bounded local operations with disclosed coverage. Trip probe-reachability classified standard_http; both stdlib and Surf fetched useful 200 HTML. Core probe got 403 on both rungs and native Chrome remained 403, so core is excluded.

## Analysis and coverage
Browser-sniff generated six endpoints across spots/events with response_format html. Its analyzer counts HTML as noise (api_entry_count=0), while still generating useful HTML contracts; this does not establish a hidden JSON API. Listing card name/location/date and details basic-information rows are actual response facts. Parsing requires source-specific selectors beyond generic page metadata. No original article body, contributor profile or review will be surfaced or cached. Missing facts remain unknown. Archive status and uncertain/noncontinuous date schedules require explicit output states.

## Samples and limits
Spot 8220: child/adult JPY 300 with conditional adult admission qualifier; source states 6 months–12 years, indoor area, nursing and changing space. Event 8412: 2026-11-15; JPY 2,000 payable on day; application interval 2026-09-01 to 2026-10-16; 30-person capacity and lottery. The factual table is the source, and availability is unknown. See minimized enriched capture for class/row samples. No 429 observed. UI calls averaged several seconds each; direct probes took 0.4–0.7 s. No bundle extraction performed because HTML directly supplies the required facts.
