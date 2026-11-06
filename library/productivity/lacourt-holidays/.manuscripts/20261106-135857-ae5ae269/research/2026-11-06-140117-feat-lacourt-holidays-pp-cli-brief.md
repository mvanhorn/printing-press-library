# LA Superior Court Holidays CLI Brief

## API Identity
- Domain: Superior Court of Los Angeles County judicial holiday schedule
- Users: litigators, paralegals, legal assistants computing deadlines and court-day math
- Data profile: tiny, read-only, ~14 holidays/year, years 2006-2030 published
- Homepage: https://www.lacourt.ca.gov/apps/court-holidays (official court site)

## Reachability Risk
- None: plain HTTP GET, no auth, no bot protection observed (curl 200 JSON with a browser UA)
- Discovered via Chrome network capture: Angular micro-frontend calls
  GET /web3/holidays/api/holidays/getholidays/{year}; response is a JSON string that itself contains
  a JSON array of {date "MM/DD/YYYY", day, description}. Years 2006-2030 are published (the page only shows tabs for 2026-2030); other years return "[]". Content negotiation: with Accept: application/json the body is a JSON string wrapping the array; without it, a plain array.

## Top Workflows
1. List court holidays for a year
2. Is a given date a court holiday / court day?
3. Next court holiday from today
4. Add N court days to a date (deadline math, skipping weekends + holidays)
5. Export to ICS / JSON

## Table Stakes
- No competing CLI or SDK found; the only incumbent is the web page itself.

## Data Layer
- Primary entities: holiday (date, day, description, year)
- Sync cursor: year
- FTS/search: description

## Product Thesis
- Name: lacourt-holidays-pp-cli
- Why it should exist: scriptable, offline-cached court-day math; the page offers no export or date arithmetic

## Build Priorities
1. holidays list <year>
2. is-holiday / court-day check, next
3. add-court-days, ICS export, sync cache
