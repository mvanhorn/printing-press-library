---
name: pp-anac-pl
description: "Cerca bandi, esiti e avvisi di gara della piattaforma ANAC dalla riga di comando, con dettaglio JSON, cronologia Trigger phrases: `cerca un bando ANAC`, `trova esiti di gara`, `dettaglio avviso pubblicita legale`, `usa anac-pl`, `ricerca appalti ANAC`."
author: "aborruso"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - anac-pl-pp-cli
    install:
      - kind: go
        bins: [anac-pl-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/anac-pl/cmd/anac-pl-pp-cli
---

# ANAC Pubblicità a Valore Legale — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `anac-pl-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install anac-pl --cli-only
   ```
2. Verify: `anac-pl-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/anac-pl/cmd/anac-pl-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Usa questo CLI per cercare e analizzare programmaticamente bandi, esiti e avvisi pubblicati da stazioni appaltanti italiane sulla piattaforma ANAC. Ideale per monitoraggio appalti, due diligence e analisi dati con CSV/SQL.

## Anti-triggers

Do not use this CLI for:
- Non usare per la Banca Dati Nazionale Contratti Pubblici (BDNCP) o l'anagrafe CIG completa.
- Non usare per inviare o pubblicare avvisi: l'API e' di sola lettura.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Stato locale che si accumula
- **`sync`** — Scarica e conserva gli avvisi in un database SQLite locale per analisi offline.

  _Permette analisi ripetute e aggregazioni che l'API paginata non offre._

  ```bash
  anac-pl-pp-cli sync --resources avvisi --param keywords=microsoft
  ```
- **`search-local`** — Cerca tra gli avvisi gia' sincronizzati in locale, senza rete.

  _Risposte istantanee e componibili con jq/SQL._

  ```bash
  anac-pl-pp-cli search-local microsoft
  ```
- **`export`** — Esporta gli avvisi sincronizzati in CSV o JSON per fogli di calcolo e pipeline dati.

  _Porta i dati ANAC direttamente in strumenti di analisi._

  ```bash
  anac-pl-pp-cli export avvisi
  ```

## Command Reference

**avvisi** — Ricerca e consultazione di bandi, esiti e avvisi pubblicati sulla Piattaforma di Pubblicità a Valore Legale ANAC

- `anac-pl-pp-cli avvisi cronologia` — Cronologia delle versioni/rettifiche di un avviso nel tempo
- `anac-pl-pp-cli avvisi get` — Dettaglio completo di un avviso/esito in formato JSON, incluse sezioni e committente
- `anac-pl-pp-cli avvisi search` — Ricerca full-text di avvisi (bandi, esiti, altri avvisi) con ranking di rilevanza e filtri

**news** — Avvisi e comunicazioni della piattaforma

- `anac-pl-pp-cli news` — Ultime news pubblicate sulla piattaforma

**tipologie** — Tassonomia delle tipologie di avviso (categorie, tipologie e codici scheda)

- `anac-pl-pp-cli tipologie` — Mappa di categorie, tipologie e codici scheda usati per filtrare la ricerca
- `anac-pl-pp-cli tipologie schede` — Describes notice scheda codes (AD3, A1_29, P1_16...) from ANAC's codiceScheda.json, offline; unknown code exits 3

**cig** — CIG tools

- `anac-pl-pp-cli cig check <cig>...` — Offline structure and check-digit validation (Simog, Simog v2/PCP, SmartCIG) with ANAC's published algorithm; a wrong check digit is a result (exit 0, check `valido` per code, `jq -e 'all(.valido)'` in scripts); an argument that is not CIG-shaped (length ≠ 10, bad first char) is a usage error, exit 2


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
anac-pl-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

Matching is word-based, not semantic: a query scores on words that appear in a command name (`cerca`, `avvisi`, `cpv`, `affidamenti`, `tipologie`, ...) or in its description, ignoring common Italian and English filler words. Use the words of the domain (`bandi`, `cpv`, `cronologia`, `affidamenti`) rather than paraphrases: `which "trova bandi recenti"` resolves to `avvisi search`, `which "chi ha vinto l'appalto"` returns exit 2.

## Search caveats

These come from how the ANAC search service behaves, not from the CLI. Check them before trusting a result set.

- **Terms are OR-ed by default.** `--query` words are matched in OR and ranked by relevance: the top results are usually on topic, lower ones may contain a single term. Quotes and `+` are silently ignored; `AND` is searched as a word and adds noise. For an exact phrase use `cerca --mode esatta` or `avvisi search --fuzzy=false`: words must be adjacent and in order, and a tipologia is required (the service answers HTTP 500 without one, so the CLI refuses first). Example: `cerca -q "data visualization" --mode esatta -t affidamenti-diretti`.
- **Sorting is ignored with `--query`.** `--sort-field`/`--sort-dir` only work without free text; the portal's own "Ordina per" has no effect either. To get recent notices, filter with `--published-from GG/MM/AAAA`. There is no filter on the deadline (`dataScadenza`).
- **`--scheda` takes one tipologia**, as a template number or name from `tipologie list` (`4` or `bandi`, `7` or `esiti`). With comma-separated values the full-text search silently uses only the first, and notice codes such as `AD3` or `A1_29` make the service answer HTTP 500: the CLI rejects both.
- **`cerca --cpv` is a text match**, not a code filter: use `cerca-avanzata --cpv` to select by code. That flag also takes the official form with the check digit (`30213000-5`); the service ignores the check digit, and on a prefix it returns nothing, so the CLI refuses `302-5`.
- **There is no page number.** The service paginates by token and silently ignores a page parameter, always returning the first page. Use `cerca --pages N` (or `affidamenti --pages N`) to walk N pages of `--size` results and get them merged, deduplicated by `idAvviso`; `avvisi search --all` follows continuation tokens up to 100 pages by default. Raise the safety limit with `--max-pages N` for broad searches. Reaching the limit returns an error rather than incomplete results. Without `--all`, `--size` controls the first page.
- **The declared result count is an estimate** that changes across pages and identical calls.
- **`titolo` is often `null`** (about 60% of notices, mostly AD3 and A2_* schede). `descrizione` is always set: read the notice subject from `templates[].template.metadata.descrizione`.
- **A mistyped CIG returns unrelated notices, not an error.** Run `cig check <CIG>` before searching by CIG; `cerca --query` and `avvisi search --keywords` warn on stderr when a CIG-shaped term fails the check digit.
- **Obscured notices may be missing.** Results carry `oscurato`/`noteOscuramento`; ANAC's guide says obscured notices can be excluded from search, and that the validation checks that trigger obscuring are currently disabled (always `false` in September 2026 samples).

## Recipes

### Esiti recenti per parola chiave

```bash
anac-pl-pp-cli cerca -q 'servizi informatici' -t esiti --published-from 01/06/2026 --size 20
```

Filtra gli esiti di gara per oggetto. Con testo libero l'ordine è per rilevanza, quindi a limitarli ai più recenti è il filtro sulla data.

### Dettaglio JSON di un esito

```bash
anac-pl-pp-cli avvisi get c5bfcc8d-ebed-4b6b-ab5f-661d78fa88e2 --json
```

Recupera il JSON completo del detail page.

### Estrai solo campi chiave

```bash
anac-pl-pp-cli avvisi search --query microsoft --agent --select idAvviso,codiceScheda,dataPubblicazione,score
```

Output compatto per agenti su risposte voluminose.

## Auth Setup

No authentication required.

Run `anac-pl-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  anac-pl-pp-cli avvisi get mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
anac-pl-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
anac-pl-pp-cli feedback --stdin < notes.txt
anac-pl-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/anac-pl-pp-cli/feedback.jsonl`. They are never POSTed unless `ANAC_PL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ANAC_PL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
anac-pl-pp-cli profile save briefing --json
anac-pl-pp-cli --profile briefing avvisi get mock-value
anac-pl-pp-cli profile list --json
anac-pl-pp-cli profile show briefing
anac-pl-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (anche: `search-local` senza corrispondenze) |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `anac-pl-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/anac-pl/cmd/anac-pl-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add anac-pl-pp-mcp -- anac-pl-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which anac-pl-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   anac-pl-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `anac-pl-pp-cli <command> --help`.
