# Cloudflare Registrar Domains research

Generated from a local Cloudflare Registrar OpenAPI archive for domain search, live domain-check pricing, and gated registration.

Key API wire shapes verified for the printed CLI:

- `GET /accounts/{account_id}/registrar/domain-search` uses query parameter `q`.
- `POST /accounts/{account_id}/registrar/domain-check` sends `{ "domains": ["example.dev"] }`.
- `POST /accounts/{account_id}/registrar/registrations` sends `{ "domain_name": "example.dev" }` and is gated by an exact typed confirmation flag.
