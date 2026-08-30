# fish-audio-pp-cli shipcheck

## Runs
1. First shipcheck: verify PASS, validate-narrative FAIL (troubleshoot example `tts resolve --model <id>` stripped to a bare `--model`), dogfood PASS, workflow-verify PASS, verify-skill PASS, scorecard 94/100 Grade A, HOLD on live_api_verification (no API key yet).
2. Fix: replaced the example with a real voice id + model in research.json, README, SKILL.
3. Second shipcheck: all 7 legs PASS; scorecard HOLD only on `live_api_verification` (unverified, needs FISH_AUDIO_API_KEY).

## Generator issues found (retro candidates)
- `tts_create-stream.go` emitted an unused `statusCode` variable (compile failure at generate time).
- Group names from research.json `novel_features[].group` leaked into parent-command `Short`.
- `registerNovelCommand` hooks run before root attaches novel parents (`render`, `voice`); hook had to construct the groups itself.
- `POST /model` (multipart in spec) generated as JSON; multipart encoder cannot repeat field names (`voices[]`).
- `Fishaudio` display string in templated `auth.go` despite `display_name: "Fish Audio"`.
- dogfood rewrites research.json with `<` escapes; sed-based edits then miss.

## Scores
- verify: PASS, 0 critical. scorecard: 94/100 (A). Weak dims: MCP Desc Quality 5, Cache Freshness 5, MCP Token Efficiency 7, Insight 7.
- Sample probe: 4/6 (2 failures are 401s from the missing key).

## Verdict
`hold` pending Phase 5 live dogfood with the API key; structurally ship-ready.
