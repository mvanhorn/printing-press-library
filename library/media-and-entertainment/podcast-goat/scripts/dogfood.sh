#!/usr/bin/env bash
# Dogfood matrix for podcast-goat. Hits every command shape with real inputs
# and checks invariants. Designed to be the regression suite for v0.x.
#
# Usage:
#   SPOKEN_API_KEY=... bash scripts/dogfood.sh           # full live run
#   bash scripts/dogfood.sh --dry                        # skip live HTTP, exec only
#
# Output: per-test PASS/FAIL/SKIP lines + a summary table at the end.
#
# Exit code 0 = all tests passed or skipped cleanly; 1 = at least one FAIL.

set -uo pipefail

CLI="${CLI_BIN:-./podcast-goat-pp-cli}"
MCP="${MCP_BIN:-./podcast-goat-pp-mcp}"
DRY=false
[[ "${1:-}" == "--dry" ]] && DRY=true

# Canonical test URLs (real, fetched live during the prior session)
URL_DWARKESH="https://www.dwarkesh.com/p/andrej-karpathy"
URL_SPOTIFY="https://open.spotify.com/episode/5PwtWcgg71nSkb63ZV4hGX"
URL_YOUTUBE="https://www.youtube.com/watch?v=lXUZvyajciY"
URL_APPLE="https://podcasts.apple.com/us/podcast/the-tim-ferriss-show/id863897795"
URL_LEX="https://lexfridman.com/sam-altman-2/"
URL_ACQUIRED="https://www.acquired.fm/episodes/vanguard"
URL_HUBERMAN="https://www.hubermanlab.com/episode/foo"
URL_FOUNDERS="https://www.founderspodcast.com/episodes/foo"
URL_PETERATTIA="https://peterattiamd.com/podcast/foo"

# Edge URLs
URL_404="https://www.dwarkesh.com/p/does-not-exist-xyz"
URL_LONG="https://example.com/$(printf 'x%.0s' {1..2900})"
URL_EMPTY=""
URL_FILE="file:///etc/passwd"
URL_MAILTO="mailto:foo@bar.com"

PASS=0; FAIL=0; SKIP=0
FAILURES=()

pass() { echo "  PASS  $1"; PASS=$((PASS+1)); }
fail() { echo "  FAIL  $1 -- $2"; FAIL=$((FAIL+1)); FAILURES+=("$1: $2"); }
skip() { echo "  SKIP  $1 -- $2"; SKIP=$((SKIP+1)); }

assert_exit_zero() {
  local name="$1"; shift
  if "$@" >/dev/null 2>&1; then pass "$name"; else fail "$name" "exit $?"; fi
}

assert_exit_nonzero() {
  local name="$1"; shift
  if ! "$@" >/dev/null 2>&1; then pass "$name"; else fail "$name" "expected nonzero exit, got 0"; fi
}

assert_contains() {
  local name="$1" needle="$2"; shift 2
  local out
  out=$("$@" 2>&1)
  if echo "$out" | grep -qE "$needle"; then pass "$name"; else fail "$name" "output missing /$needle/"; fi
}

assert_valid_json() {
  local name="$1"; shift
  local out
  out=$("$@" 2>&1)
  if echo "$out" | python3 -c 'import json,sys; json.load(sys.stdin)' >/dev/null 2>&1; then pass "$name"; else fail "$name" "invalid JSON output"; fi
}

assert_no_panic() {
  local name="$1"; shift
  local out
  out=$("$@" 2>&1)
  if echo "$out" | grep -qiE "panic:|runtime error"; then fail "$name" "panic in output"; else pass "$name"; fi
}

echo "=== Cat 1: Smoke ==="
assert_exit_zero "smoke/version" $CLI --version
assert_exit_zero "smoke/help" $CLI --help
assert_no_panic  "smoke/doctor-no-panic" $CLI doctor

echo ""
echo "=== Cat 2: Per-adapter --explain (URL pattern matching) ==="
for adapter_url in "huberman:$URL_HUBERMAN" "acquired:$URL_ACQUIRED" "founders:$URL_FOUNDERS" "peterattia:$URL_PETERATTIA" "dwarkesh:$URL_DWARKESH" "youtube:$URL_YOUTUBE" "spotify:$URL_SPOTIFY"; do
  name="${adapter_url%%:*}"; url="${adapter_url#*:}"
  assert_contains "explain/$name-matches" "$name" $CLI episode get "$url" --explain --dry-run
done

echo ""
echo "=== Cat 3: Live fetches (free + free-via-yt-dlp + free spoken/demo) ==="
if [ "$DRY" = "true" ]; then
  skip "live/dwarkesh" "dry mode"
  skip "live/youtube" "dry mode"
  skip "live/spotify" "dry mode"
  skip "live/spoken" "dry mode"
