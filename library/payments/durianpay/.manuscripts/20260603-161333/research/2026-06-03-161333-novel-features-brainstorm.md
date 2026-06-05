# Novel Features Brainstorm — durianpay (2026-06-03)
Subagent output (3-pass: customer model → candidates → adversarial cut). Survivors flow into the absorb manifest transcendence table.

## Customer model

**Persona A — Rangga, integration engineer at an Indonesian merchant (mid-size e-commerce)**
- Today: Durianpay ReadMe docs in three tabs, Postman with pre-request token scripts, scratch sign.js hacked from Midtrans demo. Cannot localize SNAP 403s (signature vs timestamp vs external-id vs token expiry). Re-mints token every 15 min (TTL 900s).
- Weekly ritual: wiring a new payment method — pick surface, build signed request, fire at sandbox, decode response code, fix, repeat.
- Frustration: the SNAP dual-signature scheme and debugging which of five inputs caused the 403.

**Persona B — Sari, finance-ops / reconciliation analyst**
- Today: pulls payments + orders into CSVs, Excel VLOOKUP to find unmatched/over-refunded items; checks balances one call at a time.
- Weekly ritual: period-close reconciliation — match payments to orders, reconcile refunds, confirm disbursements landed.
- Frustration: every cross-entity question is a manual join across CSVs.

**Persona C — Dewi, Durianpay solutions/CSM engineer onboarding merchants**
- Today: dictates openssl commands over calls, walks merchants through public-key upload, token mint, sandbox magic values; Ctrl-Fs the 48-row response-code table.
- Weekly ritual: onboarding/triaging merchant integrations.
- Frustration: keygen + signature setup is where merchants get stuck; decoding response codes/sandbox rules means leaving the terminal.

## Candidates (pre-cut)
C1 snap sign --debug (KEEP) · C2 snap keygen + token --status (KEEP) · C3 pay/payout smart routing (KEEP) · C4 route explainer (soft) · C5 explain <code> (KEEP) · C6 sandbox simulate (KEEP) · C7 reconcile (KEEP) · C8 refund-audit (KEEP) · C9 stuck disbursements (KEEP) · C10 balances sweep (flag) · C11 disbursement verify-completion (KEEP) · C12 whatis ID-prefix (flag) · C13 limits lookup (flag) · C14 external-id guard (flag) · C15 export (CUT — duplicates framework search/sql --csv)

## Survivors and kills

### Survivors
9 survivors, scores 7-9/10, all hand-code. See absorb manifest transcendence table (authoritative copy).

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C4 route policy explainer | Folds into routing command dry-run output | C3 smart routing |
| C10 multi-account balance sweep | Thin fan-out of one endpoint; auth-gated dogfood | C9 stuck detector |
| C12 ID-prefix resolver | Fetch is a thin wrapper over generated get | (generated) get |
| C13 payment-method limits lookup | Overlaps explain/sandbox reference family; not weekly | C6 sandbox simulate |
| C14 X-EXTERNAL-ID uniqueness guard | Sub-helper of signing/routing | C1 snap sign |
| C15 period payment export | Duplicates framework search/sql --csv | (framework) search/sql |
