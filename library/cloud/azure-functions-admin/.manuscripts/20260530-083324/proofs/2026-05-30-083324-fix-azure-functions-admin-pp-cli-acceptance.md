# Azure Functions Admin — Phase 5 Acceptance (manual live validation)

**Level:** Manual live dogfood (all 7 novel commands + auth) against real Azure.
**Why not the auto matrix:** `dogfood --live` was intentionally declined — the auto matrix sweeps app settings across every app in the shared subscription, exceeding the single app (`my-function-app`) the user authorized for reads. The user's read-only/named-target guardrail is respected. Manual validation scoped to the authorized target stands in.

**Environment:** `az login` session, account `user@example.com`, subscription "my-subscription" `00000000-...`. DefaultAzureCredential via Azure CLI token. Read-only, $0.

## Results (all PASS, exit 0)
| Command | Live evidence |
|---|---|
| auth/credential | DefaultAzureCredential acquired ARM token from az session; output matched `az functionapp list` ground truth |
| `apps list` (absorbed) | ARM-token injection path returned subscription apps, matching `az` |
| `plan-fit` | 4 apps; correct tiers: Y1→Consumption×3, B1→Dedicated(my-function-app); has_cold_starts correct |
| `secrets-audit --app my-function-app` | 16 settings, 0 KV refs, 4 plaintext suspects (api_token, pwd, WEBSITE_CONTENTAZUREFILECONNECTIONSTRING, APPINSIGHTS_INSTRUMENTATIONKEY); key names only, no values |
| `drift --resource-group my-resource-group` | single-app RG, no drift, plaintext flagged correctly |
| `coldstart --app my-function-app` | auto-resolved AI component by ikey; 30 cold starts, p50/p95 durations returned |
| `scaling --app my-function-app` | per-day buckets: instances + requests + p95 |
| `failures --app my-function-app --since 30d` | clustered 29× HTTP 500 + 1× 499 on my-function-app |
| `stale --app my-function-app --days 30` | ARM-declared function matched to AI operation_Name; 0 stale |

## Structural gates (Phase 4 shipcheck)
verify 100%, validate-narrative PASS, dogfood PASS, workflow-verify PASS, verify-skill PASS, scorecard 88/100 Grade A. go vet 0, gosec 0 in hand-authored code, PII-audit clean (EXAMPLE client names scrubbed).

**Gate: PASS** (manual live behavioral validation + green structural shipcheck). Note: formal `phase5-acceptance.json` auto-marker not generated because the runner's broad scope conflicts with the user's named-target read constraint; this document is the scoped acceptance evidence.
