# Drive Plaza publish-time live acceptance

Fresh full live dogfood passed at 2026-10-02T12:36:10.872589+00:00.

- Run: `20261002-003344-620149db`; Printing Press 4.32.5.
- Mandatory matrix: 113 passing checks, zero failures; 91 explicit skipped/unverified cases.
- Source fingerprint before publication module rewrite: `cf501c00b5ebd46f4a4561e17b768a338f1d9ccb8415e7b0e9ce40d6e75f3e53`.
- Gate: `cli-printing-press dogfood --dir <CLI_DIR> --live --level full --timeout 120s --research-dir <RESEARCH_DIR> --write-acceptance <PROOFS_DIR>/phase5-acceptance.json --json`.
- Fresh source-bound acceptance is synchronized in embedded and archived proofs.
- The raw live transcript was private and deleted after the gate. It is not a publication artifact.
- One preliminary run disabled local learning and consequently suppressed JSON for three local dry-run helpers; normal configuration passed without source changes.

The 91 skips are unverified, not successful checks. The independent final build matrix, MCP evidence, and seven-leg shipcheck remain separate proofs. No provider account mutations were exercised.
