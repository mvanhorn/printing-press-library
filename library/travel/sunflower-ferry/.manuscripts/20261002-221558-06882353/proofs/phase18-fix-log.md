# Full dogfood fix

First actual full matrix: 116/117 tests passed; the only failure was missing Examples in routes list help. Added the concrete routes list --agent Example to that owned constructor. No runtime/provider behavior changed. Rebuild CLI/MCP and bundle, then rerun the actual full matrix and write a fresh source-bound acceptance marker. Existing phase5-acceptance.json is the runner-written failure and is not hand-edited.
