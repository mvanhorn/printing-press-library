# Context.dev Research Notes

Run ID: `20260623-222307`

Generator: `cli-printing-press` `4.25.0`

Official source material:

- Documentation index: `https://docs.context.dev/llms.txt`
- API reference/spec: `https://app.stainless.com/api/spec/documented/context.dev/openapi.documented.yml`
- Base URL: `https://api.context.dev/v1`

The official docs describe Context.dev as an API for turning domains and URLs into structured AI-ready data, including brand intelligence, web scraping to Markdown/HTML/images/sitemaps, crawl, web search, structured website extraction, design-system extraction, screenshots, industry classification, transaction enrichment, product extraction, and prefetching.

The OpenAPI spec generated endpoint coverage for:

- Brand intelligence: retrieve by domain, company name, email, ticker, ISIN, simplified domain profile, transaction identifier, prefetch by domain/email.
- AI/product extraction: single product, product list, brand AI query.
- Web extraction and scraping: scrape HTML, Markdown, images, sitemap, crawl, structured extract, search, competitors, fonts, styleguide, screenshot, NAICS, and SIC.
- People retrieval.

Hand-authored first-class workflows were added on top of the generated endpoint commands:

- `doctor-discover <name> <city, state>`
- `clinic-enrich <domain|url>`
- `scrape <url>`
- `crawl <seed>`
- `extract <url> --schema <file.json>`
- `styleguide <domain|url>`
- `screenshot <domain|url>`

Healthcare constraint for `doctor-discover`: the command is public web research about providers/practices only. It rejects patient-context terms and identifier-shaped input, returns only public provider/practice candidate fields, labels ranking as heuristic, keeps per-candidate enrichment failures non-fatal, and preserves a distinct API-failure error path from a genuine zero-result search.
