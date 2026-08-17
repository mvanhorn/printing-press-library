Live Dogfood Report: parallel-pp-cli
================================

Level:      quick
Verdict:    PASS (with skips)
Commands:   6
Tests:      16 passed, 0 failed, 9 skipped

[PASS] analytics help
[PASS] analytics happy_path
[PASS] analytics json_fidelity
[SKIP] analytics error_path: no positional argument
[PASS] api help
[SKIP] api happy_path: command path [api] has fewer segments than placeholders (1)
[SKIP] api json_fidelity: command path [api] has fewer segments than placeholders (1)
[PASS] api error_path
[PASS] balance burn help
[PASS] balance burn happy_path
[PASS] balance burn json_fidelity
[SKIP] balance burn error_path: no positional argument
[PASS] chat help
[PASS] chat happy_path
[PASS] chat json_fidelity
[SKIP] chat error_path: no positional argument
[SKIP] chat error_path_real: mutating command dry-run only
[PASS] doctor help
[PASS] doctor happy_path
[PASS] doctor json_fidelity
[SKIP] doctor error_path: no positional argument
[PASS] export help
[SKIP] export happy_path: command path [export] has fewer segments than placeholders (2)
[SKIP] export json_fidelity: command path [export] has fewer segments than placeholders (2)
[PASS] export error_path
