#!/usr/bin/env bash
# Regenerate the committed test fixtures with a fixed (all-zero) nonce so
# the output is byte-stable across runs. The test DEK is synthetic, hand-
# entered below, and committed to source control. It is NOT a Granola key
# and cannot decrypt any real Granola file (see README.md in this dir).
#
# Per plan D4: fixed nonce is acceptable for test fixtures because the
# fixture key is also test-only and committed.
set -euo pipefail
cd "$(dirname "$0")"

# Synthetic 32-byte DEK (hex). Generated once via `openssl rand -hex 32`
# and committed alongside the fixtures. Production Decrypt() accepts any
# 32-byte DEK; this constant is for tests only.
DEK_HEX="000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
NONCE_HEX="000000000000000000000000"

python3 - "$DEK_HEX" "$NONCE_HEX" <<'PY'
import sys, os
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
dek = bytes.fromhex(sys.argv[1])
nonce = bytes.fromhex(sys.argv[2])
assert len(dek) == 32 and len(nonce) == 12

# Save DEK as binary fixture (used by Go tests to set GRANOLA_SAFESTORAGE_KEY_OVERRIDE).
with open("synthetic-aes-gcm-test-dek.bin", "wb") as f:
    f.write(dek)

# Two small JSON plaintexts that mirror the shapes of the real files.
plaintexts = {
    "fixture-supabase.enc": (
        b'{"workos_tokens":"{\\"access_token\\":\\"test-access\\",\\"refresh_token\\":\\"test-refresh\\",\\"expires_in\\":21599,\\"obtained_at\\":1700000000000}",'
        b'"session_id":"test-session-id","user_info":"{\\"id\\":\\"test-uid\\",\\"email\\":\\"test@example.com\\"}"}'
    ),
    "fixture-cache.enc": (
        b'{"cache":{"state":{"transcripts":{},"entities":{"chat_thread":{},"chat_message":{}},'
        b'"documentLists":{},"documentListsMetadata":{},"panelTemplates":{},"workspaceData":{}}},'
        b'"version":8}'
    ),
}
aes = AESGCM(dek)
for fname, pt in plaintexts.items():
    ct = aes.encrypt(nonce, pt, None)
    with open(fname, "wb") as f:
        f.write(nonce + ct)
    print(f"wrote {fname}: {len(nonce + ct)} bytes ({len(pt)} bytes plaintext)")
PY

echo "Fixtures regenerated. Base64 DEK for tests:"
python3 -c "import base64; print(base64.b64encode(bytes.fromhex('$DEK_HEX')).decode())"
