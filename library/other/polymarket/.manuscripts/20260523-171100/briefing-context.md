# User Briefing Context

User intent (Indonesian → English):
"I'll use a wallet myself - you map out all the functions and set up the env vars; I'll fill in the PK (private key) later."

## Implications for absorb manifest

1. **Full trading scope**: build place/cancel/replace order, balance queries, position management - NOT stubs.
2. **L1 wallet auth flow**: derive L2 API credentials from a Polygon EOA private key.
3. **Env vars to set up now (user fills later)**:
   - `POLYMARKET_PRIVATE_KEY` (the EOA wallet PK, hex 0x-prefixed)
   - `POLYMARKET_API_KEY` (L2, derived via `auth derive` from L1)
   - `POLYMARKET_API_SECRET` (L2 HMAC secret)
   - `POLYMARKET_API_PASSPHRASE` (L2)
   - `POLYMARKET_FUNDER` (optional proxy wallet, address; for browser-wallet flow)
   - `POLYMARKET_SIGNATURE_TYPE` (0=EOA / 1=email-magic / 2=browser-wallet; default 0)
   - `POLYMARKET_CHAIN_ID` (default 137 Polygon mainnet)

## Auth strategy in CLI
- `auth derive` reads `POLYMARKET_PRIVATE_KEY`, calls POST `/auth/api-key`, prints API key+secret+passphrase the user copies to their env.
- `doctor` validates which env vars are present and what scopes they unlock.
- Trading commands require L1 PK (for order signing) AND L2 creds (for HMAC headers).
- Read commands work with no auth.
