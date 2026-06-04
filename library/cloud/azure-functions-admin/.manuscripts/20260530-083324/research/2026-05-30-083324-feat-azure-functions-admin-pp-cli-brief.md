# Azure Functions Admin CLI Brief

## API Identity
- **Domain:** Azure Functions management/observability via Azure Resource Manager (`Microsoft.Web/sites` where `kind` ~ `functionapp`) + Functions host/admin (keys, invoke) + Application Insights (metrics/logs via Log Analytics KQL).
- **Users:** Azure developers, platform/DevOps engineers, and small businesses running serverless workloads on Azure Functions who need to inspect, audit, and right-size their functions without the portal.
- **Data profile:** Function apps, functions, app settings (incl. Key Vault refs), function/host keys, hosting plans (Consumption/Premium/Dedicated), deployment slots, invocations + metrics time-series, role assignments. Time-series (invocations, cold starts, exec duration) is the high-gravity data.

## Reachability Risk
- **None.** ARM is a stable, documented, programmatically-accessible API (`https://management.azure.com`). Auth via Azure AD (`DefaultAzureCredential`). No bot protection. Live calls require a service principal + subscription (deferred to Phase 5).
- Probe-safe endpoint: `GET /subscriptions?api-version=2022-12-01` (lists accessible subscriptions; 401 expected without creds, which is a PASS for the reachability gate).

## Top Workflows
1. **"Should I move off Consumption?"** — inspect cold-start rate + exec-duration trend for an app, get a plan-fit recommendation. (NOI)
2. **Inventory + audit** — list all function apps across a subscription/RG, their functions, settings, and plan tier.
3. **Config hygiene** — diff app settings across apps/slots, flag raw secrets that should be Key Vault refs, find stale/unused keys.
4. **Operational triage** — recent invocation failures clustered by function + exception type; find functions with zero invocations (cleanup).
5. **Key/invoke discovery** — list host + function keys (masked) and invoke URLs (the real Data Factory / webhook workflow).

## Table Stakes (absorb from `az functionapp` + Azure MCP Server Functions tools)
- list/show function apps, list functions, show app settings, show plan, list slots, show config (host.json/runtime), show deployment status.
- All read-only; JSON-first; offline SQLite cache; `--select`/`--compact`/`--csv`/`--agent`.

## Data Layer
- **Primary entities:** subscriptions, resource_groups, function_apps, functions, app_settings, function_keys, hosting_plans, deployment_slots, invocations, metrics_daily, role_assignments.
- **Sync cursor:** ARM list pagers per resource; App Insights KQL windowed by time for invocations/metrics.
- **FTS/search:** across function_apps + functions + app_settings keys.
- Hot tables: `invocations` and `metrics_daily` (cold-start derivation, exec drift, failure clustering). Index `invocations(app_id, ts)` and `(app_id, result_code)`.

## Codebase Intelligence
- **Backbone SDKs:** `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appservice/armappservice` (v6.0.0 — `WebAppsClient.NewListPager`, `ListHostKeys`, `ListFunctions`, `ListApplicationSettings`), `armsubscriptions`, `armresources`; `sdk/azidentity` (`DefaultAzureCredential`); `sdk/monitor/azquery` (Log Analytics KQL for App Insights).
- **ARM REST (fallback/reference):** `GET .../Microsoft.Web/sites` (filter `kind`~functionapp), `/functions`, `/config/appsettings/list` (POST), `/host/default/listkeys` (POST), `/functions/{f}/listkeys` (POST), `/slots`, `Microsoft.Web/serverfarms` (plans). api-version `2024-11-01`.
- **Auth:** Azure AD. Env service principal `AZURE_TENANT_ID`/`AZURE_CLIENT_ID`/`AZURE_CLIENT_SECRET` + `AZURE_SUBSCRIPTION_ID`; falls back to `az login` token locally. Reader RBAC covers everything except `listkeys` (needs Website Contributor / `Microsoft.Web/sites/host/listkeys/action`).
- **Rate limiting:** ~12,000 ARM reads/hour per subscription; honor `429` + `Retry-After`.
- **Cold-start derivation:** App Insights `requests` table — detect gaps between invocations per `cloud_RoleName`, treat slow first-execution after a gap as a cold start; correlate with new `cloud_RoleInstance` appearances. (Confirmed as the practitioner-standard method; not a first-class metric.)

## User Vision (USER_BRIEFING_CONTEXT)
- **Beginner-first documentation is a core deliverable**, benchmarked against the live `cloud-run-admin` README. A small business that has never used Azure Functions should be able to install, set up auth, and get a useful first answer from the docs alone. Honest read-only scope (this CLI inspects/analyzes; it does NOT deploy — point newcomers to `func`/`az functionapp` for deploys). See seed §9.
- This CLI is the Azure analog to the user's real production app (`my-function-app`) and to the existing `cloud-run-admin` library entry — the user runs workloads on both Cloud Run and Azure Functions.

## Product Thesis
- **Name:** `azure-functions-admin` (binary `azure-functions-admin-pp-cli`), category `cloud/`.
- **Why it should exist:** `az`/portal can read individual resources, but nobody answers "are my functions cold-starting and should I change plans?" or "where are my secrets leaking across app settings?" Those need local time-series history + cross-app joins — impossible as a stateless `az` call, native to a SQLite-backed CLI. Plus agent-native output and beginner docs that the verbose `az` lacks.

## Build Priorities
1. **P0 foundation:** SQLite data layer for all entities; ARM sync pagers; App Insights KQL sync for invocations/metrics; search/SQL path.
2. **P1 absorb:** every read-only `az functionapp` + Azure MCP Server function feature — list/show apps, functions, settings, plans, slots, config, keys, invoke URLs — JSON-first, offline, `--agent`.
3. **P2 transcend (NOI):** `coldstart`, `scaling`, `plan-fit` (cold-start observatory), plus `drift`, `secrets-audit`, `failures`, `stale`.
4. **Docs:** rich beginner-first `narrative` (value_prop, step-by-step auth_narrative, cold-start-safe quickstart, teaching recipes, real-failure troubleshooting).

## Source Priority
- Single source (Azure ARM + App Insights). No combo. Multi-source gate skipped.