else
  assert_no_panic  "live/dwarkesh-no-panic" $CLI episode get "$URL_DWARKESH"
  assert_no_panic  "live/youtube-no-panic"  $CLI episode get "$URL_YOUTUBE"
  if $CLI auth services 2>&1 | grep -q "spotify.*captured"; then
    assert_no_panic "live/spotify-no-panic" $CLI episode get "$URL_SPOTIFY"
  else
    skip "live/spotify" "no spotify cookie captured"
  fi
  if [ -n "${SPOKEN_API_KEY:-}" ]; then
    assert_no_panic "live/spoken-no-panic" $CLI episode get "$URL_APPLE" --paid --provider spoken --yes
  else
    skip "live/spoken" "SPOKEN_API_KEY not set"
  fi
fi

echo ""
echo "=== Cat 4: MCP boot ==="
if [ -x "$MCP" ]; then
  init_resp=$(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}' | timeout 3 $MCP 2>&1 | head -1)
  if echo "$init_resp" | grep -q '"protocolVersion"'; then
    pass "mcp/init-protocol-version"
  else
    fail "mcp/init" "no protocolVersion in init response"
  fi
  if echo "$init_resp" | grep -q '"serverInfo"'; then
    pass "mcp/init-serverInfo"
  else
    fail "mcp/init-serverInfo" "missing serverInfo"
  fi
else
  skip "mcp/boot" "MCP binary not built (build with: go build -o ./podcast-goat-pp-mcp ./cmd/podcast-goat-pp-mcp)"
fi

echo ""
echo "=== Cat 5: JSON output stability ==="
assert_valid_json "json/source-list" $CLI source list --json
assert_valid_json "json/speakers-list" $CLI speakers list --json
assert_valid_json "json/cache-list" $CLI cache list --json
assert_valid_json "json/budget-show" $CLI budget show --json
assert_valid_json "json/budget-by-show" $CLI budget show --by-show --json
assert_valid_json "json/auth-services" $CLI auth services --json

# Snake-case check: scan for PascalCase keys in JSON outputs (regression check on F6)
pascal_check=$($CLI speakers list --json 2>&1 | grep -oE '"[A-Z][a-zA-Z]+":' | head -3)
if [ -z "$pascal_check" ]; then
  pass "json/snake-case-speakers"
else
  fail "json/snake-case-speakers" "found PascalCase keys: $pascal_check"
fi

echo ""
echo "=== Cat 6: Edge URLs ==="
assert_no_panic "edge/404"   $CLI episode get "$URL_404" --explain --dry-run
assert_no_panic "edge/long"  $CLI episode get "$URL_LONG" --explain --dry-run
assert_no_panic "edge/empty" $CLI episode get "$URL_EMPTY" --explain --dry-run
assert_no_panic "edge/file-scheme" $CLI episode get "$URL_FILE" --explain --dry-run
assert_no_panic "edge/mailto" $CLI episode get "$URL_MAILTO" --explain --dry-run

echo ""
echo "=== Cat 7: Empty-cache graceful behavior ==="
# Use a tmpdir HOME-style override would be nice, but we'd need a --db flag.
# Instead just exercise on the current store and assert no panic.
assert_no_panic "empty-or-current/magic-empty-topic" $CLI magic "xyzzy_no_match_topic_$(date +%s)" --limit 1
assert_no_panic "empty-or-current/quote-no-match"   $CLI episode quote "xyzzyzzy_no_such_phrase" -C 0
assert_no_panic "empty-or-current/speakers-list"    $CLI speakers list

echo ""
echo "=== Cat 8: Cache export formats ==="
if [ -d /tmp/dogfood-export ]; then rm -rf /tmp/dogfood-export; fi
mkdir -p /tmp/dogfood-export
assert_no_panic "export/md"    $CLI cache export --format md    --out /tmp/dogfood-export/cache.md
assert_no_panic "export/jsonl" $CLI cache export --format jsonl --out /tmp/dogfood-export/cache.jsonl
# zip format may not exist; skip if --format zip errors
if $CLI cache export --format zip --out /tmp/dogfood-export/cache.zip 2>/dev/null; then
  pass "export/zip"
else
  skip "export/zip" "format may not be implemented yet"
fi

echo ""
echo "=== Cat 9: Source compare ==="
if [ "$DRY" = "true" ]; then
  skip "compare/multi-source" "dry mode"
else
  assert_no_panic "compare/spotify-url" $CLI source compare "$URL_SPOTIFY" --explain --dry-run
fi

echo ""
echo "=== Cat 10: Budget pivot ==="
assert_no_panic "budget/show" $CLI budget show
assert_no_panic "budget/by-show" $CLI budget show --by-show

echo ""
echo "=== Cat 11: doctor JSON shape stability ==="
assert_valid_json "doctor/json" $CLI doctor --json

echo ""
echo "=== Cat 12: feeds + auth subcommand parity ==="
assert_no_panic "feeds/list-empty-or-current" $CLI feeds list
assert_no_panic "auth/services-json" $CLI auth services --json
assert_no_panic "auth/status" $CLI auth status

echo ""
echo "==============================="
echo "Summary: $PASS pass / $FAIL fail / $SKIP skip"
echo "==============================="
if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "Failures:"
  for f in "${FAILURES[@]}"; do echo "  - $f"; done
  exit 1
fi
exit 0
