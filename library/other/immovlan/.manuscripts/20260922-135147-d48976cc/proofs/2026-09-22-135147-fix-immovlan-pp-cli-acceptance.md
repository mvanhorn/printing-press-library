# Phase 5 — Live dogfood acceptance — immovlan-pp-cli (run 20260922-135147-d48976cc)

Level: **full** (user choice). Target: immovlan.be public pages, no credentials. Runner: `cli-printing-press dogfood --live --level full` with `--research-dir`.

| Pass | Matrix | Passed | Failed | Verdict |
|---|---|---|---|---|
| 1 | 137 | 127 | 10 | FAIL — Examples sections missing on saved/shortlist subcommands (6), dry-run JSON envelope missing on generated doctor / feedback list / profile list (3), `dump --format csv --json` printed CSV (1) |
| 2 | 152 | 151 | 1 | FAIL — doctor guard inserted on the wrong RunE signature |
| 3 | 152 | 152 | 0 | **PASS** |

Fixes between passes (all in-session): Example fields on six hand-written subcommands; `--json`/`--agent` now take precedence over `--format` in `dump` and its happy-args no longer include a shell redirect; generated `doctor`, `feedback list`, `profile list` honour `--dry-run --json` (recorded as patch `.printing-press-patches/dry-run-json-envelopes.json`, machine gap filed for retro).

Acceptance marker: `/Users/Szpil/printing-press/.runstate/workspace-8a5edab2/runs/20260922-135147-d48976cc/proofs/phase5-acceptance.json` (status pass, 152/152).
