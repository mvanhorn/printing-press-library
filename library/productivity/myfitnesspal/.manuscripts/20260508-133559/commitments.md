# Build Commitments — 2026-05-08

User-approved scope additions and pause-points captured at the Phase 1.5 gate. Reference these during Phase 3 + 5.6.

## Phase 3 commitments

### C-1. Harden SQLite cache permissions (printed-CLI fix)
The generator's default is `0o755` directory + umask-default file (`0o644` on macOS). For this CLI the data is personal nutrition + weight + diary content, more sensitive than the generic case.

Action in Phase 3:
- Add a small helper in `internal/store/store.go` (or extend `OpenWithContext`) that, after `MkdirAll`, calls `os.Chmod(dir, 0o700)` on the database directory.
- After the SQLite file is created on first open, call `os.Chmod(dbPath, 0o600)` to lock the file mode.
- Idempotent — chmod every open is safe and recovers from any user `chmod` drift.
- Test: a unit test asserts the dir+file modes are `0o700` / `0o600` after `OpenWithContext` returns.

Retro candidate: file an issue with `printing-press-retro` after the run suggesting the generator default move to `0o700` / `0o600` for `auth.type: cookie | composed | session_handshake` CLIs. The current 0o755/0o644 default is wrong for any CLI that handles personal data behind a session.

### C-2. Synthetic fixtures only — no real account data in testdata/
Phase 3 test fixtures (HTML scraper goldens, analytics inputs) MUST use synthetic content:
- Synthetic foods: "Banana, Raw" / "Greek Yogurt, Plain" / "Chicken Breast, Cooked" with round numbers (100 cal, 25g protein, etc).
- Synthetic users: never `example-user` or any real username.
- Synthetic diary dates: dates like `2024-01-15` (not the user's real recent dates).
- Synthetic numeric IDs: `12345`, `67890` — not the real user-ID `28464559025213` from the HAR.
- Synthetic weights: 175.0, 174.5, 174.0 — not the user's real weights.

The Phase 4.85 output review will scan committed fixtures for incidental leaks. Any reference to the real username, the real numeric user ID, or visible food names from the user's HAR is a fail.

## Phase 5.6 commitments

### C-3. Delete the original HAR from `~/Downloads` after archive
Sequence:
1. Archive `discovery/browser-sniff-capture.har` to `$PRESS_MANUSCRIPTS/myfitnesspal/<run-id>/discovery/` with response bodies stripped.
2. Verify the archive copy exists and is non-empty.
3. **Delete `~/Downloads/www.myfitnesspal.com.har`** (the user's original DevTools export).
4. Print: "Removed original HAR from ~/Downloads — archive lives at <path>."

Reason: `~/Downloads` is the most-likely-to-be-accidentally-shared folder on this machine. The HAR contains the user's MFP user ID and username; it shouldn't outlive this run.

## Pause points

### P-1. Diary HTML parser fragility
The HTML scraper for `/food/diary` (the most fragile component in the whole CLI) gets built early in Phase 3. If any of the following happen, **pause and surface to the user before continuing:**
- Parser takes unusually long to write (multiple structural rewrites).
- Test goldens reveal a structure that diverges materially from what `python-myfitnesspal` documents in its source.
- A captured page in the HAR shows MFP markup that doesn't match the `python-myfitnesspal` extraction selectors.

The user explicitly flagged this as the boomerang point that determines whether this CLI nails it or ships a parser that breaks on day one. Worth a 60-second pause to get a second look.
