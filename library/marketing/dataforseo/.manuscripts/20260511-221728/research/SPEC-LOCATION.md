# OpenAPI spec

The DataForSEO OpenAPI specification used at generation time is publicly hosted at:

https://github.com/dataforseo/OpenApiDocumentation

The exact version used for this build was the `master` branch HEAD as of
2026-05-11 22:21:30 UTC (file size ~4.2 MB, 437 paths, 554 operations).

It's not included in this manuscripts archive because:
1. It's already public and stable
2. Stripping the 4.2MB spec from manuscripts keeps the published PR diff small
3. The Printing Press PII scanner produces false positives on numeric strings
   in DataForSEO's example responses (longitude coordinates, product UPCs) that
   match its US-phone-number regex
