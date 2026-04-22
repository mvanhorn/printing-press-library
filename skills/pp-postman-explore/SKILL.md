---
name: pp-postman-explore
description: "Search and browse the public Postman API Network from the terminal. Find APIs by name, topic, or category; look up publisher teams; list network entities. Use when the user is researching an unfamiliar API and wants to see if it has a Postman collection, find alternative APIs in the same category, look up a publisher team on Postman, or discover which APIs are on the Postman Network. Read-only; no account or API key required."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["postman-explore-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-cli@latest","bins":["postman-explore-pp-cli"],"label":"Install via go install"}]}}'
---

# Postman Explore - Printing Press CLI

Search and browse the Postman API Network - the public directory of APIs, workspaces, and publisher teams indexed by Postman. The CLI is read-only and needs no API key; it wraps Postman's public discovery surface.

## When to Use This CLI

Reach for this when the user wants:

- find a Postman collection or workspace for an unfamiliar API
- discover alternative APIs in a category (e.g. "other email-verification APIs")
- look up a specific API or publisher team by name
- browse categories of APIs on the Postman Network
- check if a vendor has an official Postman collection before writing one by hand

Skip it when the user wants to manage private Postman workspaces, collections, or environments; those require the Postman API with an API key, which this CLI does not wrap.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** -> show `postman-explore-pp-cli --help`
2. **Starts with `install`** -> ends with `mcp` -> MCP installation; otherwise -> CLI installation
3. **Anything else** -> Direct Use (map to the best command and run it)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+).
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-cli@latest
   ```

   If `@latest` installs a stale build (Go module proxy cache lag):
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/cmd/postman-explore-pp-cli@main
   ```
3. Verify: `postman-explore-pp-cli --version`.
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. No auth required; the discovery surface is public.
6. Verify: `postman-explore-pp-cli doctor` reports CLI health.

## Direct Use

1. Check installed: `which postman-explore-pp-cli`. If missing, offer CLI installation.
2. Discover commands: `postman-explore-pp-cli --help`; drill into `postman-explore-pp-cli <cmd> --help`.
3. Execute with `--agent` for structured output:
   ```bash
   postman-explore-pp-cli <command> [args] --agent
   ```

## Notable Commands

| Command | What it does |
|---------|--------------|
| `search <query>` | Full-text search across the Postman Network |
| `search-all` | Broader search surface (published + community) |
| `category` | Browse or look up an API category |
| `team` | Publisher teams on the Network |
| `networkentity` | Inspect a specific API or workspace entity |
| `sync` | Populate the local SQLite store for offline queries |

Run any command with `--help` for full flag documentation.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields, with dotted-path support (see below)
- **Previewable** — `--dry-run` shows the request without sending
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Non-interactive** — never prompts, every input is a flag


### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
postman-explore-pp-cli <command> --agent --select id,name
postman-explore-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.


## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (Postman upstream) |
| 7 | Rate limited |
