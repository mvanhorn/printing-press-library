# AllTrails Auth

This PP-side package uses caller-owned AllTrails credentials. It does not scrape, store, or ship account secrets.

## Environment Variables

- `ALLTRAILS_ACCESS_TOKEN` sets `Authorization: Bearer ...`.
- `ALLTRAILS_COOKIE` sets the browser `Cookie` header for authenticated browser/mobile routes.
- `ALLTRAILS_CSRF_TOKEN` sets `x-csrftoken` for write routes that require browser CSRF protection.
- `ALLTRAILS_BASE_URL` overrides the API origin for verifier/mock-server runs.

If both token and cookie auth are present, the token becomes the primary `auth_source`; the cookie is still sent as a header because some browser-backed routes require it.

## Notes For Agents

Never print or persist these values. Use `alltrails-pp-cli doctor` to confirm whether auth is detected, and prefer `--dry-run` before sending any write route.
