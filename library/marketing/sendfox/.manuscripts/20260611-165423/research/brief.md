# SendFox CLI Research Brief

## Source docs

- API category: https://help.sendfox.com/category/270-api-integrations
- Endpoints: https://help.sendfox.com/article/278-endpoints
- Access tokens: https://help.sendfox.com/article/292-access-tokens
- API request setup: https://help.sendfox.com/article/309-creating-api-requests
- OAuth client setup: https://help.sendfox.com/article/323-creating-an-oauth-2-0-client

## Endpoint surface

SendFox publishes a compact REST surface at `https://api.sendfox.com`:

- `GET /me`
- `GET /lists`, `GET /lists/{list_id}`, `POST /lists`
- `DELETE /lists/{list_id}/contacts/{contact_id}`
- `GET /contacts`, `GET /contacts/{contact_id}`, `GET /contacts?email={contact_email}`
- `POST /contacts`
- `PATCH /unsubscribe`
- `GET /campaigns`, `GET /campaigns/{campaign_id}`

Docs state JSON responses with standard 2xx/4xx/5xx status semantics. Authentication uses `Authorization: Bearer <token>` with personal access tokens from https://sendfox.com/account/oauth.

## CLI plan

The generated endpoint mirrors cover the full documented API. Novel commands focus on compound operator workflows that save agents from manual fan-out over a small API:

- `workflow account-snapshot`: fetch `/me`, lists, contacts, and campaigns for one account packet.
- `workflow audience-map`: map observed list membership and identify contacts without list memberships.
- `workflow campaign-digest`: summarize campaign status/recency.
- `contacts onboard`: create a contact with list IDs in one dry-run-previewable call.
- `contacts import-csv`: guarded bulk import from CSV with a live `--yes` gate.
- `forms generate`: produce an embeddable form handoff with explicit server-proxy warning to avoid leaking bearer tokens.

## Safety notes

- Prefer `SENDFOX_API_TOKEN`; keep `SENDFOX_BEARER_AUTH` as a generated compatibility alias.
- Do not expose bearer tokens in browser-side signup form code.
- Bulk contact import is mutating and must support dry-run preview before live execution.
