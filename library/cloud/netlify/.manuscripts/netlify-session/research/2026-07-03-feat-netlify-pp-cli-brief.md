# Netlify CLI Brief

## API Identity
- Domain: Web hosting / deploys / DNS / forms / serverless functions (JAMstack platform)
- Spec: https://open-api.netlify.com/swagger.json — Swagger 2.0, v2.56.0, host `api.netlify.com`, basePath `/api/v1`
- Surface: 115 paths, 179 operations across ~40 tags (site, deploy, dnsZone, environmentVariables, form/submission, buildHook, function, splitTest, deployKey, member, account)
- Auth: `netlifyAuth` bearer token → `Authorization: Bearer <token>`, env `NETLIFY_AUTH_TOKEN`
- Users: JAMstack devs, agencies managing many sites, devops/CI

## Reachability Risk
- None. GET /api/v1/sites → 401 without auth, 200 with token (returned `[]`, valid). Token verified working.

## Top Workflows
1. Manage sites: list/create/get/update/delete, view deploys, trigger builds
2. Deploys: list, get, restore/rollback, lock/unlock, cancel
3. Env vars: get/set/update/delete per site & account, per-context values
4. DNS: manage zones + records
5. Forms & submissions: list forms, read submissions, delete spam
6. Build hooks & deploy keys for CI

## Table Stakes (incumbent: official `netlify-cli` npm)
- sites CRUD, deploy list/trigger, env var get/set, DNS record CRUD, form submission read, build hook trigger

## Data Layer
- Primary entities: sites, deploys, builds, dns_zones, dns_records, env_vars, forms, submissions, build_hooks, deploy_keys, members
- Sync cursor: updated_at / created_at per resource
- FTS/search: form submissions, sites, deploys

## Product Thesis
- Name: netlify-pp-cli
- Why it should exist: The official CLI is per-site and stateless. This one keeps a local SQLite mirror of your whole Netlify account, so you can search every form submission offline, diff env vars across sites, audit all DNS records at once, and answer "what deployed in the last 2h across all sites" — cross-site questions the UI and API can't answer in one call. Agent-native (`--json`, `--select`, typed exit codes, `--dry-run`).

## Build Priorities
1. Data layer + sync for all primary entities (generator)
2. Absorb full endpoint surface (generator, typed commands)
3. Transcendence: cross-site aggregation, offline FTS, env drift, DNS audit
