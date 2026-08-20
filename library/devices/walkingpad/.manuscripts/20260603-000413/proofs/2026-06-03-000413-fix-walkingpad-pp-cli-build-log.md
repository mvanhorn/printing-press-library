# WalkingPad CLI — Build Log

## Architecture
- Device generator emits a pure-Go replay scaffold (`capabilities`, `status`, `start`, `stop`, `wake`) + `Transport`/`Session` interfaces + safety harness.
- Live BLE behind `//go:build wp_live` (tinygo bluetooth v0.15.0, CGO). Default build is pure-Go stub → `verify`/CI/`go install` stay green and CGO-free; `./scripts/build-live.sh` (or `-tags wp_live`) builds the real controller.

## Built (P0–P2)
- **wpble** (pure Go): frame codec (CRC, command builders), status/last-record parse, speed/mode/pref encoding. `client.go` (backend-agnostic ops: Scan/Status/Monitor/LastRecord/Send/SendControl). `backend_live.go` (wp_live, tinygo) + `backend_stub.go` (default).
- **device/live.go**: `LiveTransport` implements the generated `Transport` so generated `status`/`start`/`stop`/`wake` go live under `--live`.
- **history** (pure Go JSONL): sessions+samples store; TotalsOn, SessionsOn, DailySeries (zero-filled), Streak.
- **profile** (pure Go): body weight + MET calorie model.
- **cli**: hand-built `scan`, `monitor`, `last-record`, `speed`, `mode`, `prefs` (×6), `record`, `today`, `sessions`, `trends`, `streak`, `calories`, `export`, `doctor`, `profile set/show`. Registered via `registerWalkingPadCommands` hook in root.go.
- root.go minimally edited: `--live`/`--address` flags, runtime transport selection, registration hook.

## Safety wiring (verified)
- Physical-effect/config writes require `--live` + `--confirm-physical-effect`; `--dry-run` previews exact payload.
- `verifyNoop` + replay default → no actuation under `PRINTING_PRESS_VERIFY=1`.
- `boundedContext` curtails long-running commands under `PRINTING_PRESS_DOGFOOD=1`.
- MCP read-only annotations on all non-actuating commands.

## Gates passed
- `go build ./...` (default, pure Go): OK
- `go build -tags wp_live ./...` (CGO): OK
- `go vet ./...`: OK
- `go test ./...`: OK (wpble codec incl. CRC/parse vs documented frame; history rollups/streak; profile MET/calories)
- `gofmt -l`: clean
- Phase 3 completion gate: all 26 approved command paths resolve as real Cobra commands.

## Deferred (per manifest, not stubbed)
- `prefs display` (undocumented 7-bit mask), `prefs target` value semantics, `ask_profile` payloads (unknown). Left out of v1 rather than shipped as guesses.

## Pending live validation (Phase 5)
- Status-frame byte layout taken from documented A1 frame; confirm against C2 and adjust offsets if different.
- Real actuation (start/stop/speed) with operator confirmation.
