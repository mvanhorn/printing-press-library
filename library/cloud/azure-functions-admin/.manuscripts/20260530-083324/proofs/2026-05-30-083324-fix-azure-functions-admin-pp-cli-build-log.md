# Azure Functions Admin — Build Log

Manifest transcendence rows: 7 planned, 7 built. All 7 LIVE-VALIDATED against real Azure (Example Org "my-subscription" subscription, app my-function-app) on 2026-05-30.

## Live validation results (read-only, $0, user-authorized target my-function-app)
- plan-fit ✅ 4 apps, correct tier classification (Y1→Consumption×3, B1→Dedicated)
- secrets-audit ✅ found 4 plaintext secret-suspects (api_token, pwd, WEBSITE_CONTENTAZUREFILECONNECTIONSTRING, APPINSIGHTS_INSTRUMENTATIONKEY), 0 KV refs, names-only
- drift ✅ single-app RG, no drift, plaintext flagged
- coldstart ✅ auto-resolved AI component by ikey; 30 cold starts, p50/p95 durations
- scaling ✅ per-day buckets, instances + p95
- failures ✅ found 29× HTTP 500 + 1× 499 on my-function-app (real operational finding: app is failing)
- stale ✅ ARM-declared fn matched to AI operation_Name, 0 stale

Auth path validated: DefaultAzureCredential via `az login` session (user@example.com), ARM token injection works for absorbed commands, output matches `az functionapp list` ground truth. EXAMPLE identity lacked Microsoft.Web/sites/read; user switched to Example Org tenant which has readable function apps.

## Phase 3 — App Insights commands (4/4 built, azquery KQL-backed)
- `internal/cli/appinsights.go`: resolveAppInsightsID (app→ikey→match AI component, or --app-insights-id override), runKQL (QueryResource), tableToMaps, kqlWindow (regex-validated to prevent KQL injection).
- `internal/azure/azure.go`: added AppInsightsComponentsClient.
- coldstart/scaling/failures/stale: gap-based cold-start derivation, hourly scaling buckets, failed-request clustering, ARM-functions vs AI-operations stale detection. All build/vet/dry-run clean + live-validated.
- Known nuance (acceptable v1): for low-frequency timer triggers every invocation reads as a cold start (true — app scaled to zero), and coldstart p50/p95 reflect total request duration not isolated cold-start overhead. Honest; refine later.

## Phase 3 — Transcendence commands (3/7 built, armappservice-backed)
- `secrets-audit` ✅ — ListApplicationSettings → classify KeyVault-ref vs plaintext-secret-suspect (secretKeyHint heuristic). Auto-resolves resource group from app name. Build/vet/test/help/dry-run clean.
- `drift` ✅ — NewListByResourceGroupPager → compare app-settings keys across apps in an RG; reports drifted keys (present_in/missing_from) + plaintext secrets. Clean.
- `plan-fit` ✅ — PlansClient + WebAppsClient join; resolve each app's ServerFarmID → SKU → ClassifyPlanTier → honest tier-aware recommendation. Clean.
- Helpers in hand-authored `internal/cli/azure_helpers.go` (azStr, rgFromID, isFunctionApp, appServiceClient, listFunctionApps, findFunctionApp, secretKeyHint, isKeyVaultRef, emitView).
- ARM token injection added to generated `internal/cli/root.go` newClient (cfg.AuthHeaderVal = azure.ARMBearer when empty) so absorbed HTTP commands authenticate. **Regen note:** edits a generated file; flag for review on future regen-merge.

REMAINING transcendence (4/7) — App Insights / azquery KQL: `coldstart`, `scaling`, `failures`, `stale`. Each needs to resolve the app's APPLICATIONINSIGHTS_CONNECTION_STRING → AI resource → KQL over the requests/exceptions tables (cold start derived from request gaps + new cloud_RoleInstance).

## Phase 2 — Generate (DONE)
- Generated from hand-authored internal YAML spec. All generation gates PASS (go mod tidy, govulncheck, go vet, go build, binary run, --help/version/doctor).
- Surface: absorbed resource commands (subscriptions/apps/functions/settings/plans/slots) + framework (search/sql/sync/doctor/auth/MCP) + 7 transcendence commands scaffolded as placeholders from research.json novel_features.
- README 389L + SKILL 325L rendered from beginner-first narrative.

## Phase 3 — Auth foundation (DONE)
Hand-authored durable package `internal/azure/` (no generated header; survives regen):
- `azure.go`: `Credential()` (azidentity.DefaultAzureCredential — env SP or `az login`), `SubscriptionID(explicit)` (flag > AZURE_SUBSCRIPTION_ID > actionable error), `AppServiceFactory(sub)` (armappservice v6 ClientFactory), `SubscriptionsClient()` (armsubscriptions), `LogsClient()` (azquery for App Insights KQL), `ARMBearer(ctx)` (raw-HTTP token for listkeys POSTs).
- `plan.go`: pure `ClassifyPlanTier(sku)` (Y1→Consumption, EP*→Premium, P*/S*/B*/I*/WS*→Dedicated; "P" is Dedicated not Premium) + `PlanTier.HasColdStarts()`.
- `plan_test.go`: table-driven tests for both — PASS.
- Deps added: azcore, azidentity, armappservice/v6, armsubscriptions, azquery (+ transitive). `go build ./internal/azure` OK, `go test ./internal/azure` ok.

Architectural decision: real commands use the armappservice/azquery SDKs (which manage token acquisition/refresh via the credential) rather than the generated static-token HTTP client, whose `AuthHeader()` returns a garbage `Bearer <tenant-id>` for Azure. The generated HTTP client is retained only for raw listkeys POSTs via ARMBearer + GetWithHeaders.

## REMAINING (Phase 3 cont.)
- Wire absorbed commands (subscriptions/apps/functions/settings/plans/slots) RunE to the SDK (currently hit the broken generated client path).
- Implement 7 transcendence RunE bodies: coldstart/scaling/plan-fit (App Insights KQL + plan tier), drift/secrets-audit (settings+keys joins), failures (KQL clustering), stale (invocation history).
- Wire local store sync for the SDK-fetched entities.
- Then Phase 4 shipcheck.
