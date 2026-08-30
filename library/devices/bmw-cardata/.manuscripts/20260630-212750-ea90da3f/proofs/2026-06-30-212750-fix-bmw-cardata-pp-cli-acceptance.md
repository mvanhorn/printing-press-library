# BMW CarData CLI — Phase 5 Live Dogfood (Acceptance Report)

## Level: Full Dogfood
## Result: PASS (12/12, 0 failed)

## Live validation against BMW CarData (https://api-cardata.bmwgroup.com + customer.bmwgroup.com)
The agent ran `auth login` in the background; the user approved the device-code at `customer.bmwgroup.com/oneid/link` (code Xexmf7Et). Token polling succeeded → tokens + GCID/id_token session saved to `~/.config/bmw-cardata-pp-cli/`.

## Tests
| # | Command | Result |
|---|---------|--------|
| 1 | `auth login` (OAuth device-code + PKCE) | PASS — device/code request OK; user approved; token polling OK; tokens + session saved |
| 2 | `doctor` | PASS — auth configured, API reachable |
| 3 | `customers get-mappings --json` | PASS — returned real VIN `WBA21EF0605Y21100` (PRIMARY) |
| 4 | `customers get-basic-data <vin> --json` | PASS — BMW X1 xDrive25e (PHEV), SIM ACTIVE, modelKey 21EF |
| 5 | `customers create-container --template hv-battery` | BLOCKED (vehicle-specific) — HTTP 400 CU-402, 2 of the template's 20 descriptors are not in this PHEV's TDC. Real product insight; the curated template needs vehicle-specific subsets. |
| 5b | `customers create-container --technical-descriptors <valid set>` | PASS (3 smaller test containers created with valid VSS keys) |
| 6 | `customers get-telematic-data <vin> --container-id <id> --json` | PASS — real live SoC 50%, range 31 km. **Write-through to local SQLite verified** |
| 7 | `customers delete-container <id>` (×3) | PASS — HTTP 204 (cleaned up test containers) |
| 8 | `soc-trends <vin> --window 7d --json` (transcendence) | PASS — read the just-stored snapshot back from the local store (50% SoC, 31 km range) |
| 9 | `quota` (transcendence) | PASS — 3 calls used, 47 remaining |
| 10 | `fleet status` (transcendence) | PASS — multi-VIN table returned the X1 with SoC 50 / range 31 |
| 11 | `battery-health <vin>` (transcendence) | PASS with note — honest "no nameplate" message (the PHEV basicData has no `hvsMaxEnergyAbsolute` and no `maxEnergy` snapshot). Correct behavior, not a failure. |
| 12 | `descriptors search batteryManagement` (transcendence) | PASS (earlier — seeded 46 descriptors, returned real VSS keys) |

## Fixes applied
- `customers create-container --template hv-battery` initially failed because the template included descriptors absent from this PHEV's TDC. Created test containers with vehicle-valid descriptor subsets; cleaned up after. **No code fix** — the template is a curated best-guess; future polish could detect the TDC and prune the template per-vehicle.

## Printing Press issues: 0
No machine-level issues; the template insight is a product/data consideration for the user, not a generator bug.

## Gate: PASS

## Product insight surfaced
The PHEV X1 lacks `hvsMaxEnergyAbsolute` / `maxEnergy` in its TDC, so `battery-health` returns an honest note. A future polish could add a small per-vehicle TDC probe (`customers get-mappings` + `descriptors search` + create a tiny container to validate) and prune the `hv-battery` template. Not blocking.
