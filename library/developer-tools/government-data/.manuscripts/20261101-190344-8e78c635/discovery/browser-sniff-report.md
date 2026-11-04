# Chrome DevTools discovery report
## User Goal Flow
Search water datasets, inspect a result and resource download links. Completed search and detail navigation; earlier agent-browser pass also exercised CSV format filter. No file download was needed for endpoint discovery.
## Pages & Interactions
Opened data.ca.gov in fresh tab; opened DevTools Network, enabled Keep log, cleared log, entered water, submitted search, selected Water Plan Water Balance Data, exported sanitized HAR.
## Browser-Sniff Configuration
Chrome DevTools native UI via computer-use. Anonymous public flow. No proxy-envelope observed.
## Endpoints Discovered
| Method | URL | Status | Content-Type |
|---|---|---|---|
| GET | https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/search.json | 200 | application/json |
| HEAD | https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/search.json | 200 | application/json |
| GET | https://publish.data.ca.gov/api/3/action/package_search | 200 | application/json |
| GET | https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/%40california-department-of-water-resources/water-plan-water-balance-data.json | 304 | application/json |
| GET | https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/%40california-state-water-resources-control-board/ground-water-water-quality-results.json | 200 | application/json |
| GET | https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/%40california-state-water-resources-control-board/surface-water-water-quality-regulatory-information.json | 200 | application/json |
| HEAD | https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/@california-department-of-water-resources/water-plan-water-balance-data.json | 200 | application/json |
## Traffic Analysis
35 HAR entries, 33 bodies; 7 inferred routes across 2 resource groups. No empty-response warning. Build-specific Next.js routes are SSR discovery evidence only, not stable product endpoints. Stable CKAN package_search carries q, rows, sort, fq, facet.field and facet.limit.
## Coverage Analysis
Search and detail HTML/SSR covered; direct package_show and resource_show documentation supply stable metadata equivalents. No authenticated or admin flows.
## Response Samples
### https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/search.json
```
{"pageProps":{"fallback":{"@\"package_search\",#tags:@,resFormat:@,orgs:@,offset:0,limit:10,groups:@,,":{"count":4693,"facets":{"groups":{"natural-resources":3508,"water":567,"health-human-services":495,"economy-and-demographics":345,"fire":119,"transportation":79,"covid-19":19,"government":19},"organization":{"california-department-of-fish-and-wildlife":2427,"california-department-of-water-resources":488,"california-energy-commission":212,"california-department-of-health-care-services":200,"california-state-controllers-office":179,"california-franchise-tax-board":147,"department-of-health-care-access-and-information":131,"cal-fire":119,"california-department-of-public-health":109,"california-department-of-conservation":105,"caltrans":78,"california-state-water-resources-control-board":57,"california-state-lands-commission":55,"california-natural-resources-agency":39,"california-office-of-environmental-health-hazard-assessment":37,"california-ocean-protection-council":33,"california-de
```
### https://publish.data.ca.gov/api/3/action/package_search
```
{"help": "https://publish.data.ca.gov/api/3/action/help_show?name=package_search", "success": true, "result": {"count": 1116, "facets": {"groups": {"natural-resources": 1035, "water": 567, "health-human-services": 7, "fire": 5, "government": 4, "covid-19": 2, "economy-and-demographics": 1, "transportation": 1}, "organization": {"california-department-of-water-resources": 488, "california-department-of-fish-and-wildlife": 466, "california-state-water-resources-control-board": 57, "california-ocean-protection-council": 33, "california-energy-commission": 20, "california-state-lands-commission": 10, "california-natural-resources-agency": 8, "california-department-of-technology": 7, "cal-fire": 5, "california-office-of-environmental-health-hazard-assessment": 5, "san-francisco-bay-conservation-and-development-commission": 5, "california-department-of-public-health": 4, "california-environmental-protection-agency-calepa": 2, "california-department-of-community-services-development": 1, "cal
```
### https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/%40california-department-of-water-resources/water-plan-water-balance-data.json
```
{"pageProps":{"dataset":{"accessLevel":"public","accrualPeriodicity":"R/P1Y","author_email":null,"conformsTo":"","contact_email":"[public agency contact omitted]","contact_name":"[public agency contact omitted]","creator_user_id":"be1db721-b4f5-4257-abcb-c2115ad06ee3","id":"a47fde7d-7071-4f89-bd95-d8912ec8c385","isopen":true,"language":"","license_id":"other-pd","license_title":"Other (Public Domain)","maintainer":null,"maintainer_email":null,"metadata_created":"2025-07-18T00:32:37.903527","metadata_modified":"2026-10-08T18:33:38.026697","name":"water-plan-water-balance-data","notes":"Department of Water Resources California Water Plan program computes applied, net, and depletion water balances for California. Water balances are simplified water budgets for a water year based on analyses of developed and dedicated water supplies, water uses by sector, water reuses, operational characteristics for an area, and inflows and outflow for a study area that occur above the root zone.  Dedicated and develo
```
### https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/%40california-state-water-resources-control-board/ground-water-water-quality-results.json
```
{"pageProps":{"dataset":{"accessLevel":"public","accrualPeriodicity":"R/P3M","additional_information":"**[GAMA GIS factsheet](https://www.waterboards.ca.gov/publications_forms/publications/factsheets/docs/gama_gis_factsheet.pdf)**","author":"California State Water Resources Control Board","author_email":null,"citation":"","conformsTo":"","contact_email":"[public agency contact omitted]","contact_name":"Groundwater Ambient Monitoring and Assessment Unit","creator_user_id":"4e98cf74-7831-4e4b-a417-5fec6c8f1967","geo_coverage":"","granularity":"","group":"","id":"81d1347c-891d-4f09-abc5-6eeb521b55d2","isopen":false,"landingPage":"https://gamagroundwater.waterboards.ca.gov/gama/gamamap/public/","language":"","license_id":"","license_title":"","maintainer":null,"maintainer_email":null,"metadata_created":"2019-08-10T03:59:27.974358","metadata_modified":"2026-10-29T23:22:28.632752","name":"ground-water-water-quality-results","notes":"<p>Groundwater quality data and related groundwater well informa
```
### https://data.ca.gov/_next/data/angFhxGvQgnvsfEcRbt_v/%40california-state-water-resources-control-board/surface-water-water-quality-regulatory-information.json
```
{"pageProps":{"dataset":{"accessLevel":"public","accrualPeriodicity":"irregular","additional_information":"","author":"California State Water Resources Control Board","author_email":null,"citation":"","conformsTo":"","contact_email":"[public agency contact omitted]","contact_name":"California Integrated Water Quality System (CIWQS)","creator_user_id":"4e98cf74-7831-4e4b-a417-5fec6c8f1967","geo_coverage":"","granularity":"","group":"water","id":"b2bda258-6433-49a0-958a-09b06c926f76","isopen":false,"landingPage":"https://www.waterboards.ca.gov/ciwqs","language":"","license_id":"","license_title":"","maintainer":null,"maintainer_email":null,"metadata_created":"2019-08-10T05:02:43.632966","metadata_modified":"2026-10-06T10:54:29.894969","name":"surface-water-water-quality-regulatory-information","notes":"This data set lists active and historical enforcement actions records that are stored in the [California Integrated Water Quality System](https://www.waterboards.ca.gov/ciwqs) (CIWQS). Enforcemen
```
## Rate Limiting Events
No observed 429s. Interactive capture; request timing not benchmarked.
## Authentication Context
No authenticated session used; credential header/cookie fields stripped from retained discovery HAR.
## Runtime
Standard HTTP calls to documented California CKAN and federal v4 gateway. No resident browser.