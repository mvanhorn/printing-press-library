#!/usr/bin/env bash
set -euo pipefail

# Usage: scripts/verify-live.sh <research-run-dir> <proofs-dir>
# All source reads are public; all fixture state is disposable.
task_cli_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$task_cli_root"
task_research_run="${1:?Pass the run directory containing research.json}"
task_proofs="${2:?Pass the directory for acceptance and matrix evidence}"
task_press="${PRINTING_PRESS_BIN:-$(command -v cli-printing-press)}"
task_cli="$task_cli_root/build/stage/bin/snowjapan-pp-cli"
task_fixture="$(mktemp -d "${TMPDIR:-/tmp}/snowjapan-live-fixture.XXXXXX")"
trap 'rm -rf "$task_fixture"' EXIT
task_db="$task_fixture/facts.sqlite"
mkdir -p "$task_proofs"

go build -o "$task_cli" "$task_cli_root/cmd/snowjapan-pp-cli"
"$task_cli" sync --resources resorts,seasons \
  --resource-param seasons:season=2025-2026 \
  --db "$task_db" --home "$task_fixture/home" --no-learn --agent \
  > "$task_proofs/fixture-capture.json"
for task_observation in first second; do
  "$task_cli" sync --resources resorts \
    --resorts nagano-prefecture/hakuba-village/able-hakuba-goryu \
    --db "$task_db" --home "$task_fixture/home" --no-learn --agent \
    > "$task_proofs/fixture-detail-$task_observation.json"
done

"$task_cli" sync --resources reports --reports hakuba-now-1st-october-2026 \
  --db "$task_db" --home "$task_fixture/home" --no-learn --agent \
  > "$task_proofs/fixture-report-capture.json"
"$task_cli" reports get hakuba-now-1st-october-2026 --data-source local \
  --db "$task_db" --home "$task_fixture/home" --no-learn --agent \
  > "$task_proofs/fixture-report-local.json"
"$task_cli" resorts get able-hakuba-goryu --data-source local \
  --db "$task_db" --home "$task_fixture/home" --no-learn --agent \
  > "$task_proofs/fixture-resort-local.json"

# This opt-in changes test annotation arguments only. Normal users retain
# portable examples, the normal database path and explicit sync semantics.
PP_SNOWJAPAN_FIXTURE_DB="$task_db" "$task_press" dogfood --live \
  --dir "$task_cli_root" --level full --allow-destructive \
  --research-dir "$task_research_run" --json \
  --write-acceptance "$task_proofs/phase5-acceptance.json" \
  > "$task_proofs/2026-10-03-dogfood-results.json" \
  2> "$task_proofs/2026-10-03-dogfood-results.stderr"
