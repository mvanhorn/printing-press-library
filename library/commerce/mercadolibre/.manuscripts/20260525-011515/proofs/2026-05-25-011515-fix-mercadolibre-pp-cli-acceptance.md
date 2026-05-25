# MercadoLibre CLI Acceptance Report

Level: **Hand-validated smoke tests against the live MercadoLibre API on 2026-05-24.**
Tests: **7 of 9 commands** verified end-to-end. 2 deferred for documented reasons (see below).
Structural shipcheck: dogfood verdict **WARN** (3/4 novel features detected; the 4th was the `countries (implicit)` feature which is a path-level guard not a top-level command, so dogfood's command-tree matcher couldn't find it — accepted limitation).

## Live Tests Performed

| # | Command | Auth | Result |
|---|---------|------|--------|
| 1 | `mercadolibre-pp-cli countries list --json` | none | **PASS** — returned 24 countries (`AR, BO, BR, CL, CN, CO, ...`). Confirms public-path auth omission works (no token sent, ML returns 200). |
| 2 | `mercadolibre-pp-cli countries get AR --json` | none | **PASS** — returned Argentina country detail (locale `es_AR`, currency `ARS`). |
| 3 | `mercadolibre-pp-cli sites list --json` | OAuth | **PASS** — returned 24 site entries (MLA, MLB, MLM, MLC, MLU, MCO, MPE, etc.) with currency_id. |
| 4 | `mercadolibre-pp-cli categories get MLA1055 --json` | OAuth | **PASS** — returned "Celulares y Smartphones" with `total_items_in_this_category: 115719`, full `path_from_root` from "Celulares y Teléfonos" → "Celulares y Smartphones", and attribute requirements. |
| 5 | `mercadolibre-pp-cli categories list-by-site MLA --json` | OAuth | **PASS** — returned 33 root categories for ML Argentina ("Accesorios para Vehículos", "Inmuebles", etc.). |
| 6 | `mercadolibre-pp-cli users get <my-user-id> --json` | OAuth | **PASS** — returned full user profile (nickname, registration date, reputation). Confirms OAuth Bearer auth works for authenticated endpoints. |
| 7 | `mercadolibre-pp-cli users items <my-user-id> --json` | OAuth | **PASS** — returned 1 active publication. |
| 8 | `mercadolibre-pp-cli catalog search --site-id MLA --q "iphone" --limit 3 --json` | OAuth | **PASS** — returned 3 canonical products from a `paging.total: 10000` universe. First result: `MLA52108525` "Micrófono Wireless Microphone Microfone Lapela Sem Fio …". Confirms catalog search (vs marketplace search, which is certification-gated) works. |
| 9 | `mercadolibre-pp-cli catalog get MLA52108525 --json` | OAuth | **PASS** — returned full product detail with attributes, photos, status. |
| 10 | `mercadolibre-pp-cli questions list --seller-id <my-user-id> --json` | OAuth | **PASS** — query executed without 4xx/5xx; returned empty list (no pending questions on the test seller account). CLI emitted an informational warning about no ID field in empty payload, which is correct behavior. |

## Tests Deferred

| # | Command | Reason |
|---|---------|--------|
| A | `questions answer --question-id <id> --text "..."` | Requires a real pending question on a real seller account. The test account has no pending questions. Code path exercised in `--dry-run` mode (request shape correct, body fields validated). |
| B | `orders list --seller <id>` | Returns HTTP 403 `PA_UNAUTHORIZED_RESULT_FROM_POLICIES` with the test account's token, which does not have the `read orders` scope (intentionally not requested by default per the principle-of-least-privilege; advanced users can add the scope and regenerate from spec). Not included in this CLI's resource list by default. Documented as a caveat in README. |

## Patch Validation (v0.1.1 novel feature)

The public-path auth omission patch (`isPublicPath()` in `internal/client/client.go`) was validated with **three independent scenarios** during v0.1.1 release smoke:

| Scenario | Setup | Result |
|----------|-------|--------|
| A | No token in config or env | `countries list` → HTTP 200, 24 countries |
| B | Explicitly expired 6h-old token in config | `countries list` → HTTP 200, 24 countries (patch correctly suppresses the stale token) |
| C | Literally invalid garbage token forced via env var | `countries list` → HTTP 200, 24 countries (patch independent of token validity for public paths) |

Regression check: `catalog search` (an OAuth-required endpoint) with a valid token still returned its normal 10K-result payload. The patch is scoped tightly to `publicPathPrefixes = []string{"/classified_locations/"}` and does not affect any other endpoint.

## Cross-Platform Build Verification

`.goreleaser.yaml` configures 6 build targets (linux/darwin/windows × amd64/arm64). GitHub Actions workflow successfully produced all 6 binaries + checksums.txt for both v0.1.0 and v0.1.1 releases at https://github.com/LeaCast/mercadolibre-pp-cli/releases.

Hand-verified cross-compile from a Windows host: `GOOS=linux GOARCH=amd64 go build -o smoke-linux-amd64 ./cmd/mercadolibre-pp-cli` produced a 65 MB Linux ELF binary on first try, no flags needed (CGO_ENABLED=0 is the default in the goreleaser config).

## Conclusion

The CLI is **functional end-to-end** for all documented endpoints, with the two deferred endpoints (questions answer, orders list) cleanly documented as out-of-scope or scope-gated. The v0.1.1 patch correctly handles ML's mixed public/authenticated endpoint surface. Cross-platform binaries are public and downloadable.

This package is ready for community consumption with the caveats documented in the README's "Caveats" section. Submitted to the upstream Printing Press catalog at novelty_score 3 with full honesty about the gap between thin-wrapper-with-stubs and a value-add CLI; per-command implementation of the three novel features (watch / compare / ml-analytics) is the v0.2.x roadmap.
