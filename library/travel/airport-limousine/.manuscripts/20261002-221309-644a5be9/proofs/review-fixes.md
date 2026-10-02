# Publication review corrections

- Baggage: the current Japanese guide explicitly limits the Shibuya–Narita LCB route to one checked piece per person. The fresh official catalog distinguishes `Narita-ShibuyaLCB` from the ordinary `Narita-Shibuya` route. `conditions` now reads both language guides, returns that route exception first, and marks the English two-piece allowance as a general rule. The returned exception retains its Japanese source URL, source update date, exact route ID and unit even with `--agent --limit 1`. The unknown fallback does not infer a count after source wording changes.
- Documentation: the first-use discovery instruction now invokes the actual `routes` and `stops find --query Shinjuku` commands.
- Public manuscripts: workstation paths have been replaced with `<home>`, `<run-dir>`, `<cli-dir>` or `<press-home>`. The executable Windows path test remains a generic example. Private raw live-gate transcripts and compiled artifacts are excluded.

Source URLs: https://www.limousinebus.co.jp/ja/guide/terms/baggage/ and https://www.limousinebus.co.jp/en/guide/terms/baggage/ . The current Japanese guide content update is `2026-03-25T00:33:16.691Z`; review observation is `2026-10-02T17:56:02Z`. Source route catalog cross-check is the current official timetable route data.

Verification: focused parser/domain/CLI/MCP tests passed, including scoped count1 and unknown fallback; live `conditions --topic baggage --agent --limit 1` returned count1 for `Narita-ShibuyaLCB` with explicit scope, Japanese provenance and three requests. The fresh full publication live gate passed for the edited source tree; see `phase5-acceptance.json` for actual executed/skipped counts and source fingerprint. The complete Go tests and go vet also passed.

The public redirect-review overlay now uses portable roots and includes `reviewer/replay_redirect_review.py`, which derives roots from its own location, materializes a private temporary Go overlay, reruns both redirect-budget and source-query regression checks, and deletes the temporary overlay. No workstation path is committed.
