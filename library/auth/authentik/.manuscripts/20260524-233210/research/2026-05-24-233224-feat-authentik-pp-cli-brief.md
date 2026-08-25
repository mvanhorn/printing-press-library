# authentik CLI Brief

## API Identity
- Domain: Identity provider / SSO (OAuth2, SAML, LDAP, OIDC, proxy auth)
- Users: HomeLab operator (single user, Mr. Finney) administering self-hosted IDP
- Data profile: Users, groups, applications, providers, flows, stages, policies, tokens, events, tenants
- Base URL: https://auth.echolimits.com
- API: OpenAPI 3.0.3, version 2025.10.3, 524 paths under /api/v3/

## Reachability Risk
- None — official OpenAPI 3 spec endpoint, Bearer token auth, exposed via Pangolin reverse proxy, returns 200 on /api/v3/, /api/v3/schema/, /-/health/live/

## Top Workflows
1. Inspect server health, version, worker tasks
2. List/audit users (status, last login, group membership)
3. List applications and their provider bindings
4. List/revoke API tokens
5. Inspect flows and their stage bindings

## Data Layer
- Primary entities: users, groups, applications, providers, flows, stages, policies, tokens, events
- Sync cursor: page-based pagination (page/page_size)
- FTS/search: username, email, application name, flow name

## Source Priority
- Single source: authentik official OpenAPI

## Product Thesis
- Name: authentik-pp-cli
- Why: HomeLab admin needs scriptable identity audit + token management without clicking through the admin UI; agent-native output for Claude Desktop MCP

## Build Priorities
1. Health/version/system status
2. User + group listing and inspection
3. Application + provider listing
4. Flow + stage listing
5. Token management (list/create/revoke)

## Scope Notes (per HomeLab Phase 4 brief)
- Include: users, groups, applications, flows, policies, tokens, providers, stages, system/health
- Exclude: crypto/cert internals, raw event streams, blueprint import/export, outpost low-level config, debug/metrics
- This is identity infrastructure — read-only smoke tests only in Phase 5; no write tests against live instance
