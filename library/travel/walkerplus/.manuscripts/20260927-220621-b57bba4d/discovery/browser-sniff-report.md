# Walkerplus browser discovery
## User Goal Flow
Goal: find Kyoto festivals and open event details. Completed public advanced filter submission, festival category link, first event link. 3/3 steps. The first form submission had unset optional timelist and yielded a duplicate slash/undefined query; normalized category links produce clean routes. UI has single-day selection, no arbitrary date range.
## Pages & Interactions
https://www.walkerplus.com/info/search/event/ -> select Kyoto and all dates -> /event_list//ar0726/?=undefinednonear0726none -> click festival link /event_list/ar0726/eg0055/ -> click first event. See captures for exact resulting event URL.
## Browser-Sniff Configuration
Installed browser-use CLI synchronous eval, isolated walkerplus-discovery session. Navigation Performance API plus DOM captures converted to HAR-shaped analysis input (DOM snapshots, not raw network body export; timestamps extraction time). No proxy envelope. Less than one page request per second.
## Endpoints Discovered
GET /event_list/ar0726/eg0055/ HTML200 public; GET event detail HTML200 public. Region/category/month listing, event base/data/price routes independently replayed by curl. No credentials required.
## Traffic Analysis
Structured HTML and JSON-LD; standard HTTP replay. Native filters path segments, categories eg*, areas ar*. No bot challenge. Generated HTML extraction scaffolding needs custom parser for faithful facts.
## Coverage Analysis
Search/filter/detail covered; pagination/month/city/category links inspected in raw HTML. Indoor attribute not yet established: any derived filter must require explicit source evidence.
## Response Samples
Captured DOM pages in browser-sniff-capture.har; raw HTTP samples event-list.html/detail.html/detail-data.html. Event JSON-LD name/startDate/endDate/location; offers.price None is unknown, offers.availability not reservation evidence.
## Rate Limiting Events
No429 observed. Conservative interactive pacing.
## Authentication Context
No authenticated session used. No cookie/token values retained.
