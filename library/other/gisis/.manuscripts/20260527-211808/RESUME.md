# Resume marker — gisis-pp-cli v1 implementation

**Updated 2026-05-28 ~22:30. Implementation COMPLETE (Run 2).**

> Companion log with full detail: `Machine/Claude/Meta/2026-05-27-Vessel-MCP-Execution-Log.md` (Phase 1a, Run 2).

## What's done

- [x] Phase 0-1.9 (research, brief, manifest, sniff-gate, reachability) — Run 1
- [x] Phase 2 generate; scaffolding builds clean — Run 1
- [x] HTML parser (`ship_get_parser.go`, 13/13 unit tests on real fixture) — Run 1.5
- [x] `ship get <imo>` — typed JSON, caches on every fetch — Run 1.5 + Run 2
- [x] `auth ping` — session liveness, exit 0/4/5 — Run 1.5
- [x] **All 6 remaining novel features (Run 2):**
  - [x] `ship history <imo>` — flag/name/type/owner history from one fetch
  - [x] `ship list [--flag --owner --type --name-like --pinned --limit]` — cache query
  - [x] `ship stale [--older-than 30d --pinned]` — age query
  - [x] `owner fleet <owner> [--like]` — group cache by registered owner
  - [x] `ship batch [--imos --file --throttle]` — throttled multi-IMO resolve → cache
  - [x] `ship pin <imo> [--label] / unpin / refresh [--pinned --older-than]` — watchlist trio
- [x] **SQLite cache:** `internal/store/ship_cache.go` (`UpsertShipByIMO` keyed by IMO, not name) + `ship_pins` table + indexes in `extras.go`. 7 store tests pass.
- [x] `go build` / `go vet` / full `go test` / `gofmt` all clean.
- [x] Live path validated end-to-end against GISIS (login-wall detection + typed auth-failure; session was expired so no successful-fetch yet).

## What's LEFT (not implementation — workflow/ops)

1. **Phase 5.5 polish** — run `/printing-press-polish` on this CLI. shipcheck is **Grade B 75/100**; the 3 failing legs are all doc/manifest/quality, NOT code:
   - `tools-manifest.json` is stale (lists `.ASPXAUTH` + `tools:[]`) → regenerate.
   - README.md / SKILL.md examples + narrative need reconciling with the real command surface (`ship pin <imo>`, `ship history <imo>`, `owner fleet <owner>`, `ship batch --file`, `ship refresh --pinned`).
   - MCP token-efficiency / tool-design / insight scores want a polish pass.
   - `verify` leg fails only because `sync` is intentionally unimplemented (HTML page-mode scraper, no list endpoint) — this is expected, not a polish target.
2. ~~One successful live fetch~~ **DONE (2026-05-29).** `ship get 9866641 --json` returns real authenticated particulars via the Surf transport (bypassed the TLS fingerprinting that blocked curl). **Re-auth is now one command** (kooky wired in via `/printing-press-amend`): log into GISIS in Brave, then `GISIS_KOOKY_BROWSER=brave gisis-pp-cli auth login --chrome`. Also fixed this session: a `registered_owner_history` parser bug (was emitting company-detail field labels).
3. **Phase 5.6 promote** — move to `~/printing-press/library/`, register the MCP, consider `/printing-press-publish`. (Promote/publish = explicit user decision.)

## How to resume in a fresh session

1. Read this file + the execution log Run 2 entry.
2. `source $HOME/printing-press/.runstate/openclaw-brain-c646f0ca/.last-run-env`
3. `cli-printing-press lock acquire --cli gisis-pp-cli --scope openclaw-brain-c646f0ca`
4. Run `/printing-press-polish` (Phase 5.5), review the before/after delta.
5. For a live fetch: re-auth in Brave, populate the cookie jar, `ship get 9866641 --json`.
6. Phase 5.6 promote when ready.

## Key facts to carry forward

- Cache is keyed by **IMO** (vessels rename) — the generated `UpsertShip` keys by name, so `UpsertShipByIMO` in `ship_cache.go` is the one to use.
- GISIS embeds full history inline → `ship history` needs no snapshot-over-time architecture.
- Hand-authored files (all `*_impl.go`, `ship_cache.go`, `ship_unified.go`, `ship_get_handler.go`) survive regen; the only generated-file edit is the 1-line `root.go` owner swap (mirrors the Run-1 ship swap).
- MCI / Companies modules still deferred to v0.2 via `/printing-press-amend`.
