# Drive Plaza publish-time live acceptance

Fresh full live dogfood after the route-panel review fix passed at 2026-10-02T12:59:27.303000+00:00.

- Run: `20261002-003344-620149db`; Printing Press 4.32.5.
- Mandatory matrix: 113 passing checks, zero failures; 91 explicit skipped/unverified cases.
- Current source fingerprint before publication module rewrite: `2ca66504c85c6391f7c5cbd5d8bc0df5644776ae430e7d6b83af3c593679c06a`.
- Gate: `cli-printing-press dogfood --dir <CLI_DIR> --live --level full --timeout 120s --research-dir <RESEARCH_DIR> --write-acceptance <PROOFS_DIR>/phase5-acceptance.json --json`.
- Embedded and archived source-bound acceptance synchronized after the parser and regression tests changed.
- The raw transcript was private and deleted after the gate.
- Full Go tests and go vet passed; promoted CLI/MCP binaries were rebuilt.

The initial review identified that nested ETC/ETC2 toll tabs shifted route-detail association. The fix selects only immediate route-level panels. Both a minimal nested-tab fixture and the real three-alternative page failed under the old selector and pass after the change. The separate live route-detail proof verifies each current alternative's stops, forecasts and warnings. A durable patch record preserves this source contract on regeneration.

The 91 skips are unverified, not successful checks. Historical completed-build shipcheck evidence is retained separately. No provider account mutations were exercised.
