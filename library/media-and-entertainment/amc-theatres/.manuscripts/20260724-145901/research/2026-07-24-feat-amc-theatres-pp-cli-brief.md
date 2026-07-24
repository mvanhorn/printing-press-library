# AMC Theatres CLI research brief

AMC's official developer portal renders its Showtime API v2 contract through
RapiDoc using `/api/specs/showtime-api-v2.yml`. The browser-rendered contract
documents five read operations: an earliest showtime for a movie and theatre,
a showtime by ID, all showtimes for a theatre, dated showtimes for a theatre,
and dated current-location showtimes.

The production server is `https://api.amctheatres.com`; the sandbox server is
`https://api.sandbox-amctheatres.com`. Every request requires an
`X-AMC-Vendor-Key`. The contract also documents optional
`X-AMC-Auth-Token` user authorization.

The contract does not document ticket purchase, seat selection, or payment.
The CLI therefore stops at read-only planning and makes that boundary explicit
in both human and machine-oriented output.

## Official source

- https://developers.amctheatres.com/
- https://developers.amctheatres.com/api/specs/showtime-api-v2.yml

Direct non-browser retrieval of the specification was blocked by Cloudflare;
the contract was captured from the official browser-rendered RapiDoc surface.

## Public CLI audit

No maintained, agent-oriented AMC Showtime API CLI was identified. AMC's own
website and app serve human interaction, while cross-chain products do not
provide this official five-operation AMC contract as a deterministic JSON CLI.
