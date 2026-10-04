# tabiwa round2 focused recheck handoff
All consolidated R1–R5 fixes are implemented; source is frozen for the same reviewer. Ledger remains sequence31 failed in phase17; reviewer may resume phase17 after its focused audit. No final approval/publication is claimed.

Source: <source-project>
Run: <press-workspace>/.runstate/tabiwa-cli-dcb60d1f/runs/20261004-191703-ed1b3f92
Hashes: proofs/round2-source-hashes.json (fingerprint ca8e23db2f8f02c2d985c30c640437194d92f1c9aa42f1ad31a2cc1269b8ac43)
CLI/MCP: build/stage/bin/tabiwa-pp-cli and tabiwa-pp-mcp
Native bundle: build/tabiwa-pp-mcp-darwin-arm64.mcpb

R1: saved.go compares parsed RFC3339 instants inside one ordinary transaction before updating a conflicting row. Equal/older observations keep existing facts. Incoming/stored invalid clocks error; original source clock strings are preserved. Saved ordering and bounded50-record retention also compare parsed instants, including nanosecond/fraction/offset variants. Focused behavioral regressions TestSavedNewestObservationWinsAcrossClockForms and TestSavedOrderingAndRetentionUseInstants pass. The reviewer's original overlay regression passes without modifying that overlay. No schema migration or filesystem redesign; read-only saved reads still do not create/migrate or fetch source data.
R2: runtime context names regional preferences/provider IDs, default10/max50 output and default500/max1000 scan caps, date-membership/stock/full-term unknowns, units/nulls and separate selected catalog/saved.db versus generic data.db SQL/search. No cursor/after/default100 guidance remains.
R3: tools-manifest.json now snapshots all27 actual tools/list schemas, with5 normalized domain response contracts (including price units/nulls and comparison ids comma-separated string). .printing-press.json distinguishes27 total/public,5 domain,22 framework counts. Round2-runtime-metadata.json retains actual schema/context proof.
R4: README/SKILL use config.json and actual local stores; unsupported credential/TOML/secret-migration guidance, blank config path and nonexistent root list reference removed. Native bundle docs say darwin-arm64 only. Feature descriptions remain aligned with unchanged research.json.
R5: source and bundled manifest declare only darwin. Extracted final peers match stage hashes and build-info shows sqlite1.60.1/libc1.77.1,Go1.27.1,darwin/arm64. Proofs/round2-bundle-audit.json records extraction checks.

Validation: focused cache/CLI/MCP tests, original reviewer overlay, full Go tests/vet/build, all7 stable-source shipcheck legs PASS and all5 actual live domain MCP calls PASS on rebuilt peers. Proofs: round2-shipcheck.json, round2-verify-skill.json, round2-runtime-metadata.json, round2-bundle-audit.json, mcp-live-samples.json; pipeline/round2-tests.log. Printing Press patch record includes chronology, metadata and MCP callback preservation. No changes to internal/cliutil or internal/mcp/cobratree.

Pending after approval: phase18 full live acceptance, promotion/archive/install and authorized personal-account publication with fresh publish-time live validation and current-head readiness. No PR exists yet.
