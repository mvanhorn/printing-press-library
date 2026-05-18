---
title: "feat: decrypt Granola encrypted local cache in granola-pp-cli"
type: feat
status: active
date: 2026-05-12
target_repo: github.com/mvanhorn/printing-press-library (working from feat/granola branch on DamienStevens fork)
---

# feat: decrypt Granola encrypted local cache in granola-pp-cli

## Summary

Patch granola-pp-cli so it reads Granola desktop's new encrypted local cache (`cache-v6.json.enc`) and encrypted WorkOS token store (`supabase.json.enc`). Granola began encrypting these files sometime before May 12 2026 and kept the plaintext copies as stale stubs. Every install of granola-pp-cli currently returns zero rows on `sync` and empty arrays on live API calls because the loaders only read the plaintext paths.

The fix lives in one new package (`safestorage`) and two surgical patches (cache loader, supabase token loader). macOS only in this round. The CLI is not yet in the public Printing Press registry, so this plan also covers the publish path: PR to the original author (DamienStevens) for review, then registry merge upstream.

---

## Problem Frame

**Observable failure on every current Granola user:**
- `granola-pp-cli sync` reports `{"meetings":0, "transcript_segments":0, ...}`.
- `granola-pp-cli meetings list --data-source live` returns `[]`.
- `granola-pp-cli doctor` says `Auth: not configured` and `Cache: unknown / sync_state is empty`.

**Root cause (empirically established this session):**

| File | Path | Plaintext state | Encrypted state |
|---|---|---|---|
| Cache | `~/Library/Application Support/Granola/cache-v6.json` | Small JSON stub frozen at the day Granola began encrypting; `entities: {}` | `cache-v6.json.enc`, multi-MB binary, mtime tracks recent desktop activity |
| Auth | `~/Library/Application Support/Granola/supabase.json` | Small JSON, last-written `access_token` expired on the same freeze day | `supabase.json.enc`, mtime tracks recent desktop activity |

The CLI's loaders read the plaintext paths unconditionally. Once Granola began encrypting, the plaintext copies froze — they are no longer kept in sync with desktop state.

**Encryption envelope (empirical evidence):**
- No Chromium `v10`/`v11` magic prefix on the cache and supabase files (verified via `xxd`).
- File-size arithmetic on `supabase.json.enc` vs the known plaintext shape matches `nonce(12) ‖ ciphertext ‖ tag(16)` AES-GCM exactly.
- `cache-v6.json.enc` has a length consistent with the same envelope.
- Keychain entry confirmed: `security find-generic-password -s "Granola Safe Storage" -w` returns a 16-byte raw key (value redacted from this document — it is a live credential; fetch fresh from your local Keychain at implementation time).
- Initial hypothesis: AES-128-GCM with raw Keychain bytes as the key. (Refuted at U1 — see safestorage/testdata/scheme.md for the actual two-tier scheme.)

**Scope of impact:**
- All ~35 CLI commands depend on the local cache or the WorkOS token. Both are blocked.
- The CLI is not in `registry.json` upstream, so the user base today is limited to people who built locally from `DamienStevens/printing-press-library:feat/granola`. Once we land both the decryption patch and the registry entry, the CLI becomes installable by anyone via `npx -y @mvanhorn/printing-press install granola --cli-only`.

---

## Goals

1. After install, `granola-pp-cli sync` reads `cache-v6.json.enc` on a Granola-signed-in macOS user without further configuration. First invocation triggers a macOS Keychain prompt; "Always Allow" makes subsequent runs silent.
2. `loadFromSupabaseJSON` discovers the encrypted token store automatically, refresh-token rotation continues to work, and `auth status` reports `authenticated: true` from the discovered token.
3. `doctor` distinguishes four states: Granola not installed; Granola installed but not signed in; encrypted store present but decrypt failed (scheme drift); decrypt succeeded.
4. Existing plaintext fixtures and the `GRANOLA_CACHE_PATH` / `GRANOLA_SUPPORT_DIR` overrides continue to work for tests and for users running older Granola versions.
5. The decryption layer is isolated in one Go package so future scheme drift is a single-package change.
6. The CLI ships through the standard Printing Press install path with no additional setup steps documented beyond "sign in to Granola desktop, then accept the Keychain prompt."

## Non-Goals

- Linux (`libsecret`) and Windows (DPAPI) decryption — follow-up, not blocking macOS release.
- The official Granola OAuth MCP integration at `https://mcp.granola.ai/mcp` — separate plan (Phase 2 in the prior conversation).
- Modifying any of the 35+ existing commands beyond the load-path changes they all inherit.
- Reverse-engineering Granola's encryption *scheme rotation strategy* (we'll handle drift reactively).

---

## Architecture Sketch

The intent is to add one boundary — `safestorage` — that owns Keychain access and AES-GCM unwrap. The two existing loaders consult this package when their `.enc` sibling is present, otherwise fall back to existing plaintext code paths.

```text
┌─ existing loaders (workos.go, cache.go) ─────────────────┐
│                                                          │
│  loadFromSupabaseJSON():                                 │
│    if supabase.json.enc exists:                          │
│       blob = safestorage.Decrypt(read .enc)              │
│    else:                                                 │
│       blob = read plaintext (legacy / test path)         │
│    json.Unmarshal(blob, &supabaseFile)  (unchanged)      │
│                                                          │
│  loadCache(path):                                        │
│    same shape; defaults to .enc if present               │
│                                                          │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─ NEW: internal/granola/safestorage ──────────────────────┐
│                                                          │
│  Key() ([]byte, error)                                   │
│    macOS:   shell out to `security find-generic-password │
│             -s "Granola Safe Storage" -w` (read-only)    │
│    linux:   stub returns ErrUnsupportedPlatform          │
│    windows: stub returns ErrUnsupportedPlatform          │
│    cached in-memory after first success                  │
│                                                          │
│  Decrypt(ciphertext []byte) ([]byte, error)              │
│    AES-128-GCM, nonce = first 12 bytes,                  │
│    tag = last 16 bytes, key = Key()                      │
│    typed errors: ErrKeyUnavailable, ErrAuth(decrypt-fail)│
│                                                          │
└──────────────────────────────────────────────────────────┘
```

This sketch illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.

---

