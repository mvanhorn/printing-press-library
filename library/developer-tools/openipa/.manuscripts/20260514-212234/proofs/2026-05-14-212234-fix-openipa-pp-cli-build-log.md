# Build Log — openipa-pp-cli

## What Was Built

### Phase 3 critical fix: HTTP form encoding
- Modified `internal/client/client.go` to use `application/x-www-form-urlencoded` for POST requests
- Added `buildFormBody()` helper that serializes body + AUTH_ID as form-encoded
- AUTH_ID moved from query string to form body (required by IPA /public-ws/ API)
- Shared extraction helpers in `internal/cli/ipa_helpers.go` for three-level IPA envelope

### Priority 0/1 (absorbed features — 17 endpoints)
All 17 endpoints from the spec work correctly with form encoding:
- WS01_SFE_CF (fatturazione cf) ✓
- WS02_AOO (aoo list) ✓
- WS03_OU (uo list) ✓
- WS04_SFE (fatturazione ente) ✓
- WS05_AMM (enti get) ✓
- WS06_OU_CODUNI (uo get) ✓
- WS07_EMAIL (cerca email) ✓
- WS08_AOOC (aoo get) ✓
- WS09_DOM_DIG_AOO (domicilio aoo) ✓ — fixed: uses COD_AMM not COD_AOO
- WS10_DOM_DIG_OU (domicilio uo) ✓
- WS11_DOM_DIG_STOR_AOO (domicilio storico-aoo) ✓ — fixed: uses COD_AMM
- WS12_DOM_DIG_STOR_OU (domicilio storico-uo) ✓
- WS13_DOM_DIG (domicilio email) ✓
- WS14_NSO_CF (nso cf) ✓
- WS15_NSO (nso ente) ✓
- WS16_DES_AMM (enti cerca) ✓
- WS18_AOO (aoo cerca) ✓ — changed to COD_UNI_AOO param (not text search)

### Priority 2 (transcendence features)
- `cf.go` — Parallel WS01+WS14+WS23 compliance check ✓
- `enti_tree.go` — Parallel WS05+WS02+WS03 hierarchical view ✓
- `fatturazione_batch.go` — Stdin batch CF lookup ✓
- `domicilio_verifica.go` — Sequential WS13+WS07 PEC status ✓
- `aoo_cerca.go` — WS18 AOO by unique code ✓
- `stats.go` — Stub (requires sync) ✓
- `report.go` — Stub sfe-mancante (requires sync) ✓

## Known Issues / Stubs

1. **WS23_DOM_DIG_CF** — HTTP 500 server-side bug on IPA
   - Affects: `cf <CF>` command (dom_status = "error")
   - Workaround: None (server-side issue)

2. **WS20_PEC / WS21_PEC_ENTE_STOR / WS22_PEC_STOR** — HTTP 500 server-side bug
   - Affected commands: not yet implemented (stub planned)

3. **stats** / **report sfe-mancante** — require local SQLite sync
   - Currently show informative stub message
   - Full implementation requires `openipa sync` + SQLite store layer

4. **aoo cerca** — NOT text search (WS18 requires COD_UNI_AOO)
   - Research.json updated to reflect actual behavior
   - Text search by description not available via WS API

## Generator Limitations Found

- `govulncheck` gate fails due to Go toolchain version mismatch (Go 1.24 vs 1.26 requirement)
  → Not a code issue; the code compiles and runs correctly with Go 1.26 toolchain
- POST body encoding was JSON by default; required Phase 3 patch for form-encoding
