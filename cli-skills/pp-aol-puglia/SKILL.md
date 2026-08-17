---
name: pp-aol-puglia
description: "Printing Press CLI for Aol Puglia. API per l'Albo Pretorio Online della Sanità Pugliese."
author: "aborruso"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - aol-puglia-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/aol-puglia/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Aol Puglia — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `aol-puglia-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install aol-puglia --cli-only
   ```
2. Verify: `aol-puglia-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

API per l'Albo Pretorio Online della Sanità Pugliese.
Accesso pubblico (bandi, concorsi, delibere, determine) tramite OAuth2 client_credentials.
13 aziende sanitarie pugliesi disponibili.

## Known Data Limitations

Questa API espone i dati così come pubblicati dalla piattaforma regionale (`sanita.puglia.it/AlboOnline`). Verificato al 2026-07-18: 10 delle 13 aziende sono aggiornate correntemente, ma 3 risultano ferme:

| Azienda | Ultimo aggiornamento rilevato |
|---|---|
| ASL Taranto | ~agosto/settembre 2018 |
| ARES | ~inizio 2018 |
| Sanitaservice ASL BR | ~inizio 2024 |

Per queste 3 aziende non fidarsi del risultato di `atti search` come "atti recenti": la fonte non riceve più (o riceve con forte ritardo) i nuovi atti di quegli enti. Avvisare l'utente se la ricerca riguarda una di queste aziende ed è rilevante la recenza dei dati.

## Command Reference

**atti** — Manage atti

- `aol-puglia-pp-cli atti download-allegato` — Restituisce il contenuto dell'allegato come base64 con il nome originale del file.
- `aol-puglia-pp-cli atti get-atto` — Restituisce il dettaglio completo di un singolo atto inclusa la lista allegati.
- `aol-puglia-pp-cli atti get-trasparenza` — Restituisce le sezioni di trasparenza disponibili per l'azienda.
- `aol-puglia-pp-cli atti search` — Ricerca paginata degli atti pubblicati: bando, concorso, delibera, determina (la disponibilità per tipo varia per azienda).

**config** — Manage config

- `aol-puglia-pp-cli config get-configurazione-item` — Restituisce la configurazione di durata pubblicazione e proroga per ogni tipo di atto dell'azienda.
- `aol-puglia-pp-cli config list-proponenti` — Restituisce tutti i proponenti configurati per l'azienda e il tipo di documento.
- `aol-puglia-pp-cli config list-proponenti-attivi` — Come getProponenti ma filtra solo i proponenti con attivo=true.

**fileexport** — Manage fileexport

- `aol-puglia-pp-cli fileexport` — Esporta i risultati di una ricerca come file CSV.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
aol-puglia-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `aol-puglia-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
aol-puglia-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `AOL_PUGLIA_BEARER_AUTH` as an environment variable.

Run `aol-puglia-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  aol-puglia-pp-cli atti search --azienda example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
aol-puglia-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
aol-puglia-pp-cli feedback --stdin < notes.txt
aol-puglia-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/aol-puglia-pp-cli/feedback.jsonl`. They are never POSTed unless `AOL_PUGLIA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AOL_PUGLIA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
aol-puglia-pp-cli profile save briefing --json
aol-puglia-pp-cli --profile briefing atti search --azienda example-value
aol-puglia-pp-cli profile list --json
aol-puglia-pp-cli profile show briefing
aol-puglia-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `aol-puglia-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add aol-puglia-pp-mcp -- aol-puglia-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which aol-puglia-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   aol-puglia-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `aol-puglia-pp-cli <command> --help`.