## Output Structure (Greenfield Subdirs)

```
library/productivity/granola/
├── internal/
│   └── granola/
│       └── safestorage/                      # NEW
│           ├── safestorage.go                # Key() and Decrypt(), platform dispatch
│           ├── safestorage_darwin.go         # macOS Keychain implementation
│           ├── safestorage_linux.go          # stub returning ErrUnsupportedPlatform
│           ├── safestorage_windows.go        # stub returning ErrUnsupportedPlatform
│           ├── safestorage_test.go           # AES-GCM round-trip + envelope tests
│           └── testdata/
│               ├── fixture-key.bin           # 16 raw bytes (test-only key)
│               ├── fixture-supabase.enc      # supabase.json fixture encrypted with fixture key
│               └── fixture-cache.enc         # small cache fixture encrypted with fixture key
```

---

## Implementation Units

### U1. Confirm encryption scheme empirically

**Goal:** Verify the AES-128-GCM-with-raw-Keychain-bytes hypothesis against the real encrypted files before writing any production code. If wrong, the rest of the plan needs revision. This is a 30-minute experiment, not production work.

**Requirements:** Goal 5 (decryption layer must rest on confirmed scheme, not speculation).

**Dependencies:** None.

**Files:** No production files modified. The experiment uses a throwaway script (Python with `cryptography` library is fastest; can also be a one-off Go file in `/tmp` outside the repo).

**Approach:**
1. Read the Keychain blob with `security find-generic-password -s "Granola Safe Storage" -w` and base64-decode to 16 raw bytes.
2. Attempt AES-128-GCM decryption of `supabase.json.enc`:
   - Nonce = first 12 bytes.
   - Tag = last 16 bytes.
   - Ciphertext = everything in between.
   - Expected plaintext: valid JSON with `workos_tokens`, `session_id`, `user_info` keys matching the shape of the May-4 stub.
3. If success, repeat against `cache-v6.json.enc` and confirm valid JSON with a `cache.state` top-level key.
4. If failure, walk a small set of fallback schemes in order:
   - AES-256-GCM with `SHA-256(key)`.
   - AES-128-GCM with PBKDF2-SHA256(key, salt=`"saltysalt"`, iter=1003, len=16) — Chromium-style key derivation despite the missing magic prefix.
   - AES-128-CBC with PKCS7 padding (in case my GCM byte-count math was coincidental).
   - As a last resort, inspect `/Applications/Granola.app/Contents/Resources/app.asar` (an `asar` archive of compiled Electron JS) for the encrypt routine using `npx @electron/asar extract` and reading the resulting JS. Look for calls like `crypto.createCipheriv` or `safeStorage`.

**Pattern to follow:** None — this is empirical research, not coded work.

**Test scenarios:** Test expectation: none — this is a reverse-engineering experiment that produces a finding (which scheme works), not code. The finding gates Units 2-8.

**Verification (tightened per doc-review):** The experiment succeeds only when ALL of the following hold:

1. Decryption produces a byte stream that `json.Unmarshal` parses into a `map[string]any` with **no trailing bytes** (a wrong-key partial-decrypt under non-GCM schemes can produce a substring match inside a longer garbage buffer; trailing-bytes rejection catches this).
2. The decrypted JSON contains **at least 3 distinct expected keys** per file — for `supabase.json.enc`: `workos_tokens`, `session_id`, and either `user_info` or the inner `access_token`; for `cache-v6.json.enc`: `cache` at the top level, plus `state` and `entities` inside it.
3. **Both `supabase.json.enc` AND `cache-v6.json.enc` decrypt successfully under the same scheme.** A coincidental match on one file does not survive the second. If only one parses, treat as a partial finding: U4 (supabase loader) can proceed against the working scheme; U3 (cache loader) needs additional investigation (chunked envelope? compressed-then-encrypted? per-entity key?). Do NOT proceed with U3 against an unverified scheme.

Capture the working scheme as a test vector (Keychain key bytes + ciphertext byte ranges + expected plaintext head) directly in `safestorage/testdata/scheme.md` and in a comment in `safestorage_darwin.go`. No separate `docs/plans/notes/` file is required — the test vector lives next to the code that consumes it. If no candidate scheme works within ~2 hours, stop and escalate — the plan needs revision.

---

### U2. New `safestorage` package with Keychain key fetcher and AES-GCM decrypt

**Goal:** Build the isolated decryption boundary. Single entry point with platform-specific key fetching; one decryption function that takes a ciphertext blob and returns plaintext. Typed errors so callers can distinguish "no Keychain entry" from "decrypt failed" from "unsupported platform."

**Requirements:** Goals 1, 3, 5.

**Dependencies:** U1 (must know which scheme to implement).

**Files:**
- `library/productivity/granola/internal/granola/safestorage/safestorage.go` (CREATE) — exported `Key()`, `Decrypt(ciphertext []byte) ([]byte, error)`, error sentinels.
- `library/productivity/granola/internal/granola/safestorage/safestorage_darwin.go` (CREATE) — macOS Keychain key fetch via `os/exec` invoking `security find-generic-password -s "Granola Safe Storage" -w`. Build tag `//go:build darwin`. Cache the key in a `sync.Once` after first success.
- `library/productivity/granola/internal/granola/safestorage/safestorage_linux.go` (CREATE) — returns `ErrUnsupportedPlatform`. Build tag `//go:build linux`.
- `library/productivity/granola/internal/granola/safestorage/safestorage_windows.go` (CREATE) — returns `ErrUnsupportedPlatform`. Build tag `//go:build windows`.
- `library/productivity/granola/internal/granola/safestorage/safestorage_test.go` (CREATE) — see test scenarios below.
- `library/productivity/granola/internal/granola/safestorage/testdata/fixture-key.bin` (CREATE) — 16-byte test-only key, committed.
- `library/productivity/granola/internal/granola/safestorage/testdata/fixture-supabase.enc` (CREATE) — pre-encrypted fixture mirroring the real Granola supabase shape.
- `library/productivity/granola/internal/granola/safestorage/testdata/fixture-cache.enc` (CREATE) — small encrypted fixture mirroring the real cache shape (a few hundred bytes of valid cache JSON is enough).

