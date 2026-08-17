---
name: pp-open-library
description: "Use Open Library source-backed recipes for book search, ISBN edition lookup, author works, work metadata, editions, and subject browsing. Trigger phrases: Open Library, book metadata, ISBN lookup, author bibliography, editions, subject books, open-library-pp-cli."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - open-library-pp-cli
    install:
      - kind: go
        bins: [open-library-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/open-library/cmd/open-library-pp-cli
---

# Open Library - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `open-library-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install open-library --cli-only
   ```
2. Verify: `open-library-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/open-library/cmd/open-library-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When To Use

Use `open-library-pp-cli` when an agent needs source-backed Open Library metadata:

- Search for book/work candidates by title or keyword phrase.
- Resolve ISBNs to Open Library edition metadata.
- Find an author record and fetch a bounded sample of their works.
- Fetch canonical work metadata by Open Library work ID.
- List bounded editions for a known work.
- Browse subject works and optional subject facets.

## When Not To Use

- Do not use it for borrowing, waitlists, patron data, account actions, or catalog edits.
- Do not use it for bulk harvesting; use Open Library data dumps for bulk access.
- Do not scrape Open Library HTML pages.
- Do not treat subject facets as stable; Open Library documents the Subjects API as experimental.

## Setup

No API key is required.

The module declares `go 1.26.4` so environments with Go toolchain auto-download disabled should use Go 1.26.4 or newer for direct installs.

For regular or frequent use, configure a request identity:

```bash
export OPEN_LIBRARY_USER_AGENT="my-research-tool"
export OPEN_LIBRARY_CONTACT_EMAIL="you@example.org"
```

Without `OPEN_LIBRARY_CONTACT_EMAIL`, keep requests near Open Library's documented non-identified rate posture of 1 request per second.

## Recipes

### Book Search

```bash
open-library-pp-cli book "Designing Data-Intensive Applications" --limit 5 --agent
```

Use this to seed a bibliography with compact work candidates, author names, first publish year, edition count, ISBN hints, and source URLs.

### ISBN Lookup

```bash
open-library-pp-cli isbn 9780131103627 --agent
```

Use this when the workflow has an ISBN and needs edition-level metadata.

### Author Works

```bash
open-library-pp-cli author "Ursula K. Le Guin" --limit 5 --agent
```

Use this to identify an author and fetch a bounded sample of works.

### Work Metadata

```bash
open-library-pp-cli work OL45804W --agent
```

Use this when a workflow already has an Open Library work ID.

### Editions

```bash
open-library-pp-cli editions OL45804W --limit 10 --agent
```

Use this to compare editions for a known work without bulk-fetching every record.

### Subjects

```bash
open-library-pp-cli subjects "distributed systems" --details --limit 10 --agent
```

Use this for adjacent-book discovery. Keep the experimental Subjects API caveat with downstream notes.

### Source Coverage

```bash
open-library-pp-cli sources --agent
open-library-pp-cli doctor --agent
```

Use these before a larger workflow to confirm rate-limit posture, optional request identity, source coverage, and non-goals.
