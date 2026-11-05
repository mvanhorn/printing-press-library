# Shipcheck Report: yahoo-fantasy-pp-cli

## 1. Command Outputs and Scores
- **Umbrella invocation:** `cli-printing-press shipcheck --dir "$CLI_WORK_DIR" --spec "$RESEARCH_DIR/yahoo-fantasy-spec.yaml" --research-dir "$API_RUN_DIR"`
- **Umbrella Verdict:** `PASS` (7/7 legs passed, exit 0)
  - `verify`: `PASS` — 100% pass rate (64/64 passed, 0 critical)
  - `validate-narrative`: `PASS` — 10/10 narrative commands resolved and full examples passed
  - `dogfood`: `PASS` — 0 dead flags, 10/10 commands have examples, 7/7 novel features survived (`novel_features_check: {"planned": 7, "found": 7}`)
  - `workflow-verify`: `PASS` (`workflow-pass`)
  - `apify-audit`: `PASS`
  - `verify-skill`: `PASS` — all mechanical and canonical-section checks passed
  - `scorecard`: **92/100 — Grade A** (`Sample Output Probe: 5/5 passed (100% pass rate)`)

## 2. Top Blockers Found & Fixes Applied
1. **`ExtractMatchups` and `ExtractDraftPicks` duplicate extraction on top-level JSON arrays:**
   - **Root cause:** `walkJSON` merges slice elements to handle Yahoo's tuple-array convention and then recurses into each element. `ExtractPlayers`, `ExtractTeams`, and `ExtractTransactions` deduplicated by key, whereas `ExtractMatchups` and `ExtractDraftPicks` lacked a `seen` map.
   - **Fix:** Added key-based deduplication (`week:team_key:opponent_key` and `round:pick:player_key`) to `ExtractMatchups` and `ExtractDraftPicks` in `internal/yahoofantasy/yahoo.go`.
2. **Narrative command positional argument alignment:**
   - **Root cause:** `leagues get-scoreboard` takes `<league_key> <week>` as positional arguments rather than `--week`.
   - **Fix:** Updated `research.json` narrative examples and verified with `validate-narrative --strict --full-examples`.

## 3. Before / After Metrics
- **Verify Pass Rate:** 100% (64/64) -> 100% (64/64)
- **Scorecard Total:** 92/100 (Grade A) -> 92/100 (Grade A)

## 4. Final Ship Recommendation
- **Verdict:** `ship`
