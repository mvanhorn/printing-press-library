#!/usr/bin/env bash
# Run the dice-fm test suite under the Go data-race detector.
#
# The 2026-06-08 whole-project review flagged that `go test -race
# ./internal/cli/` was not a routine gate; a test that opened the operator's
# real ~70k-row default-path store made the race build time out, masking the
# bar. That test is now isolated to a temp store, and the package-global output
# toggles (--no-color/--human-friendly) were moved off mutable package state, so
# the suite is race-clean. This script keeps `-race` a one-command local gate.
#
# CI for the public library is generator/library-owned (per-CLI workflows can't
# be added), so this is the local equivalent until an upstream `-race` step
# lands. Run from the CLI module root:  bash scripts/test-race.sh
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> go build ./..."
go build ./...

echo "==> go vet ./..."
go vet ./...

echo "==> go test -race -count=1 ./..."
go test -race -count=1 ./...

echo "==> race-clean"
