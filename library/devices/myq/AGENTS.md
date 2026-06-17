# MyQ Printed CLI Agent Guide

This directory is the published `myq-pp-cli` package.

## Local Contract

Use the CLI directly when you want the current runtime truth:

```bash
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli devices
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli state <serial-number>
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli open <serial-number>
MYQ_USERNAME=you@example.com MYQ_PASSWORD=secret myq-pp-cli close <serial-number>
```

This CLI uses the legacy myQ login flow and does not require a subscription for basic door control.

## Local Customizations

If you change the published code, keep the intent recorded in `.printing-press-patches/` so the next publish can carry the same behavior forward.
