# Granola .enc decryption scheme — empirical finding (U1)

Captured 2026-05-12. Granola desktop 7.205.0 on macOS.

## Result

**Two-tier encryption.** Both `cache-v6.json.enc`, `supabase.json.enc`, and
`user-preferences.json.enc` share the same scheme.

## Layer 1: storage.dek → 32-byte Data Encryption Key (DEK)

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

Reproducible test against the live machine that captured this finding:

```
Keychain entry: service="Granola Safe Storage", account="Granola Key"
Keychain value (base64): <pull live with `security find-generic-password -s "Granola Safe Storage" -w`>

storage.dek first 32 bytes (hex):
  76 31 30 cc e1 61 00 49 ec 76 5b 19 bf fa fa a1
  ab f7 74 29 5b b7 9c 6d a6 24 03 aa 0d db f7 70

After layer-1 unwrap:
  44-char base64 plaintext, decodes to 32 raw bytes (the DEK)

cache-v6.json.enc first 32 bytes (hex):
  32 07 b2 e3 b2 ea df d1 40 82 72 79 cc 82 1c 6f
  39 bf dc cc dc 9d 90 40 ab 7d d6 d7 57 80 f2 7e
  ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^
  nonce (12 bytes)                  ciphertext start

After layer-2 unwrap of cache-v6.json.enc:
  ~4 MB JSON. Top-level keys: ["cache"]
  cache.state has ~68 keys including: transcripts, entities, documentLists,
  documentListsMetadata, panelTemplates, publicRecipes, featureFlags, workspaceData
```

## Cache schema change — separate from encryption

Empirical probe on 2026-05-12 shows the decrypted cache **no longer contains
documents at `cache.state.documents`**. The schema present today:

- `cache.state.transcripts` — `dict[document_id, [transcript_segment]]` (24 entries)
- `cache.state.entities` — only `chat_thread` and `chat_message` (216 + 241 entries)
- `cache.state.documentLists` — folder→[doc_id] mapping (5 folders)
- `cache.state.documentListsMetadata` — folder metadata (title, members, ...)
- `cache.state.panelTemplates` — 31 panel templates
- `cache.state.publicRecipes` — 57 public recipes
- ... and 60+ smaller state keys

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
