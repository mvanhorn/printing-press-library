# serply-pp-cli Phase 5 live acceptance

Level: full. Auth: api_key (SERPLY_API_KEY from the environment; the value is not recorded here).

## Tests

- Matrix: 114 tests over the command tree, run by `cli-printing-press dogfood --live --level full`.
- Run 1: 109 passed, 5 failed.
- Run 2 (after fixes): 114 passed, 0 failed. Verdict PASS.
- Run 3 (after the polish G104 fix, to rebind the source fingerprint): 114 passed, 0 failed. Verdict PASS. Skipped rows are the runner's own safety skips: mutating commands without --allow-destructive, non-id positionals and commands with no positional argument.

## Failures (run 1)

1. `job-search` happy_path and json_fidelity: `unknown flag: --num`. The Example line added in polish passed `--num 5`, but the generated job-search command has no num flag.
2. `products` happy_path and json_fidelity: the same `--num` Example error.
3. `research` error_path: `research __printing_press_invalid__` exited 0 with web results. Any free-text topic is a valid search, so exit 0 is correct behaviour; the positional was named `[topic]`, so the runner treated it as an ID-style argument.

## Fixes

1. Dropped `--num 5` from the job-search and products Example lines (covered by the promoted-verticals-carry-runnable-examples patch record).
2. Renamed the research positional to `[query]` (and its pp:happy-args key), so the runner classifies it as a search-shaped command, which is what it is. No local "empty means invalid" heuristic was added.

## Gate

PASS. phase5-acceptance.json: status pass, level full, matrix 114, 114 passed, 0 failed.