**Approach:**
- Errors as sentinels: `ErrUnsupportedPlatform`, `ErrKeyUnavailable` (Keychain prompt denied, Granola not installed), `ErrDecryptFailed` (wrong key, scheme drift, corruption). Callers branch on these via `errors.Is`.
- `Key()` shells out to `/usr/bin/security` via `exec.Command`. We use the binary rather than CGO into Security.framework because: (a) the rest of the CLI is pure Go and we don't want to introduce CGO build complexity for one shell-out; (b) the Keychain prompt UX comes from the `security` binary's own ACL flow, which surfaces "Always Allow" correctly. Stderr capture to distinguish "no entry" from "user denied".
- `Decrypt()` uses `crypto/aes` + `crypto/cipher.NewGCM`. Inputs validated: ciphertext must be at least `nonce_len + tag_len + 1` = 29 bytes. Nonce length pulled from `gcm.NonceSize()` to keep the implementation honest if we ever bump key size.
- Optional: a `GRANOLA_SAFESTORAGE_KEY_OVERRIDE` env var that lets tests and CI inject a key without touching the OS Keychain. Already a convention for `GRANOLA_CACHE_PATH` in this codebase.
- Helper `Available() bool` returns true if Keychain access succeeded once. Used by `doctor` and by the loaders to decide whether to surface a hint.

**Patterns to follow:**
- Build-tag-per-OS pattern: existing Go convention; nothing in this repo uses it yet, but `os/exec` + `runtime.GOOS` checks are already in `internal/granola/cache.go` for path resolution.
- Error sentinel style: `internal/granola/internalapi.go` already defines typed errors for the WorkOS auth path. Mirror that.
- Test fixture pattern: `internal/granola/testdata/` is the existing convention for cache test fixtures.

