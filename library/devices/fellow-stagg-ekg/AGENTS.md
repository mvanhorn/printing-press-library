# Fellow Stagg EKG Printed CLI Agent Guide

This directory is a generated `fellow-stagg-ekg-pp-cli` printed CLI. It was produced for the Printing Press library, so keep local edits narrow and record any generated-tree customization in `.printing-press-patches/`.

## Local Operating Contract

Use the kettle CLI directly when you want the current runtime truth:

```bash
fellow-stagg-ekg-pp-cli status
fellow-stagg-ekg-pp-cli state
fellow-stagg-ekg-pp-cli settings
```

Prefer explicit host configuration before calling the CLI:

```bash
FELLOW_STAGG_HOST=192.168.1.86 fellow-stagg-ekg-pp-cli status
```

For install, examples, and user-facing guidance, read `README.md` and `SKILL.md`.

## Local Customizations

This tree is generated output. If you change the published code, record the intent in `.printing-press-patches/` so a future print carries the same behavior instead of dropping it.

