# Novel feature decision

## Shipping: `movie-plan`

Binary acceptance:

1. Require exactly one location mode: an AMC theatre number, or both latitude
   and longitude.
2. Query the official dated theatre or current-location showtime operation.
3. Normalize documented and compatible nested showtime response shapes.
4. Filter case-insensitively by movie and presentation format.
5. Filter by `--after HH:MM`.
6. Rank by distance when available, then start time, and enforce `--limit`.
7. Emit an explicit read-only boundary: no purchase, seats, or payment.
8. Preserve dry-run and verification behavior without making a network call.

The mock-HTTP test must prove the exact current-location path, page-size query,
vendor-key and optional auth-token headers, normalized output, and ranking.
