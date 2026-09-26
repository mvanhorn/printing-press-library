# thunderbird-pp-cli shipcheck

## Loop 1
- verify PASS, dogfood PASS (WARN: 9 dead generated helpers), workflow-verify PASS (no manifest), apify-audit PASS, verify-skill PASS, scorecard 81/100 Grade A, live sample probe 5/5.
- validate-narrative FAIL: recipe `drafts reply 1234 ...` — numeric positional parsed as a subcommand word.
- Fix: quoted positional id in research.json, README.md, SKILL.md.

## Loop 2
- All 7 legs PASS (shipcheck exit 0). Scorecard 81/100 Grade A. Sample output probe 5/5 novel features.
- dogfood rewrote internal/cli/which.go without gofmt (known papercut) — re-formatted.

## Remaining non-blocking gaps (for polish)
- dead_code 0/5: 9 unused generated HTTP helpers (handleBinaryResponseDelivery, hasChangedLocalFlags, parseSyncKVFlags, parseSyncUserParams, readSecretFromStdin, retainCLIQueryParams, retainExplicitQueryParams, successfulNoop, truncateJSONArray).
- mcp_description_quality 0/10: static tools-manifest.json still lists the 6 spec endpoint tools with HTTP paths; runtime MCP surface is the Cobra-tree mirror (search, sql, context + all commands).
- sync_correctness 0/10 and cache_freshness 5/10 score the generic HTTP sync template; local profile sync is custom.

## Behavioral check
Every novel command sampled against the real store (slice C): exit 0, non-empty plausible counts (awaiting-reply 374 threads/90d, filters audit 1 filter evaluated, largest ordered, newsletters 4 groups).

## Verdict: ship
