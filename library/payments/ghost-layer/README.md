# ghost-layer-pp-cli

CLI Printing Press generated CLI for [Ghost Layer](https://ghost-layer.onrender.com) — dual-chain XRPL/Base toll gateway.

## Install

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/ghost-layer@latest
```

## Quick Start

```bash
ghost-layer status
ghost-layer x402 catalog
ghost-layer x402 quote --product routing.telemetry --wallet rXXX
ghost-layer x402 dispense routing.telemetry
ghost-layer agent rXXX
```
