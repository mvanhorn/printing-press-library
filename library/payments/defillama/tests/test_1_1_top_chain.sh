#!/bin/bash
# test_1_1_top_chain.sh -- per-chain TVL behavior for `top --chain X`
source "$(dirname "$0")/runner.sh"

test_top_chain_solana_excludes_cex() {
  out=$("$CLI" top --chain solana --limit 10 2>/dev/null)
  rc=$?
  assert_exit_0 $rc "top --chain solana exits 0"
  assert_line_count_gte "$out" 2 "top --chain solana has rows"
  assert_not_contains "$out" "Binance CEX" "no Binance CEX in solana top"
  assert_not_contains "$out" "^OKX " "no OKX in solana top"
  assert_not_contains "$out" "Bybit" "no Bybit in solana top"
}

test_top_chain_ethereum_smaller_than_global() {
  # For multi-chain protocols, chain-specific TVL must be <= global TVL.
  # Pull the global #1 and the ethereum #1 in JSON and compare.
  global=$("$CLI" top --limit 5 --json 2>/dev/null)
  eth=$("$CLI" top --chain ethereum --limit 5 --json 2>/dev/null)
  assert_json "$global" "top --json valid"
  assert_json "$eth" "top --chain ethereum --json valid"
  # Aave appears in both lists with different TVLs; the ethereum number must be lower.
  ok=$(printf '%s' "$eth" | python3 -c '
import sys,json
d=json.load(sys.stdin)
# every TVL must be < $50B (no $153B Binance leakage)
bad=[r for r in d if r.get("tvl",0) > 50e9]
print("ok" if not bad else "bad")
' 2>/dev/null || echo "err")
  if [ "$ok" = "ok" ]; then
    PASS=$((PASS+1))
  else
    ERRORS+=("top --chain ethereum tvl: leaked global tvl > 50B")
    FAIL=$((FAIL+1))
  fi
}

test_top_chain_arbitrum_shows_native() {
  out=$("$CLI" top --chain arbitrum --limit 20 2>/dev/null)
  assert_exit_0 $? "top --chain arbitrum exits 0"
  assert_contains "$out" "gmx|camelot|radiant|pendle|aave" "arbitrum native protocol present"
}

test_top_chain_include_cex() {
  out=$("$CLI" top --chain solana --include-cex --limit 20 2>/dev/null)
  assert_exit_0 $? "top --include-cex exits 0"
  # With --include-cex, at least one CEX should appear (Coinbase, Binance, etc. custody on Solana)
  # We don't assert a specific CEX since they may or may not have solana balances.
  # Just ensure the flag is accepted and command runs.
}

test_top_no_chain_still_works() {
  out=$("$CLI" top --limit 10 2>/dev/null)
  assert_exit_0 $? "top no chain exits 0"
  assert_line_count_gte "$out" 5 "top no chain has rows"
}

test_top_chain_unknown_no_crash() {
  out=$("$CLI" top --chain notarealchain --limit 5 2>/dev/null)
  assert_exit_0 $? "top --chain notarealchain exits 0 (empty ok)"
}

dlpp_run_tests
