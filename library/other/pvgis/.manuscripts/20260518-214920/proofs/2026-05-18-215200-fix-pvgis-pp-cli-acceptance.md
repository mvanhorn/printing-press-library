# Phase 5 Live Acceptance Report — pvgis-pp-cli

## Summary
**Gate: PASS.** Level: Full Dogfood. Matrix size: 63 mechanical tests.

- Passed: 62/63 real-CLI behaviors.
- 1 test (`production hourly --json` JSON-fidelity) reports invalid JSON in dogfood's truncated `output_sample` field; the actual CLI stdout is valid 6.9 MB JSON containing 166,536 hourly rows under `results.outputs.hourly` (verified directly with `python3 json.load` and `len(rows)`).
- 4 fixture-dependent tests (`sites rank` and `weather similar`, both happy_path + json_fidelity) initially failed with `no such file or directory: sites.csv`. Added `sites.csv` (Milano/Roma/Napoli) to the CLI working directory; they now pass.

## Fixes applied during Phase 5
1. **Rebuilt the staged binary.** The build/stage/bin binary was older than the patched `production_hourly.go` source, so the `outputformat=json` default fix wasn't actually in the binary. After `go build -o build/stage/bin/pvgis-pp-cli ./cmd/pvgis-pp-cli`, hourly returns proper JSON (was returning CSV-in-string envelope before).
2. **Added `sites.csv` fixture.** Three real European cities to let `sites rank` and `weather similar` run against real PVGIS data.

## Behavioral smoke tests (manual)
- `radiation --lat 45 --lon 9 --horizontal 1 --json` → valid JSON; SARAH3 database; monthly H(h)_m values from 2005-2023.
- `production monthly --lat 45 --lon 9 --pnom 5 --system-loss 14 --tilt 30 --json` → valid JSON; outputs.totals.fixed.E_y = 6762.3 kWh/yr.
- `production hourly --lat 42 --lon 42 --json` → 166,536 hourly rows of valid JSON.
- `production optimal-tilt --lat 45 --lon 9 --azimuth 0 --json` → best_tilt found in 0-60° sweep.
- `production compare --lat 45 --lon 9 --pnom 1 --system-loss 14 --tilt 30 --json` → SARAH3 1352.46, ERA5 1320.47 kWh, NSRDB correctly errors out (out of EU coverage). Spread 2.4%.
- `sites diff --baseline 45.0,9.0 --target 41.9,12.5 --pnom 5 --system-loss 14 --tilt 30 --json` → Milano 6762, Roma 7421, delta +9.7% target_better=true.
- `sites rank --input sites.csv --tilt 30 --pnom 5 --system-loss 14 --json` → 3 cities ranked by e_y_per_kwp; Napoli > Roma > Milano (physically correct: lower lat → more sun).
- `weather similar --to 45.0,9.0 --within sites.csv --top 3 --json` → ranked by 12-month T+GHI signature distance.
- `weather --lat 45 --lon 9 --json` → 8760 TMY hourly rows.
- `horizon --lat 46.2 --lon 11.3 --json` → 49-point horizon profile.

All behavioral checks return the right shape and physically plausible numbers.

## Printing Press issues for retro
- **Dogfood `output_sample` truncation creates false JSON-fidelity failures for endpoints that return large JSON bodies.** The `production hourly` failure is purely a tooling artifact. Worth adding a "if output size > N MB, validate stream not sample" branch to dogfood.
- **Live dogfood test matrix runs Example invocations literally.** Example strings that reference fixture files (e.g. `sites.csv`, `rooftops.csv`) cause unavoidable failures unless dogfood is given a fixture directory or the CLI's working directory already contains the fixture. The current workaround (add the fixture to CLI workdir) bypasses this, but a Phase 5 contract for test fixtures would be cleaner.
- **Generated binary at `build/stage/bin/pvgis-pp-cli` is not re-built after manual source edits in `internal/cli/*.go`.** Shipcheck only runs `go build ./...`, not `go build -o build/stage/bin/`. Easy to miss; consider adding a post-edit hook or making dogfood rebuild the staged binary.

## Verdict
**Gate PASS — proceed to Phase 5.5 (Polish).**
