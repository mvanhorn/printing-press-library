#!/bin/bash
# test_2_7_skill.sh -- SKILL.md presence and content.
source "$(dirname "$0")/runner.sh"

SKILL="$DLPP_REPO_ROOT/SKILL.md"

test_01_skill_exists() {
  if [ -f "$SKILL" ]; then PASS=$((PASS+1)); else ERRORS+=("SKILL.md missing"); FAIL=$((FAIL+1)); fi
}

test_02_contains_binary_name() {
  content=$(cat "$SKILL" 2>/dev/null)
  assert_contains "$content" "defillama-pp-cli" "binary name mentioned"
}

test_03_contains_sql() {
  content=$(cat "$SKILL" 2>/dev/null)
  assert_contains "$content" "sql" "sql escape hatch documented"
}

test_04_contains_sync() {
  content=$(cat "$SKILL" 2>/dev/null)
  assert_contains "$content" "sync" "sync documented"
}

test_05_contains_schema_tables() {
  content=$(cat "$SKILL" 2>/dev/null)
  assert_contains "$content" "protocol_chain_tvl" "protocol_chain_tvl table documented"
  assert_contains "$content" "fees_overview" "fees_overview documented"
  assert_contains "$content" "pools" "pools documented"
}

test_06_contains_scope_boundaries() {
  content=$(cat "$SKILL" 2>/dev/null)
  assert_contains "$content" "wallet|Nansen|Arkham" "what DefiLlama does not cover"
}

test_07_no_wrong_column_names() {
  content=$(cat "$SKILL" 2>/dev/null)
  assert_not_contains "$content" "total_24h_revenue" "old wrong column total_24h_revenue absent"
}

dlpp_run_tests
