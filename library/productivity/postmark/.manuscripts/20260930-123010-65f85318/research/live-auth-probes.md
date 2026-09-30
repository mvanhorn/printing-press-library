<!-- slop-gate: off -->
# Live auth probes (2026-09-30, read-only GETs)

Credentials: a password-manager entry holding the account and server tokens (one account; ACCOUNT_API identical across Postmark_* items).

| Request | Headers | Result |
|---|---|---|
| GET /servers?count=50&offset=0 | account | 200, TotalCount 6 (Main App, Staging, Billing, Marketing Site, Docs, Support); each server includes ApiTokens[] |
| GET /server | server (Main App) | 200, Name "Main App" |
| GET /server | server + account | 401, ErrorCode 10 |
| GET /servers?count=1 | server + account | 200 |
| GET /messages/outbound?count=1 | server + account | 200, TotalCount 28 |

Implications:
- Do not send both auth headers globally; select the header per endpoint family (server-token vs account-token ops).
- Account token can resolve any server's token via GET /servers[/{id}] ApiTokens, enabling `--server <name>` switching from a single account credential.