**Test scenarios:**
- `Decrypt` round-trips a known plaintext through the fixture key — encrypt with a sister test helper, then call `Decrypt`, assert plaintext equality.
- `Decrypt` on a too-short ciphertext (< 28 bytes) returns `ErrDecryptFailed` and does not panic.
- `Decrypt` on a corrupt-tag ciphertext (last byte flipped) returns `ErrDecryptFailed`.
- `Decrypt` on a corrupt-ciphertext (middle byte flipped) returns `ErrDecryptFailed` (GCM tag check should catch this).
- `Decrypt` on a wrong-key ciphertext (key XOR'd with `0xFF`) returns `ErrDecryptFailed`.
- On Linux/Windows builds, `Key()` returns `ErrUnsupportedPlatform`. (Build-tag-gated test; runs only on non-darwin CI lanes.)
- On macOS, `Key()` with `GRANOLA_SAFESTORAGE_KEY_OVERRIDE=<base64>` set returns the override and never shells out to `security`. (CI runs the macOS-tagged tests without real Keychain access.)
- `Available()` returns `false` after a `Key()` failure and `true` after success.
- Decrypting `testdata/fixture-supabase.enc` with `testdata/fixture-key.bin` produces valid JSON containing the expected key `workos_tokens` (sanity check on real shape).

**Verification:** `go test ./internal/granola/safestorage/...` passes on macOS and Linux CI lanes. Race detector clean (`go test -race`).

---

### U3. Patch `cache.go` to read `cache-v6.json.enc` when present

**Goal:** Make the existing `LoadCache(path string)` function in `internal/granola/cache.go` (line 266) discover and decrypt the `.enc` file automatically. Leave existing plaintext code path intact for tests and for users on pre-encryption Granola versions.

**Requirements:** Goals 1, 4, 5.

**Dependencies:** U2.

**Files:**
- `library/productivity/granola/internal/granola/cache.go` (MODIFY) — new internal helper `resolveCachePath()` that returns `(path, isEncrypted, error)`, and a refactor of the `os.ReadFile(path)` call site at line 270 to branch on `isEncrypted`.
- `library/productivity/granola/internal/granola/cache_test.go` (MODIFY) — add scenarios for the new branching.

**Approach:**
- Resolution order:
  1. `GRANOLA_CACHE_PATH` (explicit override) — read as-is, treated as plaintext. Preserves test fixtures.
  2. `cache-v6.json.enc` in the Granola support dir, if it exists — decrypt via `safestorage`.
  3. `cache-v6.json` in the Granola support dir — read as plaintext (legacy fallback for older Granola versions).
- When the `.enc` file decrypt fails, return a wrapped error from the load function (not a silent fallback to the stale plaintext). The doctor command surfaces this state; silent fallback would mask the problem.
- `DefaultCachePath()` continues to return the plaintext path string for backward compatibility (it's exported and used in tests). Add a new `DefaultEncryptedCachePath()` and `ResolveCachePath() (string, bool)` for the encrypted-aware callers.
- Cache the resolved path within a single `Load()` call but do not memoize across calls — callers may run `Load()` multiple times across a long-running process.

**Patterns to follow:**
- `granolaSupportDir()` in `internal/granola/workos.go` is the canonical way to get the support dir, including the `GRANOLA_SUPPORT_DIR` override. Reuse it (the function may need to be exported or moved into a shared package — minimal refactor).

**Test scenarios:**
- Load with `GRANOLA_CACHE_PATH` set to an existing plaintext fixture: reads as plaintext, ignores `.enc` even if present in the support dir. (Backward compat for test suite.)
- Load with only `cache-v6.json` present in support dir: reads plaintext (legacy Granola fallback).
- Load with only `cache-v6.json.enc` present and a valid Keychain key (via override env var): decrypts and parses.
- Load with both `.enc` and stale plaintext: prefers `.enc`. (This is the live production case on every current user.)
- Load with `.enc` present and a wrong/missing key: returns wrapped error referencing `safestorage.ErrDecryptFailed` or `safestorage.ErrKeyUnavailable`. Does NOT silently fall back to the stale plaintext.
- Load with `.enc` present but valid decrypt produces empty/malformed JSON: existing JSON parse error path is exercised unchanged.
- Edge: Load with neither file present returns a clear "cache not found, Granola may not be installed" error.

**Verification:** `go test ./internal/granola/...` passes. Manual sanity: on the dev macOS box with Granola signed in, `granola-pp-cli sync` reports nonzero meetings and transcript_segments. (Manual step deferred to U8.)

---

### U4. Patch `workos.go` to read `supabase.json.enc` when present

**Goal:** Make `loadFromSupabaseJSON` discover the encrypted token store. Auto-refresh logic (already implemented) continues to work unchanged because it operates on the in-memory `workosTokens` struct.

**Requirements:** Goals 1, 2.

**Dependencies:** U2.

**Files:**
- `library/productivity/granola/internal/granola/workos.go` (MODIFY) — at line 187 (`func loadFromSupabaseJSON`), change `os.ReadFile(supabaseJSONPath())` to a resolver that prefers the `.enc` file.
- `library/productivity/granola/internal/granola/workos_test.go` (MODIFY) — add scenarios mirroring U3's table.

**Approach:**
- Mirror U3's resolution order:
  1. `GRANOLA_WORKOS_TOKEN` env var (already supported) — fully bypasses both files.
  2. `supabase.json.enc` in support dir — decrypt via `safestorage`.
  3. `supabase.json` in support dir — read plaintext (legacy fallback).
- The fallback to `stored-accounts.json` (existing code path) stays in place for the case where neither supabase file is usable. Both `stored-accounts.json` and `stored-accounts.json.enc` should be checked when the time comes; for this round, only patch `supabase.json` since that's the primary surface and `stored-accounts.json` is rarely populated on modern Granola installs. Defer `stored-accounts.json.enc` to follow-up.
- The token-expiry / refresh logic at the bottom of `loadFromSupabaseJSON` operates on the parsed `workosTokens` struct and needs no changes.

**Patterns to follow:** Mirror U3's resolver shape so the two loaders look like siblings.

**Test scenarios:**
- `loadFromSupabaseJSON` with `GRANOLA_WORKOS_TOKEN` set: returns the env token, never touches files. (Existing behavior; assert it still holds.)
- With only plaintext `supabase.json`: returns parsed tokens (legacy behavior; assert unchanged).
- With only `supabase.json.enc` and valid key override: decrypts and returns parsed tokens.
- With both files present: prefers `.enc`. (Production case.)
- With `.enc` and missing/wrong key: returns wrapped error pointing to `safestorage.ErrKeyUnavailable` or `ErrDecryptFailed`. Token refresh logic is not invoked (we never had a token to begin with).
- With `.enc` decrypted plaintext that is valid JSON but missing `workos_tokens` field: returns the existing "supabase.json: workos_tokens missing" error unchanged.
- Edge: when both files are absent and `stored-accounts.json` is also empty, returns the existing "no Granola token found in supabase.json or stored-accounts.json" hint message. Verify the message still references both files by name so users know where to look.

**Verification:** `go test ./internal/granola/...` passes. After this lands together with U3, the CLI on a live Granola install can do `granola-pp-cli auth status` and report `authenticated: true`.

---

### U5. Extend `doctor` to report encrypted-store status and detect plaintext-stub staleness

**Goal:** Make `granola-pp-cli doctor` self-explanatory when something goes wrong with the new encrypted loaders. Today it reports `Auth: not configured` for an issue that's actually "your plaintext token expired because Granola encrypted the file 8 days ago" — wildly misleading.

**Requirements:** Goal 3.

**Dependencies:** U2, U3, U4.

**Files:**
- `library/productivity/granola/internal/cli/doctor.go` (MODIFY) — add an `EncryptedStore` check section that reads the sync state file (does not invoke `safestorage.Decrypt`).
- `library/productivity/granola/internal/cli/doctor_test.go` (CREATE if absent, MODIFY if present) — assert each verdict under different sync-state-file inputs.
- `library/productivity/granola/internal/granola/syncstate.go` (CREATE) — small helper exposing `WriteSyncState(status, errClass)` and `ReadSyncState()`. State file path: `~/.local/share/granola-pp-cli/sync_state.json` (honors `XDG_DATA_HOME` per existing CLI convention).
- `library/productivity/granola/internal/granola/syncstate_test.go` (CREATE) — round-trip read/write, missing-file behavior, malformed-file recovery.
- `library/productivity/granola/internal/cli/sync.go` (MODIFY, light touch) — call `WriteSyncState("ok", "")` after a successful sync; call `WriteSyncState("failed", <error-class>)` when `safestorage.Decrypt` returns an error during the load path. This is a 2-line change at the sync entry point, not a sync redesign.

**Approach (redesigned per doc-review):**

`doctor` cannot observe the "decrypt succeeded" state without triggering the Keychain prompt itself, and the previous "attempt-with-timeout" framing was unachievable (the `security` binary either prompts and blocks, or doesn't prompt and succeeds — no graceful middle). The redesign avoids that contradiction by reading a **sync-state file** that `sync` writes after each successful decrypt, rather than by re-running the decrypt at doctor time.

| State | Detector | Doctor verdict |
|---|---|---|
| Granola not installed | Support dir missing | INFO Encrypted store: no Granola install detected |
| Encrypted files absent (old Granola) | Support dir present, no `.enc` files | INFO Encrypted store: not in use (Granola pre-encryption) |
| Encrypted files present, never synced (or last sync failed) | `.enc` files present; sync state file (`~/.local/share/granola-pp-cli/sync_state.json`) is missing OR has `last_decrypt_status: "failed"` | INFO Encrypted store: present; run `granola-pp-cli sync` to authorize Keychain access |
| Encrypted files present, last sync decrypted successfully | Sync state file shows `last_decrypt_status: "ok"` and timestamp within last 7 days | OK Encrypted store: ok (last decrypted: <relative timestamp>) |
| Encrypted files present, last sync decrypt failed | Sync state file shows `last_decrypt_status: "failed"` with error class | FAIL Encrypted store: last sync failed to decrypt (<error class>) — see hint |

The 24h-mtime staleness check on `cache-v6.json` (plaintext) is dropped: it was unjustified scope creep, no Goal requires it, and the 24h threshold was arbitrary. The existing `Cache` check stays as-is.

`sync` is the only command that triggers the Keychain prompt. `doctor` is observation-only: it reads the sync state file and the support-dir contents but never invokes `safestorage.Decrypt`. First-time users get a clear instruction ("run `granola-pp-cli sync` to authorize") rather than a surprise prompt from a diagnostic command.

**Patterns to follow:** Existing doctor sections in `internal/cli/doctor.go` — each is a function returning `(status, message string, hint string)`. Mirror the shape.

**Test scenarios:**
- Doctor with no Granola support dir: reports `no Granola install detected` and exits 0.
- Doctor with support dir but no `.enc` files: reports `not in use (Granola pre-encryption)`, exits 0.
- Doctor with `.enc` files and no sync state file: reports `present; run sync to authorize Keychain access`, exits 0.
- Doctor with `.enc` files and sync state file showing `last_decrypt_status: "ok"` within 7 days: reports `ok (last decrypted: <relative>)`, exits 0.
- Doctor with `.enc` files and sync state file showing `last_decrypt_status: "failed"`: reports `last sync failed to decrypt` with the error class in the hint, exits 0 (warning, not failure — existing convention).
- Doctor in `--json` mode: the new section appears as a structured object with `state`, `last_decrypt_status`, and `last_decrypt_at` fields so agents can branch on it.
- Doctor must never invoke `safestorage.Decrypt` directly: verified by injecting a panicking key-fetcher mock and asserting doctor still completes.

**Verification:** `go test ./internal/cli/...` passes. Manual: on the dev box right now, `granola-pp-cli doctor` should change from `Auth: not configured / Cache: unknown` to `Encrypted store: present / first sync will prompt Keychain` after U2 lands but before the user accepts the prompt.

---

### U6. Update the pp-granola skill documentation for the new install flow

**Goal:** Bring the user-facing skill into agreement with the new CLI behavior. Today the skill tells users to set `GRANOLA_API_KEY` — that path was always largely cosmetic, and is even more confusing now that the real install flow is "sign in to Granola desktop, accept Keychain prompt." Skill update lives outside the CLI repo (in the user's local `~/.claude/skills/pp-granola/` and ultimately in the Printing Press cli-skills publish path).

**Requirements:** Goal 6.

**Dependencies:** U2, U3, U4. (No CI dependency on this unit; can land in parallel with U7a/U7b or after.)

**Files:**
- `~/.claude/skills/pp-granola/SKILL.md` (MODIFY) — development workspace copy; replace the "Auth Setup" and "Prerequisites" sections.
- `cli-skills/pp-granola/SKILL.md` (MODIFY) — published copy at the repo root, alongside sibling printing-press CLI skills (pp-airbnb, pp-allrecipes, etc.). Same content as the workspace copy; this is what ships to new installs.

**Approach:**
- Auth Setup section becomes three short paragraphs: (1) "Install Granola desktop and sign in." (2) "Run any granola-pp-cli command — first invocation will prompt for Keychain access. Click 'Always Allow' so subsequent runs are silent." (3) "Power users: set `GRANOLA_WORKOS_TOKEN` directly to bypass the Keychain prompt entirely (useful for CI)."
- Drop the `GRANOLA_API_KEY` and `auth set-token` instructions from the front matter — the public REST API path is rarely useful and is misleading as the primary auth surface. Move it to a "Optional: public API" footnote.
- Add a one-line note under the "Daily MEMO loop" recipe: "First-run note: a Keychain prompt fires; click Always Allow."
- Add a "Troubleshooting" stanza pointing users at `granola-pp-cli doctor` and listing the four `Encrypted store:` verdicts and what they mean.

**Patterns to follow:** Recent Printing Press skill updates that landed with auth-flow changes — e.g., `feat-npm-publish-handoff-plan.md` style.

**Test scenarios:** Test expectation: none — documentation update. Verified by U8's fresh-install dry run.

**Verification:** Read-through with a teammate (or self-review against a fresh macOS install). Cross-link from CLI's README so search-via-skill and search-via-CLI lead to the same instructions.

---

### U7a. Distribution: courtesy PR to DamienStevens/printing-press-library

**Goal:** Contribute the encryption-handling patch back to the original CLI author. Damien decides when (and whether) to merge into his fork; we don't gate user-facing delivery on his review window.

**Requirements:** Goal 6 (jointly with U7b — either merge unblocks Goal 6).

**Dependencies:** U2-U5 complete and tested.

**Files:**
- `library/productivity/granola/CHANGELOG.md` (CREATE or MODIFY) — add a `[1.1.0] - 2026-05-12` entry naming the encrypted-cache support.
- PR description draft (no repo file — lives in the PR body).

**Approach:**
- Open PR from `DamienStevens/printing-press-library:feat/granola` (with our patch commits on top) → `DamienStevens/printing-press-library:main`. PR description structure: "Background: Granola encrypted the local cache around early May 2026. Without these patches, the CLI is non-functional on every current install. This PR adds a `safestorage` package (macOS-only this round) plus surgical loader patches, preserves all existing plaintext code paths for tests and pre-encryption Granola versions, and extends `doctor` to surface diagnostic states via a sync state file. macOS first; Linux/Windows are stubbed with `ErrUnsupportedPlatform` and tracked in follow-up. Credit: original CLI authored by @DamienStevens; encryption layer contributed by @mvanhorn. Companion PR opened against mvanhorn upstream to unblock current users; merge order is your call."
- Link the companion U7b PR in the body.

**Patterns to follow:**
- `feat: ...` commit style from the existing branch history.
- CHANGELOG style from sibling CLIs.

**Test scenarios:** Test expectation: none — distribution step, not behavior change. Verified by U8.

**Verification:** PR URL in hand; link added to the U7b PR body for cross-reference.

---

### U7b. Distribution: registry entry PR to mvanhorn/printing-press-library

**Goal:** Make the patched CLI installable via `npx -y @mvanhorn/printing-press install granola --cli-only` immediately, without waiting on Damien's review window. The Problem Frame establishes every current user is broken right now; this PR is what unblocks them.

**Requirements:** Goal 6 (jointly with U7a).

**Dependencies:** U2-U5 complete and tested. Independent of U7a — open in parallel.

**Files:**
- `library/productivity/granola/go.mod` (MODIFY) — confirm module path `github.com/mvanhorn/printing-press-library/library/productivity/granola`. No new deps; safestorage uses only stdlib.
- `registry.json` at repo root (MODIFY) — add the granola entry with category `productivity`, path `library/productivity/granola`, and the standard `printer` / `api` / `description` fields mirroring siblings (e.g., `notion`, `slack`).

**Approach:**
- Open PR against `mvanhorn/printing-press-library:main` carrying the encryption patch commits + the registry entry. Co-author trailers attribute Damien on the commits that originate from his branch (`Co-Authored-By: Damien Stevens <…>`).
- PR description: "Adds granola CLI to the public registry. Original CLI authored by @DamienStevens (companion courtesy PR open against his fork at <link>); this PR carries the encryption-handling layer needed for any current Granola desktop user to use it. macOS-only this round; Linux/Windows stubs return `ErrUnsupportedPlatform`. Merging this PR makes the CLI installable via `npx ... install granola` immediately; @DamienStevens' merge of the companion PR is independent and unblocks nothing downstream."
- After merge, verify the registry entry is reachable: `curl -s https://raw.githubusercontent.com/mvanhorn/printing-press-library/main/registry.json | jq '.entries[] | select(.name == "granola")'` returns the new entry.

**Patterns to follow:**
- Registry entry shape: copy `notion` or `slack` entry verbatim and swap names/paths.
- Co-author trailer format already used in the repo's history.

**Test scenarios:** Test expectation: none — distribution step, not behavior change. Verified by U8.

**Verification:**
- PR URL in hand; link added to U7a PR body.
- After upstream merge, `curl` confirms the granola entry in registry.json.
- Fresh shell on a clean macOS machine running `npx -y @mvanhorn/printing-press install granola --cli-only` produces a working `granola-pp-cli` binary; `granola-pp-cli doctor` confirms decrypted-store-ready after a `sync`.

---

### U8. End-to-end integration verification on a live Granola install

**Goal:** Prove the full chain works on the user's actual machine before declaring done. This is the unit that catches reverse-engineering mistakes, build-tag mistakes, and skill/docs drift in one pass.

**Requirements:** All goals (1-6).

**Dependencies:** U2-U5 complete; at least one of U7a or U7b merged (U8's reinstall step depends on at least one merge surface being live).

**Files:** No code. Optional: `docs/plans/notes/2026-05-12-001-verification-log.md` capturing the run.

**Approach:**
Run, in order:
1. Wipe the local CLI binary: `rm ~/printing-press/library/granola/granola-pp-cli` (or whatever the install location is on the verifying machine).
2. Reinstall via the public path: `npx -y @mvanhorn/printing-press install granola --cli-only`. Confirm the binary lands and `granola-pp-cli --version` reports the new version.
3. `granola-pp-cli doctor`. Expected: `Encrypted store: present; first sync will prompt Keychain access`. Auth state should report not-yet-authenticated until first refresh.
4. `granola-pp-cli sync`. Expected: macOS Keychain prompt fires. Click "Always Allow". Sync reports nonzero `meetings`, `transcript_segments`, `documents` counts.
5. `granola-pp-cli doctor` again. Expected: `Encrypted store: ok`, `Auth: ok`, `Cache: ok` with nonzero `db_bytes` and a recent `last_synced_at`.
6. `granola-pp-cli meetings list --limit 3 --agent --select id,title,started_at`. Expected: three rows with real titles and dates.
7. `granola-pp-cli memo run --since 24h --to /tmp/memo-test --json`. Expected: ndjson stream of any meetings since yesterday written to the directory.
8. `granola-pp-cli auth status --agent`. Expected: `authenticated: true`, source pointing at `supabase.json.enc`.

**Test scenarios:** Test expectation: none — this IS the manual integration verification. Failures in any step trigger a rollback or a hotfix PR.

**Verification:** All 8 steps succeed on a Granola-signed-in macOS machine. If any step fails, document the failure in the verification log and decide between hotfix-PR or revert.

---

## Scope Boundaries

### In scope
- macOS-only `safestorage` with AES-GCM decryption.
- Surgical patches to `cache.go` and `workos.go` load paths.
- `doctor` enhancements that surface the new states clearly.
- Skill update (`pp-granola/SKILL.md`) so the install flow matches the new auth shape.
- PR routing through DamienStevens; registry entry into mvanhorn upstream.

### Out of scope (Deferred to Follow-Up Work)
- Linux `libsecret` implementation of `safestorage.Key()`.
- Windows DPAPI implementation of `safestorage.Key()`.
- `stored-accounts.json.enc` discovery (rarely populated; the encrypted supabase file covers the primary token surface today).
- Official Granola OAuth MCP integration (`https://mcp.granola.ai/mcp`) as a fallback when local decryption is unavailable.
- A `granola-pp-cli warm-keychain` subcommand to pre-authorize the Keychain prompt outside of sync.
- Tracking and auto-handling Granola encryption-scheme rotation if/when it happens (we'll handle the first rotation reactively).

### Outside this product's identity
- Re-implementing Granola's network API surface (covered by chrisguillory/granola-mcp, pedramamini/GranolaMCP, and the official OAuth MCP).
- Reverse-engineering Granola's WorkOS application setup beyond reading the existing client_id (`client_01HJK46TGGY2DFQ2NX9P9XYJZN`) that the community already documented.

---

## Risks and Mitigations

**R1. Encryption scheme guess is wrong (U1 fails the AES-128-GCM check).**
- Likelihood: moderate. The byte arithmetic is suggestive but not proof.
- Impact: high — U2 onwards is gated on the scheme.
- Mitigation: U1 includes explicit fallback schemes (AES-256-GCM/SHA-256, PBKDF2 derivation, AES-CBC, app.asar inspection). Time-box to 2 hours; if no candidate works, stop and revise the plan rather than ship a wrong-scheme implementation that succeeds by coincidence on a corrupt fixture.

**R2. Granola rotates the encryption scheme in a future release.**
- Likelihood: moderate — they already changed once.
- Impact: high — silent breakage for all users.
- Mitigation: Decryption is isolated in `safestorage`. Add a `granola-pp-cli feedback` auto-emit in doctor when decrypt fails, so we get fast signal. Document the scheme test vector in `safestorage/testdata/` and `docs/plans/notes/...` so a future investigator can confirm "yes, the scheme changed" quickly. Long-term: the Phase 2 official-MCP fallback would route around rotation entirely.

**R3. Keychain prompt UX is jarring on first run.**
- Likelihood: certain (this is how macOS Keychain works).
- Impact: low — one-time prompt with an "Always Allow" option.
- Mitigation: Mention prominently in the skill's install instructions and in the CLI's first-run message. `doctor` pre-announces the prompt so users aren't surprised.

**R4. ToS / posture concerns about reading Granola's encrypted cache.**
- Likelihood: low — the user owns their Keychain and the data; we're not bypassing auth, we're decrypting data the user has legitimate access to. But Granola encrypted on purpose.
- Impact: reputational, not functional.
- Mitigation: One-line note in the CLI README acknowledging that the desktop app encrypts these files and that this CLI reads them with the user's Keychain consent. No `--force-decrypt` or `--bypass-keychain` flags that would let someone use this against another user's account. We're not advertising this as "circumventing" anything — we're providing typed access to data the user owns.

**R5. CI lacks macOS Keychain access, so tests need a key-override path.**
- Likelihood: certain — CI lanes for Go projects rarely have populated Keychains.
- Impact: low if planned for, high if not (silent test gaps).
- Mitigation: `GRANOLA_SAFESTORAGE_KEY_OVERRIDE` env var (Unit 2) lets tests inject a known key. Fixture files are committed pre-encrypted with that key. Macos-tagged tests run on CI without ever touching the real Keychain.

**R6. PR to DamienStevens stalls.**
- Likelihood: low to moderate — Damien is the original author but may not be actively monitoring the branch.
- Impact: distribution path blocked.
- Mitigation: After 7 days without review, ping Damien directly via the contact channel referenced in the workos.go copyright line; offer to either land it on his branch ourselves or to fork+land upstream with a clear "co-authored by" credit. If the branch goes inactive entirely, escalate to mvanhorn upstream with a respectful fork rationale.

**R7. Plan-time encryption-scheme finding diverges from production at install time (Granola desktop auto-updates).**
- Likelihood: low in the near term, rises over months.
- Impact: depends on whether the new scheme is a key rotation (re-auth fixes it) or an envelope change (code change required).
- Mitigation: `safestorage.Decrypt` checks the input length up front and returns a typed error that doctor can surface specifically. We'll see the divergence in support tickets / doctor failures before users notice silently-empty outputs.

---

## Key Technical Decisions

**D1. Shell out to `/usr/bin/security` instead of CGO into Security.framework.**
- Why: The rest of the CLI is pure Go. Adding CGO for a single Keychain read complicates the build, the cross-compile story, and the goreleaser pipeline. The shell-out's UX (Keychain prompt with "Always Allow") is exactly what we want.
- Trade-off: Slightly slower first-call (process spawn overhead is ~50ms), but cached in `sync.Once` after success.
- When this gets revisited: If we add Linux/Windows decryption in a follow-up and the cross-platform key-fetching code feels like it wants a common abstraction, evaluate whether to standardize on a small CGO Keychain wrapper across all three platforms or to keep three independent shell-outs.

**D2. Fail loudly on decrypt failure rather than silently falling back to stale plaintext.**
- Why: The current pathology (CLI returns empty results) is misleading and hard to debug. A typed error surfacing through `doctor` is much better signal than zero rows.
- Trade-off: A misconfigured key now produces a hard failure where today it produces silent empty output. That's the intended change.
- When this gets revisited: If we ever support a heterogeneous mix of Granola versions in a single org (rare), reconsider.

**D3. Build-tag-gated platform files (`_darwin.go`, `_linux.go`, `_windows.go`) rather than runtime branching.**
- Why: Standard Go convention. Keeps platform-specific code obvious. Lets the Linux/Windows follow-ups land as one-file additions without restructuring.
- Trade-off: None meaningful.

**D4. Commit pre-encrypted test fixtures rather than encrypting on test run.**
- Why: Deterministic test runs, no Keychain dependency in CI, no flakiness from RNG seeding the GCM nonce per test invocation.
- Trade-off: Fixture regeneration when test data changes — small. Add a `safestorage/testdata/regenerate.sh` helper for future maintainers.
- Fixture nonce strategy: `regenerate.sh` uses an all-zero (or other fixed deterministic) 12-byte nonce so the committed `.enc` bytes are stable across regen runs and produce zero-diff churn. This is acceptable because the fixture key is also test-only and committed; production `Decrypt()` of course handles any random nonce read from the first 12 bytes of input.

**D5. PR upstream to DamienStevens first, not directly to mvanhorn main.**
- Why: Damien is the original author. Respect the authorship line. He contributes the original CLI; we contribute one focused improvement.
- Trade-off: Adds one review hop. Mitigated by R6's escalation path and by D7 (parallel PRs to both forks, not sequential).

**D6. CLI is read-only against the WorkOS token store; never call `RefreshAccessToken` when the source is the encrypted file.**
- Why: WorkOS uses single-use refresh-token rotation (`workos.go:28` — "a new access_token plus a NEW refresh_token (single-use rotation)"). If the CLI reads the live refresh token from `supabase.json.enc` and refreshes it, the rotated token never makes it back into the encrypted file (we have no encrypt/write path). Next time Granola desktop tries to refresh, WorkOS rejects the now-invalidated token and signs the user out of the desktop app. This is unacceptable collateral damage on a CLI whose purpose is to coexist with desktop, not replace it.
- Resolution: `loadFromSupabaseJSON` may read the current access_token and call API endpoints with it, but the CLI's auto-refresh path is disabled when the token came from `supabase.json.enc`. On 401 / token-expired, the CLI surfaces a clear error ("Granola access token expired — open Granola desktop briefly to refresh, then retry"). Power users who need long-running CLI-side refresh can set `GRANOLA_WORKOS_TOKEN` directly; that path is opt-in and the desktop-sign-out risk is documented.
- Trade-off: A user whose desktop is asleep can't run granola-pp-cli until they wake it. Acceptable — meeting transcripts haven't moved when desktop is asleep anyway. The write-back path (encrypting a rotated token back into `.enc`) is deferred as follow-up.
- Affects: U4 (load logic), R8 (new risk entry for desktop-sign-out under `GRANOLA_WORKOS_TOKEN`), skill docs (U6 mentions the trade-off).

**D7. Open the DamienStevens PR and the mvanhorn registry PR in parallel, not sequentially.**
- Why: The Problem Frame states every current user is broken right now. D5's courtesy-PR hop is appropriate authorship hygiene, but gating Goal 6 (CLI ships through standard install path) on Damien's review window is the wrong sequencing. The CLI's user-facing fix and Damien's authorship credit are independent deliverables.
- Resolution: U7 splits into U7a (open PR to DamienStevens fork, asking him to merge at his pace) and U7b (open PR to mvanhorn upstream with the registry entry + decryption patch + co-author credit to Damien). Both PRs reference each other; whichever Damien wants becomes the canonical merge — we don't predetermine the order.
- Trade-off: Some duplication in PR descriptions. Negligible.

---

## Alternative Approaches Considered

**A1. Skip cache decryption entirely, build the official Granola OAuth MCP fallback instead.**
- Rejected per user decision. The official MCP exposes only 4 tools and would kill ~80% of the CLI's differentiated features (memo run, talktime, calendar overlay, TipTap extractor, recipes coverage, chat threads). Captured as a separate plan for future Phase 2 work.

**A2. Read the WorkOS token from the Granola process's memory (e.g., via `lldb` attach) and skip cache decryption.**
- Rejected. Process-attach is more fragile than file decryption, requires more entitlements, and produces a worse UX (would need to relaunch Granola periodically). Cache decryption is the cleanest seam.

**A3. Ship a Python sidecar that decrypts the files and pipes plaintext to the Go CLI.**
- Rejected. Adds a runtime dependency, complicates the installer, and the encryption is simple enough (AES-GCM round-trip) that the Go stdlib handles it without help.

**A4. Add `--use-plaintext` flag that re-reads the stale plaintext as a last-resort fallback.**
- Rejected. Stale data is worse than no data; encourages confused bug reports.

---

## Dependencies / Prerequisites

- Confirmed encryption scheme from Unit 1 (gates U2-U8).
- Granola desktop signed in on the verifying macOS machine (gates U8).
- `gh` CLI authenticated with push access to both `DamienStevens/printing-press-library` and `mvanhorn/printing-press-library` (or forks thereof) (gates U7a and U7b respectively).
- npm publish access not required for this plan (the installer pulls registry dynamically).

---

## Open Strategic Questions (Deferred to After Phase 1 Ships)

These questions were surfaced by doc-review (product-lens) and are real — but answering them changes Phase 2+ scope, not Phase 1 execution. Carry forward; revisit once the Phase 1 patch is in users' hands.

1. **Do most users actually need the 35 differentiated commands, or would the official MCP cover the modal use case?** A1's rejection cites "loses 80% of features" but presents no evidence on actual command-mix usage. After Phase 1 ships, instrument or survey to find out — if the modal user lives in `meetings list` / `export` / `memo run` and rarely touches `talktime` / `calendar overlay` / `recipes coverage`, the Phase 2 MCP-only path becomes a credible primary, with the decryption layer as a power-user opt-in. If the long tail is widely used, Phase 1's investment is well-spent.

2. **Should the registry-publish step gate on a soft-launch window?** U7b ships to the public registry immediately. An alternative — publish to a "beta" registry entry or hold the registry PR for 14-30 days of fork-URL-only installs while observing one Granola desktop release cycle — would trade quicker user delivery for less exposure if scheme drift hits during the first weeks. The current plan optimizes for "users are broken now, ship the fix"; a soft-launch optimizes for "every Granola release becomes our P0 only after we've seen one rotation in the wild." Decision likely tied to (1): if Phase 2 MCP fallback lands fast, soft-launch matters less.

These are not blockers. Phase 1 ships against the current plan; reopen at Phase 2 planning.

---

## Open Questions Deferred to Implementation

- Whether `GRANOLA_SAFESTORAGE_KEY_OVERRIDE` should be a single base64 string or a path to a key file. Decide based on test ergonomics once safestorage_test.go is in flight.
- Exact go.mod version bump strategy — confirm against `feat-npm-publish-handoff-plan.md` whether the printing-press-library uses per-CLI tagging or repo-wide.
- Whether to expose `safestorage.Available()` publicly or keep it package-private. Default to package-private; expose only if `doctor` needs it from another package.
- Whether the `stored-accounts.json.enc` fallback in U4 is worth wiring up in this round or deferring. Defer unless Unit 1 turns up evidence that `stored-accounts.json.enc` exists and is populated on modern installs.

---

## System-Wide Impact

- **Build pipeline:** No new dependencies. macOS build picks up `safestorage_darwin.go`; Linux/Windows pick up the stubs. goreleaser config unchanged.
- **Test matrix:** macOS CI gains a tagged test suite for `safestorage`. Linux/Windows CI runs the cross-platform tests and the stub returns. No new external services in CI.
- **Operational footprint:** Adds one shell-out to `/usr/bin/security` per CLI invocation (cached after first success). No new network calls. No new logs by default; doctor surfaces diagnostic info on demand.
- **Documentation:** Skill update lands in parallel via U6; CLI README needs a paragraph about the new install flow.

---

## Verification Strategy

- Unit-test coverage for `safestorage` (round-trip, edge cases, errors) — committed in U2.
- Unit-test coverage for cache.go and workos.go resolver branching — committed in U3, U4.
- Unit-test coverage for the new doctor section — committed in U5.
- Integration verification via U8 — manual, on a live Granola install, fully scripted in the verification log.
- Smoke test post-merge: anyone fresh-installing the CLI runs `granola-pp-cli sync` and gets nonzero rows.

---

## Distribution Checklist (post-merge)

- [ ] U7a: PR opened against `DamienStevens/printing-press-library`, crediting language + scheme-finding note linked, links to companion U7b PR.
- [ ] U7b: PR opened against `mvanhorn/printing-press-library:main` (in parallel with U7a — not gated on Damien's review) carrying registry entry + patch commits + co-author trailers to Damien, links to U7a.
- [ ] U7b merged; `curl` confirms granola entry is reachable in registry.json.
- [ ] U7a merged (independent timing — Damien's pace; tracked separately from user-facing delivery).
- [ ] Skill update merged to wherever cli-skills publishes from (confirm path in U6).
- [ ] Verification log captured at `docs/plans/notes/2026-05-12-001-verification-log.md` (optional — only required if U8 surfaced any failures worth recording).
- [ ] One-line announcement in the relevant channel pointing to the install command and the doctor's new diagnostic states.
