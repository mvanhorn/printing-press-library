# SEC read-only live smoke

The SEC_USER_AGENT contact was supplied only to ephemeral process request headers. DATA_GOV_API_KEY was not set or needed for these SEC commands.

- `edgar facts 0000320193 --concept Assets --json`: success; expected cik/entityName/tag/taxonomy/units and USD fact arrays.
- `edgar facts 0000320193 --json --no-learn`: success; dei and us-gaap taxonomies returned.
- `edgar search water --forms 10-K --limit 1 --json --no-learn`: success; one real hit returned; upstream total relation gte, value 10000 retained.
- `edgar filings 0000320193 --form 10-K --limit 1 --json --no-learn`: success; one real 10-K, accession 0000320193-25-000079, scanned_pages 1.
- `edgar documents 0000320193 --accession 0000320193-25-000079 --json --no-learn`: success; 93 archive entries.
- Selected `0000320193-25-000079-index-headers.html` downloaded to a temporary explicitly selected output path, 21434 bytes. Atomic no-overwrite save used.

A date-bounded EFTS request returned HTTP200 with SEC backend errorType ConnectionError rather than a hits envelope. Source now rejects application-error/missing-hits envelopes to avoid reporting a false empty search. This is an upstream transient, not valid zero-hit evidence.

Four focused parser/safety tests passed, and SEC slice built/vetted before concurrent catalog changes. Parent must rebuild the integrated module after all slices settle to pick up the final error-envelope change.
