#!/usr/bin/env bash
# auto-refresh-smoke.sh — smoke-test the GORGIAS_AUTO_REFRESH_TTL hook.
#
# What it proves: when the local mirror's `tickets` sync_state is older
# than GORGIAS_AUTO_REFRESH_TTL, a read against `--data-source local`
# triggers an opportunistic sync transparently. Until this test was
# written, the auto-refresh code path had never been observed running.
#
# Isolation: uses a temp HOME so the test never touches your real
# `~/.local/share/gorgias-pp-cli/data.db`.
#
# Live creds required for the sync side. If GORGIAS_API_KEY or
# GORGIAS_BASE_URL is unset, the test exits 0 with a "skip" message.
#
# Usage:
#   GORGIAS_USERNAME=... GORGIAS_API_KEY=... GORGIAS_BASE_URL=... \
#     bash scripts/auto-refresh-smoke.sh

set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO}"

if [ -z "${GORGIAS_USERNAME:-}" ] || [ -z "${GORGIAS_API_KEY:-}" ] || [ -z "${GORGIAS_BASE_URL:-}" ]; then
  echo "auto-refresh-smoke: SKIP (no credentials — set GORGIAS_USERNAME, GORGIAS_API_KEY, GORGIAS_BASE_URL to run)"
  exit 0
fi

BINARY="${REPO}/gorgias-pp-cli"
if [ ! -x "${BINARY}" ]; then
  echo "==> building gorgias-pp-cli"
  go build -o gorgias-pp-cli ./cmd/gorgias-pp-cli
fi

TMP_HOME=$(mktemp -d)
trap 'rm -rf "${TMP_HOME}"' EXIT
DB="${TMP_HOME}/.local/share/gorgias-pp-cli/data.db"
mkdir -p "$(dirname "${DB}")"

echo "==> seeding isolated mirror at ${DB}"
HOME="${TMP_HOME}" "${BINARY}" sync --resources tickets --max-pages 1 --agent > /dev/null

# Mark the tickets sync_state as stale (last_synced_at = 1 hour ago).
echo "==> marking sync_state as 1h stale"
sqlite3 "${DB}" "UPDATE sync_state SET last_synced_at = datetime('now', '-1 hour') WHERE resource_type = 'tickets'"

# Run a read with TTL=1m; the hook should detect staleness and refresh.
echo "==> running read with GORGIAS_AUTO_REFRESH_TTL=1m (expect 'auto-refresh: tickets' on stderr)"
OUT=$(HOME="${TMP_HOME}" GORGIAS_AUTO_REFRESH_TTL=1m \
  "${BINARY}" tickets list --data-source local --limit 1 --agent 2>&1 >/dev/null)

echo "----- captured stderr -----"
echo "${OUT}"
echo "---------------------------"

if echo "${OUT}" | grep -q 'auto-refresh: tickets'; then
  echo "==> PASS: auto-refresh fired as expected"
  exit 0
else
  echo "==> FAIL: no 'auto-refresh: tickets' line in stderr"
  exit 1
fi
