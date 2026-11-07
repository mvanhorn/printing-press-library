# Bowmark: research brief

Bowmark gives AI agents typed functions for live websites (LinkedIn, Amazon, Google Maps, Zillow and hundreds more), each tested on a schedule. Two calls cover most work: `GET /v1/library` returns the functions for a task (read-only, free), and `POST /v1/run` runs a short async script written against them. Every site also has its own OpenAPI spec under `/v1/sites/{site}/openapi.json`, callable one function at a time.

- Spec: https://api.bowmark.ai/openapi.json (official, OpenAPI 3.1)
- Docs: https://bowmark.ai/docs/api
- Auth: bearer API key from https://bowmark.ai/dashboard/keys; new accounts get a free $10 of usage.
- Live-tested with press 4.35.3: `run --script 'return 1+1'` returned 2 (status ok, ~1 s); `library --query "amazon product"` found `bowmark.products` and `bowmark.providers.amazon`; `sites list` returned the site catalog.
