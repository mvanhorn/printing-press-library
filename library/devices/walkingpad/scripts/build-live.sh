#!/usr/bin/env bash
# Build walkingpad-pp-cli WITH the live BLE backend so it can control real
# hardware. The default `go build`/`go install` produces a pure-Go binary whose
# live commands print a "rebuild with -tags wp_live" notice; this script links
# the tinygo BLE backend (CGO required).
set -euo pipefail

cd "$(dirname "$0")/.."

OUT="${1:-./walkingpad-pp-cli}"

echo "Building live BLE binary (CGO + tinygo bluetooth) -> $OUT"
CGO_ENABLED=1 go build -tags wp_live -trimpath -o "$OUT" ./cmd/walkingpad-pp-cli
echo "Done. Verify with: $OUT doctor --live"
