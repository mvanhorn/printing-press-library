# Azure Functions Admin — Absorb Manifest

Sources catalogued: `az functionapp` (Azure CLI), Azure MCP Server (Functions tools), `func` Core Tools (deploy — out of scope), Azure portal Functions blade, `armappservice` Go SDK surface.

## Absorbed (match or beat everything that exists — all READ-ONLY)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List function apps in subscription/RG | `az functionapp list` / Azure MCP | `azure-functions-admin apps list` | Filters `kind~functionapp`, JSON-first, offline SQLite, `--select` |
| 2 | Show one function app | `az functionapp show` | `azure-functions-admin apps get <name>` | Compact high-gravity fields, plan tier resolved inline |
| 3 | List functions in an app | `az functionapp function list` | `azure-functions-admin functions list --app <name>` | Trigger type surfaced, offline |
| 4 | Show a function | `az functionapp function show` | `azure-functions-admin functions get --app <name> <fn>` | Invoke URL + binding summary |
| 5 | List app settings | `az functionapp config appsettings list` | `azure-functions-admin settings list --app <name>` | Classifies each as plaintext vs `@Microsoft.KeyVault` ref |
| 6 | Show site config (runtime, timeout) | `az functionapp config show` | `azure-functions-admin config show --app <name>` | Surfaces host.json-level: runtime, OS, timeout, extension bundle |
| 7 | List host + function keys | `az functionapp keys list` / `function keys list` | `azure-functions-admin keys --app <name>` | Masked by default; invoke URLs; flags RBAC need (Website Contributor) |
| 8 | List hosting plans | `az functionapp plan list` | `azure-functions-admin plans list` | Tier decode: Y1=Consumption, EP*=Premium, P*=Dedicated |
| 9 | List deployment slots | `az functionapp deployment slot list` | `azure-functions-admin slots list --app <name>` | Per-slot settings/keys aware (feeds drift) |
| 10 | Show deployment status | `az functionapp deployment ...` | `(generated endpoint) deployments list` | Read-only deployment history |
| 11 | List subscriptions | `az account list` | `azure-functions-admin subscriptions list` | Credential-discovered; no hardcoded sub |
| 12 | Show managed identity | `az functionapp identity show` | `(behavior in azure-functions-admin apps get --identity)` | Surfaces principal + type for RBAC context |
| 13 | List role assignments on app | `az role assignment list` | `azure-functions-admin roles --app <name>` | Scopes RBAC surface to the function app |
| 14 | Invocation/metrics read | App Insights portal / `az monitor` | `azure-functions-admin metrics --app <name>` | KQL-backed, local cache, time-windowed |
| 15 | Cross-resource search | (none — portal only) | `azure-functions-admin search <term>` | Offline FTS across apps/functions/settings |
| 16 | Raw SQL over local store | (none) | `azure-functions-admin sql "<query>"` | Composable analysis, agent-native |
| 17 | Local sync | (none — `az` is stateless) | `azure-functions-admin sync` | Builds the SQLite store that makes transcendence possible |
| 18 | Health/auth check | (none) | `azure-functions-admin doctor` | Validates creds, subscription access, RBAC; verify-safe |

## Transcendence (only possible with our local-history + cross-join approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Cold-start observatory | `coldstart --app <name>` | hand-code | Cold start is not a first-class metric; must derive from App Insights `requests` gaps + new `cloud_RoleInstance` over local time-series. Stateless `az` cannot. | Use to measure cold-start frequency + p50/p95 latency and the projected cost of moving to Premium always-on. Do NOT use for live invocation tailing; use `failures`. |
| 2 | Scaling/exec-drift trend | `scaling --app <name> --window 7d` | hand-code | Instance-count + exec-duration drift across a window needs persisted history no single API call returns. | Use to see whether p95 execution time or scale-out is creeping. |
| 3 | Plan-fit recommendation | `plan-fit --resource-group <rg>` | hand-code | Recommends Consumption/Premium/Dedicated per app from invocation density + cold-start sensitivity + duration — a join across plans, metrics, and history. | Use to answer "should I move off Consumption?" across a whole resource group. |
| 4 | Config-drift across apps/slots | `drift --resource-group <rg>` | hand-code | Diffs app settings across apps AND slots and flags plaintext-vs-KeyVault — cross-entity comparison `az` can't do in one call. | Use to find settings missing in one env or secrets that should be Key Vault refs. |
| 5 | Secret-sprawl audit | `secrets-audit --app <name>` | hand-code | Flags raw-secret app settings vs `@Microsoft.KeyVault` refs and stale/unused keys — needs the full local settings+keys join. | Use to find leaked plaintext secrets and unused function keys. Directly targets the key-in-a-doc problem. |
| 6 | Invocation failure clustering | `failures --app <name> --since 24h` | hand-code | Clusters App Insights failures by function + exception type over a window; requires local invocation history. | Use for triage: which function + exception is failing most. |
| 7 | Stale-function finder | `stale --days 90` | hand-code | Functions with zero invocations in N days — needs persisted invocation history to know "zero." | Use to find cleanup candidates. |

All transcendence rows ≥5/10 and grounded in: (a) the user's real `my-function-app` deployment, (b) Phase 1 research confirming cold-start derivation and plan-tier decode, (c) the documented gap (no read-only Azure Functions analysis CLI exists).

## Out of scope (read-only posture)
create/update/delete sites, zip/`func` deploy, restart/start/stop, scale ops, slot swaps, set/remove app settings, rotate/create keys, role-assignment changes.

## Stubs
None. Every row ships fully or is a `(generated endpoint)`/`(behavior in ...)` disposition. The `keys` command ships fully but documents that it requires Website Contributor RBAC.
