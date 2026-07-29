# Granola .enc decryption scheme — empirical finding (U1)

Captured 2026-05-12. Granola desktop 7.205.0 on macOS.
Tier-1 update captured 2026-07-25 against Granola desktop 7.447.1 — see
"Tier-1 update" below before re-deriving anything.

## Result

**Two-tier encryption.** Both `cache-v6.json.enc`, `supabase.json.enc`, and
`user-preferences.json.enc` share the same scheme.

## Tier-1 update (2026-07-25, Granola 7.447.1): the DEK moved

**Layer 2 below is unchanged.** AES-256-GCM, 12-byte nonce, 16-byte tag, no
AAD — still exactly as documented. Do not re-derive it, and do not touch the
tier-2 decrypt path chasing this.

**Layer 1 is dead for third parties.** Granola 7.447.1 moved the 32-byte DEK
out of `storage.dek` and into the macOS **data-protection** keychain, under
service `com.granola.app.dek` in access group `QZ7DHHLN25.granola`. That
access group is gated by a `keychain-access-groups` entitlement bound to
Granola's Apple Team ID, so no third-party binary can read it. The decompiled
app shows a migrate-then-unlink flow: on first launch after the update it
imports the old DEK into a bundled `keychain.node` module and then **unlinks
`storage.dek`**.

Two consequences worth carrying forward:

1. The migration **imports** the existing DEK rather than generating a fresh
   one. A pre-migration `storage.dek` recovered from a backup therefore still
   decrypts today's `cache-v6.json.enc` when its 32 raw bytes are supplied
   base64-encoded through `GRANOLA_SAFESTORAGE_KEY_OVERRIDE`. That override is
   a genuine remedy, not just a test seam.
2. The unlink happens **only after a successful import**, and the import is
   wrapped in a five-attempt retry because it can fail at launch. A failed
   migration therefore leaves `storage.dek` on disk while the app has already
   rotated to a fresh Keychain DEK. Such an install is migrated with a stale
   key file — it is **not** an unmigrated install, and its decrypt failure is
   not envelope drift.

`safestorage` classifies these states from observable state only, never from a
Granola version string (version sniffing breaks on every release):

| support dir | `cache-v6.json.enc` | `storage.dek` | `Granola Safe Storage` Keychain item | classification |
|---|---|---|---|---|
| absent | — | — | not probed | `ErrKeyUnavailable`, Granola not installed |
| present | absent | absent | not probed | `ErrKeyUnavailable`, pre-encryption build |
| present | present | absent | not found | `ErrKeyUnavailable`, signed out |
| present | present | absent | resolves | **`ErrSchemeMigrated`**, DEK moved upstream |
| present | present | present, unwraps, GCM rejects | resolves | **`ErrSchemeMigrated`**, stale key file from a failed migration |
| present | present | present and valid | resolves | unchanged two-tier unwrap |

`ErrSchemeMigrated` errors also report as `ErrKeyUnavailable` under
`errors.Is`. Both statements are true, and the refinement must not silently
delete fallbacks that only know the older sentinel.

## Layer 1: storage.dek → 32-byte Data Encryption Key (DEK)

Historical for installs that predate the 7.447.1 migration, and still live for
anyone supplying a recovered DEK through the override.

