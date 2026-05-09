---
name: pp-marginalrevolution
description: "Marginal Revolution from your terminal via the public RSS feed. Trigger phrases: `check marginal revolution`, `search marginal revolution`, `latest MR posts`, `marginal revolution links`, `use marginalrevolution`, `run marginalrevolution`."
author: "Nuri Chang"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - marginalrevolution-pp-cli
    install:
      - kind: go
        bins: [marginalrevolution-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/marginalrevolution/cmd/marginalrevolution-pp-cli
---

# Marginal Revolution - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `marginalrevolution-pp-cli` binary. Verify it is installed before invoking commands:

```bash
marginalrevolution-pp-cli --version
```

If missing, install it:

```bash
npx -y @mvanhorn/printing-press install marginalrevolution --cli-only
```

Fallback direct install:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/marginalrevolution/cmd/marginalrevolution-pp-cli@latest
```

## When to Use This CLI

Use this CLI when you need recent Marginal Revolution posts in structured form, including author/category filtering, recent-feed search, post text, comment counts, and outbound links.

## When Not to Use This CLI

Do not use it for full historical archive search. The command-line-safe surface is the public RSS feed, so `search` is intentionally scoped to posts currently present in that feed.

## Command Reference

- `marginalrevolution-pp-cli latest --limit 10 --json` - list recent posts
- `marginalrevolution-pp-cli latest --author Tyler --category Economics` - filter recent posts
- `marginalrevolution-pp-cli search "ai" --agent` - search current feed title/body/category text
- `marginalrevolution-pp-cli read <url|guid|title>` - read a current-feed post
- `marginalrevolution-pp-cli links --limit 5` - extract outbound links from recent posts
- `marginalrevolution-pp-cli categories` - show category counts in the current feed
- `marginalrevolution-pp-cli authors` - show author counts in the current feed
- `marginalrevolution-pp-cli doctor --json` - verify RSS feed reachability
- `marginalrevolution-pp-cli which "<capability>"` - choose a command from a plain-language request

## Recipes

### Get the latest posts for an agent

```bash
marginalrevolution-pp-cli latest --limit 5 --agent
```

### Find recent posts mentioning AI

```bash
marginalrevolution-pp-cli search "ai" --agent
```

### Extract cited links for follow-up reading

```bash
marginalrevolution-pp-cli links --limit 3 --json
```
