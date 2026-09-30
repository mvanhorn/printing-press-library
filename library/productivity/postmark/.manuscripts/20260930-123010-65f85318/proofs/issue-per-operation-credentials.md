## Problem

Some APIs split auth by endpoint family. Postmark uses `X-Postmark-Server-Token` for sending, messages, bounces, templates, and stats, and `X-Postmark-Account-Token` for servers, domains, sender signatures, and template push. Each OpenAPI operation needs exactly one of the two.

The generator cannot express that. `selectSecurityScheme` picks one winning scheme by usage count, and `collectAdditionalAuthHeaders` only emits a second credential when it shares an AND requirement with the winner. So a spec author has two choices:

1. Declare per-operation security with one scheme each (OR across operations). The losing scheme gets no env var, no config field, and no header. Account endpoints cannot authenticate.
2. Declare an AND pair on every operation. Both env vars are emitted, and the client sends both headers on every request.

Option 2 breaks Postmark. Sending both tokens to `GET /server` returns HTTP 401 with ErrorCode 10 ("wrong token type was used for the endpoint"). Other endpoints tolerate the extra header, so the failure only shows up on some commands.

## Reproduction

Two apiKey header schemes, `serverToken` and `accountToken`, with `security: [{serverToken: [], accountToken: []}]` at the root. Generate, set both env vars, run the generated `server get`. The request carries both headers and fails with 401.

## What the printed Postmark CLI does today

A hand-written client hook wraps the transport and deletes whichever header the request path does not accept. It works, but every split-token API would need the same hand code, and the generated MCP execute path depends on that hook being present.

## Proposed fix

Honor per-operation security when it names a single scheme:

- Emit config fields and env vars for every scheme referenced by any operation, not only the winner and its AND siblings.
- Record the scheme per endpoint (for example `Endpoint.AuthScheme`) and have the client attach only that scheme's credential for the request.
- Keep current behavior for real AND requirements and for specs with one scheme.

Affected code: `internal/openapi/parser.go` (`selectSecurityScheme`, `collectAdditionalAuthHeaders`, `effectiveSecurityRequirements`), the client template's header block, and the doctor auth check, which should report each scheme separately.

## Acceptance

- A spec with two single-scheme operation groups generates both env vars and config fields.
- Requests for each operation carry only that operation's credential (golden fixture plus an httptest assertion on headers).
- `doctor` reports both credentials.
