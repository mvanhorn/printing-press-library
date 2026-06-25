# xero-pp-cli — generated CLI operating guide

This is a Printing Press generated local-library candidate for `xero-pp-cli`.

Safety rules:

- Fixture-only and read-only in this candidate.
- No live OAuth, no token storage, no `.env`, no live provider API calls.
- No accounting mutations or write commands.
- Do not add live auth until the auth safety checklist is implemented and tested.
- Use `testdata/fixtures/xero/` for examples and verification.
