# Running Race Results Printed CLI Agent Guide

This directory is a `running-race-results-pp-cli` printed CLI package. It was ported into the Printing Press library from `github.com/jiahongc/running-race-results`, so keep local edits narrow and record any behavior-changing customization under `.printing-press-patches/`.

## Local Operating Contract

Start by asking the CLI for runtime truth:

```bash
running-race-results-pp-cli agent-context --pretty
running-race-results-pp-cli --help
```

Inspect command help before invoking a provider-backed lookup:

```bash
running-race-results-pp-cli lookup --help
running-race-results-pp-cli athlete --help
```

This CLI is read-only. It looks up publicly published race results and does not create, update, or delete remote data.

## Auth

Most commands do not require credentials. `ATHLINKS_TOKEN` is optional and is only needed for `athlete --me` or as a fallback if an Athlinks endpoint returns 401/403 for anonymous requests.

## Local Customizations

If you modify this CLI beyond the published package, record each change under `.printing-press-patches/` at this CLI root. One file per patch keeps future reprints from silently dropping local intent.

Minimum shape:

```json
{
  "schema_version": 2,
  "id": "short-identifier",
  "applied_at": "YYYY-MM-DD",
  "base_run_id": "<copy from .printing-press.json>",
  "base_printing_press_version": "<copy from .printing-press.json>",
  "summary": "What changed.",
  "reason": "Why this customization was needed.",
  "files": ["internal/cli/example.go"],
  "validated_outcome": "Focused check that proved the change."
}
```
