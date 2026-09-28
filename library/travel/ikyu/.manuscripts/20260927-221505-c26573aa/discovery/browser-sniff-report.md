# Ikyu anonymous discovery

## User goal flow
Goal: inspect a dated ryokan, filter for an outdoor-bath room, and inspect its exact plan before booking handoff.
Completed: dated property page; room-plan anchor; outdoor-bath room filter; plan detail drawer. Destination/date/party and pagination were separately verified by HTTP probes in live-contract.md. No reservation, account, checkout or coupon acquisition action was performed.

## Pages and interactions
1. Anonymous headless homepage https://www.ikyu.com/ returned a visible Forbidden page; no usable capture.
2. User approved retry. A fresh visible anonymous browser opened https://www.ikyu.com/00002889/?cid=20261117&cod=20261118&ppc=2&rc=1&lc=1 successfully. Login link remained visible.
3. Clicked プランをみる, then 露天風呂付客室. URL added acr=18. The list narrowed to room 10193727.
4. Clicked the public 詳細・予約 link /00002889/11055986/10193727/. This opened a plan detail drawer showing cancellation text and prices; no booking button was clicked.

## Configuration and endpoints
browser-use 0.13.1 CLI, isolated session ikyu-20260927, visible anonymous retry. Three read-only interactions. Browser closed after capture. Only www.ikyu.com GraphQL request/response bodies retained; headers empty. Opaque booking action URLs and credential-named fields redacted before writing.

| Method | Path | Status | Content type | Auth |
|---|---|---|---|---|
| GET | /00002889/ | 200, usable UI | HTML/SSR | anonymous |
| POST | /graphql?lang=ja-JP | 200 (5 batches) | application/json | anonymous |

Observed query operations include AccommodationIkyu, AccommodationMeta, PlansAndRooms, Search, PlanAndRoomFilter, KodawariFilter, RoomPlanDetailAlt, RoomPlanDetailAmount and RoomPlanDetailInventory. Browser-generated BookingButton, donation/coupon and sharing helpers are discovery evidence only, outside product scope.

## Traffic analysis
Printing Press browser-sniff analyzed the sanitized capture and emitted five endpoints across three resources. Protocol graphql confidence 0.92; response shape medium with populated fields; no empty_response_shapes warning. Analysis describes captured traffic, not a guarantee of runtime replay. Its generated batch-to-operation spec needs scoped repair before generation.

## Runtime evidence and coverage
Headless homepage plus Press stdlib/Surf homepage probes returned 403. Fresh stock curl homepage and dated property/room/plan/destination reads returned 200. A separate Go1.27.1 default net/http client fetched dated property SSR with HTTP/2.0, 860225 bytes, no cookies or explicit UA, in 1.983s. This establishes native Go SSR viability for that route, not global reachability. GraphQL anonymous replay is tested separately in live-contract.md.

## Response sample
Room filter returned JSON batches shaped [{"data":{"accommodation":{...}}}], with Japanese property/room/plan names, stable IDs, source amounts and detailed policies. Five sanitized response bodies are in browser-sniff-capture.json. Raw HTML and full response bodies are not CLI output.

## Rate limiting and authentication
No 429 responses were observed. Interactions were paced manually; five GraphQL batches were observed across three interactions. No authenticated session, login, challenge solution or cookie transfer was used. Initial homepage access denial remains a documented route-specific limitation.
