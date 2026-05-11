# Cloudflare Registrar domain CLI

Small headless CLI for [Cloudflare Registrar](https://developers.cloudflare.com/registrar/) domain workflows.

It wraps the Cloudflare Registrar API endpoints documented in the [Cloudflare API reference](https://developers.cloudflare.com/api/resources/registrar/) for headless agent and script workflows:

- Search domain ideas
- Check exact domain availability/pricing
- Register a domain, gated by typed confirmation

## Install

```bash
go install ./cmd/cf-domain-pp-cli
```

From the Printing Press library checkout:

```bash
cd library/cloud/cf-domain
go install ./cmd/cf-domain-pp-cli
```

## Auth

Required env:

```bash
export CLOUDFLARE_API_TOKEN=...
export CLOUDFLARE_ACCOUNT_ID=...
```

Create the token using Cloudflare's [API token guide](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/).

Minimum Cloudflare token permission:

- Account → Registrar → Edit, scoped to the specific account

Optional:

- Account → Account Settings → Read, only if external tooling needs account discovery

## Commands

```bash
cf-domain-pp-cli doctor
cf-domain-pp-cli domain-search --query mybrand --limit 20
cf-domain-pp-cli domain-check --domain mybrand.dev
cf-domain-pp-cli domain-register --domain mybrand.dev --confirm-domain mybrand.dev
```

## Safety

`domain-register` refuses to run unless `--confirm-domain` exactly matches `--domain`.

Before registering, always run `domain-check` immediately before purchase and confirm the exact domain, availability, price, renewal price, and currency returned by Cloudflare.

If Cloudflare returns an async/pending registration response, do not retry blindly.
