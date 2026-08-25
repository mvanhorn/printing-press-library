# Policy Intel Live Validation Summary

Validated on 2026-06-18.

## Federal Register

- `federal-register search "artificial intelligence" --agent` returned current Federal Register documents.
- `rules "artificial intelligence" --agent` returned Federal Register rules/proposed rules.
- Agency slug filtering was checked with `federal-trade-commission`.

## Regulations.gov

- `docket EPA-HQ-OPPT-2018-0462 --agent` returned docket metadata using `DEMO_KEY`.
- `comments EPA-HQ-OPPT-2018-0462 --agent` returned public comments using `DEMO_KEY`.
- `deadlines "water" --from 2026-06-18 --agent` reached the Regulations.gov public `DEMO_KEY` rate limit during fresh validation after the docket/comment smoke calls. The error redacted `api_key` in the URL. Use `POLICY_INTEL_REGULATIONS_API_KEY` for repeatable deadline polling.
- `page[size]=2` was rejected by the upstream API, so the CLI normalizes Regulations.gov limits to at least 5.

## Security

- No API key values were committed.
- Error URLs redact `api_key`, `key`, `token`, and related query parameters.
- All commands are GET-only.
