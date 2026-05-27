# squeezeos-pp-cli

CLI Printing Press generated CLI for [SqueezeOS](https://squeezeos-api.onrender.com) — institutional AI market intelligence.

## Install

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/squeezeos@latest
```

## Quick Start

```bash
squeezeos demo                 # free IWM verdict
squeezeos preview TSLA         # bias + regime (free)
squeezeos status               # health check
squeezeos council NVDA         # AI verdict (paid, needs SQUEEZEOS_TOKEN)
squeezeos scan                 # squeeze scanner (paid)
```

## Auth

Premium endpoints require a JWT from [402Proof](https://four02proof.onrender.com).
Agents pay RLUSD on XRPL — no API keys, no subscriptions.

```bash
export SQUEEZEOS_TOKEN=<token-from-402proof>
```
