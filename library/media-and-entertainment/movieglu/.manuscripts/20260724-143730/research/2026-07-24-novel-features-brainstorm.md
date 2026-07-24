# Novel feature decision

## Shipping: `movie-night`

Binary acceptance:

1. Resolve a case-insensitive exact or substring film name from the official
   `filmsNowShowing` response.
2. Query `filmShowTimes` with the resolved numeric film ID and requested date.
3. Flatten dynamic format keys such as Standard and IMAX.
4. Filter by `--after`, then rank by distance and start time.
5. With `--booking-link`, query `purchaseConfirmation` for the first option.
6. Reject non-HTTPS handoffs and launch only with explicit `--launch`.
7. State that seats and payment remain on the cinema website.

The fixture test must prove all three request paths, query parameters, required
headers, normalized output, and booking URL.