`storage.dek` is encrypted with **Electron's standard `safeStorage` v10 envelope**
(Chromium's OSCrypt on macOS):

- 3-byte prefix: ASCII `v10`
- Cipher: **AES-128-CBC** with PKCS7 padding
- Key derivation: **PBKDF2-HMAC-SHA1**, with:
  - password = the **base64 string** as bytes (NOT the base64-decoded raw bytes)
  - salt = `b"saltysalt"`
  - iterations = `1003`
  - key length = 16 bytes
- IV = 16 bytes of ASCII space (`0x20`)
- Plaintext = UTF-8 base64 string; decoding gives **32 random bytes**, the DEK.

The "password = base64 string, not raw bytes" subtlety mirrors Chromium's OSCrypt
implementation: macOS Keychain stores the bytes as base64, and Chromium/Electron
treats the base64 string itself as the PBKDF2 password input.

## Layer 2: file.json.enc → JSON

The three encrypted JSON files (`cache-v6.json.enc`, `supabase.json.enc`,
`user-preferences.json.enc`) all use:

- Cipher: **AES-256-GCM** (DEK is 32 bytes)
- Envelope: `nonce(12) || ciphertext || tag(16)`
- AAD: none (verified empirically)

The decrypted plaintext is UTF-8 JSON, parseable with no trailing bytes.

## Test vector

Reproducible against any Granola desktop install. The synthetic vectors in
`fixture-key.bin`, `fixture-cache.enc`, and `fixture-supabase.enc` (this
directory) cover the round-trip for unit tests. To verify on a real install:

```
Keychain entry: service="Granola Safe Storage", account="Granola Key"
Keychain value (base64): <pull live with `security find-generic-password -s "Granola Safe Storage" -w`>

storage.dek prefix (first 3 bytes): ASCII "v10" (Electron safeStorage marker;
  identical on every Granola install). Remaining bytes are AES-128-CBC
  ciphertext of a base64-encoded 32-byte DEK, derived per the layer-1 spec
  above.

After layer-1 unwrap:
  44-char base64 plaintext, decodes to 32 raw bytes (the DEK).

cache-v6.json.enc envelope: nonce(12) || ciphertext || tag(16).
  After layer-2 (AES-256-GCM with the DEK):
  UTF-8 JSON, top-level keys ["cache"]; cache.state carries the per-user
  transcripts/entities/documentLists/etc. surface described below.
```

(Per-install byte dumps and per-user cache counts are intentionally not
checked in. To inspect your own install, run the Python probes in
`/Users/<you>/code/granola-pp-encrypted-cache/` or write a one-off Go
test using the `safestorage.Decrypt` API. See README.md in this directory.)

## Cache schema shape — separate from encryption

The decrypted cache **does not contain meeting documents at
`cache.state.documents`** on modern Granola installs. Granola moved
documents server-side at approximately the same time as the encryption
rollout. The cache.state surface present today carries:

- `transcripts` — `dict[document_id, [transcript_segment]]`
- `entities` — `chat_thread` and `chat_message` dicts (no `document`)
- `documentLists` — folder → [doc_id] mapping
- `documentListsMetadata` — folder metadata (title, members, …)
- `panelTemplates` — panel template definitions
- `publicRecipes`, `userRecipes`, `sharedRecipes`, `unlistedRecipes`
- … plus ~60 smaller state keys (`featureFlags`, `workspaceData`,
  `multiChatState`, etc.).

Documents (meeting metadata + notes + attendees) are **fetched from the network**
via `https://api.granola.ai/v2/get-documents` and `/v1/get-documents-batch`.
This is a Granola desktop architectural shift unrelated to encryption — see
asar inspection of Granola 7.205.0's `dist-electron/main/index.js`.

This affects the granola-pp-cli implementation strategy: decrypting the cache
alone yields transcripts, folders, recipes, and panels, but **not the meeting
list**. The existing `internalapi.go` already exposes `GetDocuments` and
`GetDocumentsBatch` methods; `sync.go` must call them in addition to reading
the decrypted cache.

## Source of truth

This finding is derived from inspecting Granola's bundled JavaScript at
`/Applications/Granola.app/Contents/Resources/app.asar` (Granola 7.205.0).
Relevant code paths:

- `dist-electron/main/index.js`: `safeStorage.encryptString` / `decryptString`
  on `storage.dek`; `createStorage({file: "cache-v6.json", encrypted: !0, ...})`
  for the cache file; `createStorage({file: "supabase.json", encrypted: !0, ...})`
  for the auth store; `createStorage({file: "user-preferences.json",
  encrypted: !0, ...})` for prefs.
- The DEK is created via `createDek()` (calls `generateDek()`, encrypts the
  result with `safeStorage.encryptString`, writes to `storage.dek`).
- `getCacheStorage()` reads the file, decrypts with the DEK via what is
  observed as standard AES-256-GCM (envelope confirmed empirically).
