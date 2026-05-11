---
name: pp-cf-domain
description: Cloudflare Registrar domain search, live check, and typed-confirm registration from the terminal.
---

# pp-cf-domain

Use `cf-domain-pp-cli` when you need headless Cloudflare Registrar domain operations.

## Required environment

```bash
export CLOUDFLARE_API_TOKEN=...
export CLOUDFLARE_ACCOUNT_ID=...
```

Required token permission:

- Account → Registrar → Edit, scoped to the specific account

## Commands

```bash
cf-domain-pp-cli doctor
cf-domain-pp-cli domain-search --query <name> --limit 20
cf-domain-pp-cli domain-check --domain <domain.tld>
cf-domain-pp-cli domain-register --domain <domain.tld> --confirm-domain <domain.tld>
```

## Safety gate

Never run `domain-register` until the user confirms the exact domain and fresh returned price from `domain-check`.

The CLI also enforces a typed confirmation gate: `--confirm-domain` must exactly match `--domain`.
