# MovieGlu CLI research brief

MovieGlu is the best first movie-ticket discovery surface because its official
API spans participating cinemas rather than one theater chain. The documented
v2 resources cover films now showing, nearby cinemas, showtimes by cinema or
film, closest screenings, and a cinema booking deep link.

The API requires `x-api-key`, `client`, `authorization`, `territory`,
`api-version`, and `device-datetime` headers. Location-aware resources also
require `geolocation` in `lat;lng` form. Evaluation access is country-scoped and
limited to 75 requests; sandbox access is limited to 10,000 requests.

MovieGlu explicitly does not support seat selection or payment. The CLI’s
booking boundary is therefore a read-only HTTPS handoff to the cinema website.

## Source URLs

- https://developer.movieglu.com/v2/api-index/quick-start-guide/
- https://developer.movieglu.com/v2/api-index/setup/
- https://developer.movieglu.com/v2/api-index/filmsnowshowing/
- https://developer.movieglu.com/v2/api-index/cinemasnearby/
- https://developer.movieglu.com/v2/api-index/cinemashowtimes/
- https://developer.movieglu.com/v2/api-index/filmshowtimes/
- https://developer.movieglu.com/v2/api-index/closestshowing/
- https://developer.movieglu.com/v2/api-index/purchaseconfirmation/

## Public CLI audit

No maintained MovieGlu, AMC, Regal, or Atom Tickets CLI was found. The only
relevant GitHub results were two zero-star Cinemark scrapers last updated in
2018/2020 and a one-star Fandango CLI last updated in 2019. None overlaps this
cross-chain, agent-oriented workflow.
