# WalkingPad CLI — Phase 5 Live Acceptance (real WalkingPad C2)

Tested with the `-tags wp_live` binary against a physical WalkingPad C2
(`KS-BLC2`, CoreBluetooth UUID `07ef94a9-…-136a`). The binary-owned live-dogfood
runner cannot enumerate a device CLI (no `agent-context` command — machine gap),
so this matrix was executed manually with the live binary.

## Matrix (all PASS)
| Check | Result | Evidence |
|-------|--------|----------|
| `doctor --live` | PASS | `reachable: true, found: 1` |
| `scan --live` | PASS | found `KS-BLC2`, service `fe00`, rssi −82 |
| `status --live` | PASS | frame parsed correctly; tracked physical state change standby(belt_state 4)→manual(belt_state 0) |
| `monitor --live --duration` | PASS | clean bounded stream, exit 0 |
| write path | PASS | fixed: `fe02` is write-without-response on the C2 (write-with-response → "Writing is not permitted") |
| `run --speed 2.0` | **PASS — belt physically moved** | held connection ramped belt 0→**2.1 km/h**→stop; session recorded (12 samples) |
| clean stop | PASS | belt returned to mode=manual, speed 0 after each run |
| `record` | PASS | session persisted to local store |
| `today` / `sessions` | PASS | reflected 3 recorded sessions, 15s total |
| `streak` | PASS | computed from real history |
| `profile set` + `calories` | PASS | computed (0 kcal — belt ran empty, no actual walking; correct) |
| safety gating | PASS | under verify → no-op; without `--live` → notice; without `--confirm-physical-effect` → refusal |
| `speed 0.3` | PASS | rejected (out of range) after the code-review fix |
| `$WALKINGPAD_ADDRESS` | PASS | actuation sequence targeted the pad via the env var (code-review fix) |

## Key live findings (fed back into the CLI)
1. **C2 advertises as `KS-BLC2`, not "WalkingPad"** → discovery matches the `fe00` service UUID (works family-wide). Caught during discovery.
2. **`fe02` is write-without-response** → driver now prefers write-without-response, falls back to write-with-response.
3. **The belt only runs over a held, polled BLE connection** (like the official app / ph4 shell). One-shot `start` does not sustain it → added the `run` command (held connection + keep-alive poll), which engaged the belt successfully. `run` is now the documented primary control command.
4. **Software start requires a physically-awake pad** (display on); deep-standby start is blocked for safety — correct device behavior, documented in troubleshooting.

## Acceptance
- **Gate: PASS.** Every applicable command validated against real hardware, including physical belt actuation and the full record→analytics loop.
- Distance/steps read 0 because the belt was run empty (no one walking) for short test intervals; speed telemetry (2.1 km/h) and session recording confirm the data path end-to-end.
- Only test sessions (empty walks) were written to the local history store; no pre-existing user data was touched.
