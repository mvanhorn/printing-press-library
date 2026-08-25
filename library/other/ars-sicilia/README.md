# ARS Sicilia CLI

**L'unica CLI per il portale dell'Assemblea Regionale Siciliana: cerca, sincronizza in locale e interroga tutti i 12 archivi documentali con SQL, FTS e MCP.**

Sostituisce le 12 maschere JSP del portale ufficiale con una CLI agent-native. Sync in SQLite locale per query SQL, ricerca full-text cross-archivio, e novel commands come `ddl iter` (timeline completa di un disegno di legge) e `deputato profilo` (tutta l'attività di un parlamentare in un'unica chiamata).

Learn more at [ARS Sicilia](https://dati.ars.sicilia.it).

Printed by [@aborruso](https://github.com/aborruso) (aborruso).

## Install

The recommended path installs both the `ars-sicilia-pp-cli` binary and the `pp-ars-sicilia` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ars-sicilia
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ars-sicilia --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ars-sicilia --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ars-sicilia --agent claude-code
npx -y @mvanhorn/printing-press-library install ars-sicilia --agent claude-code --agent codex
```

### Without Node

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/cmd/ars-sicilia-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ars-sicilia-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ars-sicilia --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ars-sicilia --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-ars-sicilia skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-ars-sicilia. The skill defines how its required CLI can be installed.
```

## Use as an MCP server

Every command of this CLI is also an MCP tool, served over stdio by the `ars-sicilia-pp-mcp` binary. Any MCP client can run it — Claude Desktop, Claude Code, Cursor, VS Code (Copilot), Windsurf, Zed, Codex, Gemini CLI, and the rest of the [MCP client directory](https://modelcontextprotocol.io/clients). Asking in plain language ("quali ddl sulla sanità nella XVIII legislatura?") is usually easier than composing flags, so this is the recommended path for non-technical users.

### 1. Install the server binary

The `npx` installer and the pre-built binaries above ship the **CLI** only. Install the MCP binary with Go (1.26.6 or newer, matching this module's `go` directive) — same command on Linux, macOS, and Windows:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/cmd/ars-sicilia-pp-mcp@latest
```

It lands in `$(go env GOPATH)/bin` (or `go env GOBIN`); make sure that directory is on your `PATH`.

### 2. Point your client at it

Most clients accept the same stdio server entry:

```json
{
  "mcpServers": {
    "ars-sicilia": {
      "command": "ars-sicilia-pp-mcp"
    }
  }
}
```

What changes per client is where that entry goes — Claude Desktop's `claude_desktop_config.json`, `.cursor/mcp.json`, VS Code's `mcp.json`, Gemini CLI's `~/.gemini/settings.json` — and occasionally its syntax: Codex takes the same command in TOML (`[mcp_servers.ars-sicilia]` in `config.toml`). See [connect local MCP servers](https://modelcontextprotocol.io/docs/develop/connect-local-servers), or your client's own docs: [Claude Code](https://docs.claude.com/en/docs/claude-code/mcp), [Cursor](https://docs.cursor.com/context/model-context-protocol), [VS Code](https://code.visualstudio.com/docs/copilot/chat/mcp-servers). If your client does not inherit the shell `PATH`, put the absolute path of the binary in `command`.

Claude Code needs no JSON at all:

```bash
claude mcp add ars-sicilia -- ars-sicilia-pp-mcp
```

No credentials to configure: the ARS portal is public.

## Authentication

Nessuna credenziale richiesta: il portale ARS è pubblico. La sessione `JSESSIONID` per la ricerca è gestita automaticamente in modo trasparente dal client.

## Quick Start

```bash
# Verifica raggiungibilità del portale e stato del database locale.
ars-sicilia-pp-cli doctor

# Sincronizza in locale leggi e DDL degli ultimi 30 giorni in SQLite.
ars-sicilia-pp-cli sync --resources leggi,ddl --max-pages 0

# Cerca i DDL della XVIII legislatura presentati nel 2024.
ars-sicilia-pp-cli ddl cerca --anno 2024 --legisl 18 --json

# Ricerca full-text cross-archivio sui documenti già sincronizzati.
ars-sicilia-pp-cli search "bilancio sanitario" --limit 20

# Timeline completa del DDL 1153 della XVIII legislatura.
ars-sicilia-pp-cli ddl iter 18 1153 --json

# Tutta l'attività parlamentare di un deputato in un'unica chiamata.
ars-sicilia-pp-cli deputato profilo "Abbate Ignazio" --json --select tipo,data,titolo

# I gruppi parlamentari della XVIII legislatura (anagrafica dal sito istituzionale).
ars-sicilia-pp-cli gruppi elenco --legisl 18 --json

# Composizione di un gruppo: cariche, collegio di elezione, email e scheda.
ars-sicilia-pp-cli gruppi get XVIII-misto --json

# In quale gruppo sta un deputato, con ruolo e collegio.
ars-sicilia-pp-cli gruppi elenco --legisl 18 --deputato "Cracolici" --json

```

## Known Gaps

- **HTTP error exit codes**: Non-429 HTTP errors from the Icaro portal (404, 5xx) exit with code 1 rather than a typed exit code such as 3 for not-found. Rate-limit responses (HTTP 429) correctly return exit 7. Scripts that branch on specific exit codes should use `ars-sicilia-pp-cli doctor` to check connectivity first.
- **Result `url` and `doc_id` are session-scoped, not citable** ⚠️: on the Icaro archives `icaDocId` is the row's position in the *current session's* short list, not the document's identity. Run a different query and the same `icaDocId` opens a different act (`(18.LEGISL E 738.NUMDDL)` → `icaDocId=1` is `docno(9037)`, not the one you saw before); outside a session the URL answers 302. Do not store or cite them. `get` also returns `docno` — the portal's own stable document number — and `permalink`, which reopens exactly that document in a fresh session: those are the ones to keep. The short list does not carry `docno` (its markup only has `showDoc(N)`), so search rows cannot expose it without one detail fetch per row.
- **One number can match more than one document** ℹ️: the portal keeps distinct documents under the same legislature+number, usually successive versions of the same file. Ddl 6030 has two — `docno(9513)` with the bill text and the iter up to 4 Aug 2026, and `docno(9390)`, scheda-only and stuck at 14 Jul — identical in every list field, title and date included. `cerca` flags it with a hint and `get` says which one it opened (with its `docno`) instead of silently taking the first. The `cerca` hint counts inside the window it downloaded, so a tight `--limit` can hide the second row and silence it — that case is covered by `troncato`/the truncation hint, which always speaks. Neither hint fires on `leggi` (indexed per article) or `resoconti` (indexed per agenda item), where repeated numbers are the norm.
- **`legge cronologia` needs `--anno` when the number repeats** ⚠️: the same law number recurs in different years of one legislature — the XVIII has two L.R. 26 (7 Oct 2024, 10 Jun 2025). The archive returns one row and the command takes it, so without `--anno` you can get a perfectly coherent timeline for the wrong act. A stderr hint names the law it picked (`uso la L.R. 26 promulgata il 7.10.2024`): check that date, or pin `--anno`. The report's root carries `ddl_originari`, the numbers of the bills the law came from (more than one when they were merged) — the direct handle for `ddl iter`. In the other direction, when a `ddl iter` timeline ends at the Assembly vote with no Gazzetta event, the report says so in `note`: the two archives publish at different lags, so a freshly approved law may simply not be indexed yet.
- **`legge cronologia` date filtering**: The sommari search finds committee meetings that mention the law number in free text without a date ceiling. A committee meeting held after the law's promulgation date may appear in the timeline if it references the same number. Filter results by the `data` field when you need only pre-promulgation events.
- **`--csv` on empty results**: when a command (e.g. `analytics --csv`) produces an empty result set, the CSV output is the JSON literal `[]` instead of an empty/header-only CSV. Piping that to a `.csv` file yields malformed content. Use `--json` for empty/unsynced data until this is fixed upstream.
- **`search` JSON shape**: `search --json` returns an object `{ "meta": {...}, "results": [...] }`, not a top-level array. When piping to `jq`, select `.results` (e.g. `search "x" --json | jq '.results[]'`). Status lines ("no search endpoint…", sync hints) go to stderr, so `2>/dev/null` keeps stdout clean.
- **Local-store commands need a sync first**: `search`, `analytics` and `sync stale` read the local SQLite store. On a fresh install run `ars-sicilia-pp-cli sync --full` first; until then they return empty results (with a sync hint on stderr). All other commands (`*/cerca`, `*/get`, `ddl iter`, `deputato profilo`, `legge cronologia`, `commissione dossier`, `gruppi elenco`, `gruppi get`) query the portal live and need no sync.
- **`analytics --group-by cofirmatari` and `ddl drift` need a deep sync** ⚠️: a plain sync stores only the list-page fields (`data`, `numero`, `title`, `url`, …). The signatories (`firmatari`) and per-bill iter status (`iter`) live only inside each document `body`, so run **`ars-sicilia-pp-cli sync --resources ddl --deep`** to extract them into the store — then `analytics --group-by cofirmatari --type ddl` works, and `ddl drift` compares the iter state across two deep syncs. The deep pass fetches one detail page per ddl (~1 extra request each), so a full legislature takes a few minutes; a normal sync stays fast.
- **`gruppi` reads a second site, and it is HTML** ℹ️: the group roster is not in the documentary engine — on `dati.ars.sicilia.it` a group is only a string next to a signature. `gruppi elenco` and `gruppi get` read `www.ars.sicilia.it`, which has no API (`/jsonapi`, `/rest/session/token` and `/sitemap.xml` all answer 404), so the selectors are worth exactly what the Icaro `Columns` are worth: verified against the live pages and pinned by fixtures in `internal/wwwclient/testdata`. A restyling breaks them silently, which is why an empty extraction is an error and not a result — the portal publishes no empty groups, so zero rows means the selector moved. Two consequences: `sync coverage` measures `dati.` only and says nothing about this source, and the slugs are not derivable from the name (`XVIII-fratelli-ditalia`, `XVIII-prima-litalia-lega-salvini-premier`) — take them from `gruppi elenco`, do not guess them.
- **`analytics --group-by cofirme` runs live (no sync)** ℹ️: how many acts each deputy **co-signed** — a different question from `--group-by cofirmatari`, which counts *pairs* of co-signers and still needs the deep sync (pairs live inside each document). The count comes from the portal's own search engine, asked in ISIS: `(18.LEGISL E ((Nome.FIRMAT) NOT (1 ADJ Nome).FIRMAT))` — appears among the signatories but not in first position. One request per deputy (~66 for a legislature, ~80 s), so it needs `--legisl`; the names in the exact form the portal indexes come from the built-in `firmatari` table (1110 entries, legislatures X–XVIII). Works on every archive with a signatory field (`ddl`, `interrogazioni`, `interpellanze`, `mozioni`, `odg`, `risoluzioni`). Cross-checked against the counters published on www.ars.sicilia.it: Cracolici 302 and Catanzaro 306 co-signed bills in the XVIII, same to the single act. Deputies the portal did not answer for are named on stderr, not counted as zero.
- **`analytics --group-by oratore` runs live (no sync)** ℹ️: the speaker ranking is built by querying the `/bd/resoconti` backend once per speaker of the legislature (≈90 requests, ~1 min), so it needs `--legisl` (without it the ~1000 all-time speakers would be too many requests) and does not read the local store. Example: `analytics --type resoconti --group-by oratore --legisl 18`. If the backend drops some of those requests, the ranking is still published with the speakers that answered and a `nota:` on stderr names the ones left unmeasured — they are *not* zero-intervention speakers.
- **The `/bd/` backend truncates large responses — narrow the search** ⚠️: on `sommari`, `resoconti` and `convocazioni` the portal intermittently delivers a half-cut body (HTTP 200, regular headers, content stopping mid-page). It is not a timeout (cut responses arrive in 0.2s) and not protocol-related (same on HTTP/2 and HTTP/1.1): it scales with response size. Measured on `sommari`: a single-sitting search (24 KB) succeeded 8/8, the same search with no filters (44 KB) 0/8. The CLI retries every read up to 3 times and, when it gives up, reports a backend failure — never an absence of data: `il backend /bd/ non ha risposto` does not mean the document does not exist. To make a search reliable, narrow it: `--numero` (single sitting; available on `resoconti` and `commissioni sommari` — `convocazioni` has no sitting number in the portal form), then `--anno`, then `--commissione`. On failure the CLI names the missing filter.
- **ISIS-only filters do not exist on the migrated archives** ℹ️: `sommari`, `resoconti` and `convocazioni` are served by the portal's `/bd/` backend, which has a fixed form instead of an ISIS query string. `--isis-query`, `--escludi` and `--frase` (plus `--presidente` on `commissioni sommari`) have no equivalent there, so **those flags are not registered on those commands**: `--help` no longer advertises a criterion that could only ever fail, and typing one gets `unknown flag`. They used to be accepted and then rejected at runtime, which cost a round-trip to discover — for an agent reading the MCP schema, a wasted call. Every filter those commands do expose works: `--legisl`, `--anno`, `--data`, `--numero`, `--testo` (the `/bd/` full-text field, now available on `commissioni convocazioni` too), `--oratore` on resoconti, `--argomento` on sommari (an alias of `--testo`), `--commissione`/`--codcom` on sommari and convocazioni. ISIS filters remain available on all the other archives (`ddl`, `leggi`, `interrogazioni`, …).
- **Six-digit `AAMMGG` dates resolve their century from the archive, and stop at 2046** ℹ️: `--data` also accepts the portal's native six-digit form, which carries no century. On the `/bd/` archives `47`–`99` reads as 1900s and `00`–`46` as 2000s, because the oldest document those archives serve is the inaugural sitting of **25 May 1947** (nothing exists for 1946 — the ARS begins there). So `--data 510412` is 12 April 1951; it used to be read as 2051 and returned `[]` on a sitting that exists, and with it every date between 1947 and 1999 written this way. Dates from 2047 on cannot be expressed in six digits — write them in full, which is never ambiguous.
- **A malformed `--data` is an error, and only on the `/bd/` archives** ⚠️: on `sommari`, `resoconti` and `convocazioni`, a value the date parser cannot read (`2025-01-01:garbage`, `garbage`) or a date that does not exist (`2025-13-45`, `2025-02-30`) now fails with exit 2 and a message naming the accepted forms. It used to drop the filter entirely and return the archive from the beginning — `resoconti cerca --data 2025-01-01:garbage` answered with sittings from **1951** — presented as a valid answer. On the Icaro archives the same input is passed through to ISIS and simply matches nothing (`mozioni cerca --data garbage` → `[]`), so the wrong-results failure was `/bd/`-only.
- **`--data` across calendar years costs one request per year** ℹ️: the `/bd/` form filters by a single year, so `--data 2024-11-01:2026-02-28` queries 2026, 2025 and 2024 in turn (most recent first) and filters the exact days client-side. With a small `--limit` you get the most recent records of the range and `troncato`/truncation flags report that earlier years were left unread.
- **`leggi cerca` returns one row per law, not per article** ℹ️: archive 201 is indexed **per article**, so a twenty-article law used to fill twenty rows — and `--limit 10` was spent on the articles of the first law, answering "which laws passed in 2025?" with a single law while there were dozens. The command now aggregates by law (`articoli_trovati` counts the articles this search matched, not the law's total) and `--limit` counts laws. Use `--articoli` for the raw per-article rows, which is what you want with `--testo` when you need to know *which* article mentions a term. Pagination stops on the unit you asked for: it keeps reading pages until the requested laws are collected, rather than guessing a row budget up front. That guess (10 rows per law) is what made `leggi cerca --legisl 18 --anno 2025` answer with 4 laws out of the year's 31 — the first laws of a year are the budget ones, ~25 article-rows each, and they ate the window. Short laws now cost fewer requests than before, long ones cost more (the portal allows 2 requests/second, so a full default page of 10 laws takes ~20 s on a budget-heavy year). A safety ceiling still caps the rows read; when it cuts in before the requested laws are collected, the CLI says so on stderr instead of returning a short list silently. Reaching `--limit` is reported too: `--anno 2026` at the default 10 used to return 10 of the year's 14 laws with `troncato: false`, asserting a completeness nobody had checked. That case now sets `troncato: true` and tells you to raise `--limit`. The portal's delivery order is not chronological, so a cut list is not even "the most recent ones".
- **Look an act up by its number with `--numero`, never with `--testo`** ⚠️: `--numero` is field-qualified (`NUMORD`/`NUMDDL`/`LEGNUM`) and returns the act itself; it is available on every `cerca` (`ddl`, `leggi`, `odg`, `interrogazioni`, `interpellanze`, `mozioni`, `risoluzioni`). Passing the number as free text instead matches every document that *mentions* it, newest first, so the act you want can sink past the default `--limit`: `mozioni cerca --testo "143"` buries mozione 143 at position 17 of 19, while `--numero 143` returns it alone.
- **`--testo` is AND over the whole document; `--frase` matches a phrase** ℹ️: `--testo "aree idonee"` builds `(aree E idonee)`, so both words must appear *somewhere* in the document — on a text as long as a bill that also matches acts with one word in article 3 and the other in article 40 (peschicoltura, coworking). `--frase "aree idonee"` builds `(aree adj idonee)`: adjacent, in the given order, which surfaces the acts that actually legislate on the topic (ddl 803, 726). A single word passes through unchanged, and a value already containing operators or parentheses is left verbatim. The flag does not exist on `resoconti`, `sommari` and `convocazioni` (the `/bd/` backend takes no ISIS expression) — there the text search is `--testo`.
- **Full-text results are not ranked by relevance, and a short list is not proof of absence** ⚠️: the portal returns matches in its own order (roughly newest first), not by how well they match `--testo`. The CLI now pulls the rows whose **title** contains every search term to the front, which is usually enough: `ddl cerca --legisl 17 --testo "gestione rifiuti" --limit 100` moves ddl 290 ("Riforma degli ambiti territoriali ottimali e nuove disposizioni per la gestione integrata dei rifiuti") from row 75 to row 2. **That reordering only sorts the window you already downloaded** — it cannot surface a document that sits past `--limit`. So when none of the rows shown has the terms in its title, a stderr hint says exactly that: read it as "the act you want is probably further down", raise `--limit`, or use `--frase` for the exact phrase. On `resoconti`, `sommari` and `convocazioni` the hint stops at `--limit`, since `--frase` does not exist on the `/bd/` backend. Whenever the result set is cut short, `*/cerca` says so too — treat both hints as "widen or narrow before concluding it does not exist".
- **The portal cuts list titles at 256 characters, so a missing term in the title proves nothing** ⚠️: long-titled acts — `Schema di progetto di legge costituzionale…`, `Disegno di legge voto…` — are exactly the ones whose subject sits past the cut. Sicily's XVII-legislature bill 199 is titled "…riconoscimento degli svantaggi derivanti dalla condizione di insularità" but the list shows "…svantaggi deriva", so a title match on `insularità` is impossible to see. Rows whose title hits the cap and does not match are therefore ranked **between** the proven matches and the off-topic rows, not lumped with the latter, and the "no relevant title" hint says how many titles were cut and tells you to open the document for the full one. `ddl cerca --legisl 17 --testo "insularità"` moves bill 199 from row 9 to row 1 this way.
- **`ddl iter` event `url` now points at the sitting record** ⚠️ *(behaviour change)*: on Aula events the event `url` is the resoconto scheda for that sitting, not the bill's own page — it used to repeat the bill page on every event, which is where the events are parsed from but not where they happened. The bill page moved to the report root (`url`, next to `legisl`/`numero`/`titolo`), which also gains it in `legge cronologia`. On events carrying a resoconto link, `archive_id`/`doc_id` are omitted: they identified the bill document and would be inconsistent beside a URL pointing elsewhere. **Aula and committee sittings are numbered independently**, so the link appears only where the portal marks the sitting as Aula — `Esitato per Aula (epa) Seduta n. 260 0400 Commissione QUARTA` is an Aula-phase event citing a *committee* sitting, and linking it would land on the unrelated Aula sitting 260. **The link is also dropped when the source contradicts itself** ⚠️: the Assembly holds one sitting per date, so two Aula events of the same iter giving that date different sitting numbers cannot both be right, and the portal does not say which is. `ddl iter 17 199` reports the final vote as "19 feb 2020 — Approvato dall'Assemblea — Seduta n. 179", but sitting 179 is 26 February — the vote is in 178, as its own record spells out. On such a date both events keep the bill's page as their `url` and a stderr hint tells you to resolve the real sitting from the date (`resoconti cerca --legisl 17 --data 2020-02-19`).
- **New: iter events whose sitting and date contradict each other are flagged `anomalia: true`** ⚠️: the coherence check now runs **both ways**, on `ddl iter` and on `legge cronologia` alike. Same date, different Aula sitting numbers — the Assembly holds one sitting a day. Same sitting number, different dates, while the resoconti archive gives that sitting a single date (`legge cronologia 17 9 --anno 2020` dates the 2020 budget vote 2 May in "Seduta n. 187"; `resoconti get 17 187` dates 187 to 28 April, and 2 May has no resoconto at all). Either way the events involved carry `anomalia: true`, keep the act's own page as their `url` instead of a resoconto link that may point at the wrong day, and the reason lands both on stderr and in the report's `note` field — which `--select` can no longer strip. The recovery step differs per direction, and getting it wrong lands you back at the false gap: on same-date you look the sitting up **from the date** (`resoconti cerca --legisl 17 --data 2020-02-19`), on same-sitting the date is the contested half and returns nothing (`--data 2020-05-02` → `[]`), so you start **from the number** (`resoconti get 17 187`, authoritative date at the root as `data`/`data_iso`, still mirrored in `fields.Data`). Before concluding a resoconto is missing, check `anomalia`: the contradiction is in the source, not a hole in the archive.

- **`--envelope` puts the truncation signal inside the JSON** ℹ️: by default `*/cerca` prints a bare array and the "results truncated" / "no relevant title" hints go to **stderr**, where a JSON consumer never sees them — that is how a truncated window gets read as "the document does not exist". Add `--envelope` and the output becomes `{"risultati": [...], "troncato": true, "hint": "..."}`. Opt-in, so existing pipelines that do `... --agent | jq '.[]'` keep working. `--select` filters **inside** `risultati` (the envelope keys always stay), and `--csv` ignores the flag since a wrapper makes no sense around a table. Recommended for agents: `resoconti cerca --legisl 17 --data 2019-10-01:2019-12-31 --agent --envelope --limit 10`. **The MCP tools handle this for you**: they pass `--envelope`, and every `hint:`/`warning:` line the CLI writes to stderr comes back inside the payload as an `avvisi` field — on every command, not just searches. Without that, the MCP transport would discard them all, since it reads stderr only when a command fails.
- **`--data` on the acts archives filters the *presentation* date, not the approval one** ℹ️: on `interrogazioni`, `interpellanze`, `mozioni`, `odg` and `risoluzioni`, `--data` is qualified on `DATPRE`, the date the act was filed. Press coverage usually reports the date an act was *approved in the chamber*, which is later — a motion approved on 2020-02-04 may well have been presented on 2020-01-28, and searching the approval date returns nothing. When a known date yields no result, widen the range backwards (`--data 2020-01-01:2020-02-04`) or combine it with `--firmatario`. The approval step itself is in the act's iter, visible with `get`.
- **New: `ddl cerca --data` filters bills by an arbitrary date range** ℹ️: `--data 2026-07-01:2026-07-28` (or a single `YYYY-MM-DD`) is qualified on `DATPRE`, the presentation date — the same field `--anno` uses, which is nothing but its Jan-1..Dec-31 range. That is why the two are **mutually exclusive**: together they would put two ranges in `AND` on one field and return zero rows with no explanation, so the command refuses them instead. It replaces the `--isis-query "(18.LEGISL E 260701/260728.DATPRE)"` workaround, which needed the `AAMMGG` date form. `leggi` has no equivalent: that archive indexes no date at all (`LEGANN`, the year, is the only temporal field), so there `--anno` plus client-side filtering on `data_iso` is the whole story.
- **New: `novita --since 7d` answers "what changed since my last check" across every dated archive** ℹ️: one call, one section per archive, and next to each the **source's publication lag**. That second half is what makes a zero readable: mozioni are published ~45 days late, so "the last 7 days" will be empty for a month and a half, and not because the Assembly is idle — when the requested window falls entirely inside the lag, the command says so rather than leaving an unexplained empty list. Distinct from `ddl drift`, which reports what *moved*: that needs an iter state to diff and only bills have one. `conteggio` is what it found, `--limit` (default 30) is what it shows. On the `leggi` archive a row is one law, not one article — the portal indexes one article per row, so without collapsing a single seven-article law counted as seven items; each row carries `articoli_trovati`, `atto` (`L.R. 14`) and `numero` (`14`, the value you pass to `--numero`). `pareri` and `biblioteca` are not dateable and are declared as such rather than reported empty. In `--since`, **`m` means months, not minutes**.
- **Repeating the same search on one session continues it instead of restarting it** ⚠️: the Icaro engine, asked the identical query twice on the same client, answers the second time from where the first left off — so the most recent documents silently vanish. Measured on 2026 interrogazioni, three identical 30-row calls: same client → first row 3 Aug, then 28 Jul, then 28 Jul; a fresh client each time → 3 Aug all three. There is no row threshold to stay under: a 30-step sweep (30…180, two runs each) shows the head is lost on every subsequent call, by as much as the previous call was long. So the fix is a new session per repeated search, not a lower limit. Commands that search once per archive were already safe (`deputato profilo` builds a client inside its loop); `sync coverage`'s two-pass year scan was latently exposed and is now fixed — today it changes nothing there, because the archive it affects (`leggi`) is delivered oldest-first and losing the head does not move the maximum.
- **`--select` can no longer strip the fields that qualify the answer** ⚠️ *(behaviour change)*: `troncato`, `conteggio`, `nota`, `hint` and `meta` now survive `--select` at the root of the payload. They are not data to choose among — they are what tells you whether the data is all of it. `deputato profilo "Chinnici Valentina" --legisl 18 --data 2026-06-01:2026-08-14 --select tipo,data` used to drop `troncato: ["interrogazioni"]` and `conteggio`, the only signal that the answer covered 46 acts out of 84 (the default is `--limit` 30 **per archive**) — and `--select` is exactly the flag agents are told to use to save context, so the warning vanished in the very usage where it matters most. The same now applies to `--envelope --select`, which keeps `troncato`/`hint`. Inside the rows of an array those names stay ordinary fields and `--select` still filters them.
- **Every date in the output now carries a `data_iso` twin** ℹ️: the portal writes dates in four different shapes — `28.07.26` (Icaro), `5.01.2026` (the `leggi` archive), `05/08/2026` (the `/bd/` backend) and `17 giu 2026` (the status block inside a bill) — and two of them coexist in a single `ddl iter` payload. None sorts as a string, and none is what the input flags want (`--data 2026-06-01:2026-08-14`), so the output could not be fed back into a query built with the same criteria. `data_iso` (`YYYY-MM-DD`) rides next to every readable `data`, in the JSON and as a CSV column. It is absent when the value is not a source date — the search range echoed at the root of `deputato profilo`, or the truncated word-dates of the `pareri` archive. This also fixes the sort in `deputato profilo`: `28/07/2026` used to beat `2026-08-03` lexicographically (`"28" > "20"`), so acts from the three `/bd/` archives surfaced above more recent ones.
- **`--csv` now renders the aggregate commands too** ⚠️ *(behaviour change)*: `ddl iter`, `legge cronologia`, `deputato profilo` and `commissione dossier` wrap their rows in an object, and the CSV renderer wanted a top-level array — so `--csv` printed their JSON and said nothing about the requested format not arriving, on exactly the commands one exports to analyse elsewhere. Two shapes are now recognised: a single list of objects at the root (`atti`, `eventi`), and lists nested one level (`sezioni[].risultati` on the dossier), where the container's scalar fields become columns on every row — `commissione dossier "SESTA" --legisl 18 --csv` yields the four sections concatenated, each row carrying the `tipo` it came from. When the rows are not deducible (no list of objects, or more than one) the output stays JSON rather than guessing, and a stderr hint says so.
- **`which` indexes the ordinary commands too, and says what the portal does not publish** ⚠️ *(behaviour change)*: the index went from the 9 hero features to 42 entries — the unique capabilities realigned with SKILL.md (`ddl stralci`, `sync coverage` and three of the five `analytics` rankings were missing), plus the everyday work: the 12 archive searches, the `get` commands, the filter vocabularies, `sync`, `doctor`. Where a capability is distinguished by a flag, the flag is part of the name (`analytics --type resoconti --group-by oratore`), so the answer is something you can paste. Questions that used to match nothing now resolve: "chi ha parlato di rifiuti in aula" → `resoconti cerca`, "chi parla di più in aula" → the speaker ranking, "novità degli ultimi sette giorni" → `ddl drift`. And questions about what the source does not carry get the reason instead of silence — `non_coperto` under `--json`, stderr otherwise: roll-call votes, attendance, amendments as an archive, ARS spending, and party affiliation as a queryable register. Each names the nearest thing you *can* do.
- **`which` matches on word boundaries, not substrings** ⚠️ *(behaviour change)*: the query `chi` used to score 3 against `commissione dossier` — two points from «ri**chi**esti» in the description and one from «cross-ar**chi**vio» in the group tag — so questions about capabilities this CLI does not have ("chi era assente alla seduta", "chi ha parlato di rifiuti in aula") got a plausible, wrong command back with full confidence. The no-match path already worked; substring noise was drowning it. Tokens shorter than four letters must now match a whole word, longer ones match by prefix ignoring the final vowel, because the index says «disegno di legge» and «commissione» while questions arrive in the plural. Note that under `--json` (so under `--agent`) a no-match still exits 0 with `matches: []` by design — the typed exit 2 is the text path.
- **`doctor`'s cache-staleness threshold (6h) and `sync stale`'s default (7d) are intentionally different, not a bug**: `doctor`'s cache section is generated framework code with a fixed, conservative 6h default and no flag to change it — it's a generic "is this stale enough to worry about" signal. `sync stale --max-age` defaults to 7d because ARS parliamentary data (sedute, commissioni, interrogazioni) doesn't turn over hourly; a weekly sync is normally plenty. An agent that automates sync should not rely on `sync stale`'s default alone as the sole freshness signal — it can report `stale: false` on a store `doctor` already flags as stale. Check `doctor`'s `cache.status` (or pass an explicit `sync stale --max-age` matching your own freshness needs) rather than assuming the two agree.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Vista cronologica cross-archivio
- **`ddl iter`** — Ricostruisce la cronologia completa di un disegno di legge: presentazione, passaggio in commissione, lavori d'aula, eventuale promulgazione come legge regionale.

  _Quando un agente deve raccontare 'a che punto sta il DDL X', questa è l'unica chiamata che restituisce la timeline completa senza incollare 5 ricerche manuali._

  Ogni evento porta il numero di **`seduta`**, quando il portale lo dichiara, e per le sedute d'Aula il campo **`url`** punta alla scheda del resoconto (la scheda dell'atto sta nel campo `url` della radice). Serve a rispondere a «in quale seduta l'hanno votato?» e, soprattutto, a non scambiare la data della notizia con la data della seduta — la stampa scrive quasi sempre il giorno dopo. Attenzione: sedute d'Aula e di commissione hanno numerazioni **indipendenti**, e il link compare solo dove il portale marca la seduta come d'Aula. In `--select` tieni sempre **`titolo`**: nella stessa seduta un ddl viene esaminato e poi votato («Esaminato in Aula» e «Approvato dall'Assemblea», 29 lug 2026 seduta 268 sul ddl 6030), e senza quel campo le due righe sono indistinguibili.

  Il campo **`sede`** dà la commissione in forma canonica (l'ordinale, quello che gli altri comandi accettano) **sulle righe in cui il portale dichiara una seduta**: è lì accanto che la scrive, e la si legge da lì anche quando il verbo dell'evento la nomina altrimenti o non la nomina. Le commissioni speciali tengono il nome per esteso, e il nome d'uso resta in `titolo`, che è verbatim — «Parere Commissione Bilancio» ha `sede: Commissione SECONDA`. Sulle righe senza seduta (assegnazioni, invii) vale la dicitura del verbo, quindi la stessa commissione può apparire con due nomi nella stessa cronologia: non raggruppare per `sede` dandola per canonica.

  L'ultimo evento di una legge è la pubblicazione in Gurs, con numero e data come li scrive la fonte («Pubblicazione Gurs n. 44o1 del 21 agosto 2020»): il suffisso dopo il numero è la notazione del portale per i supplementi — la Gazzetta è la n. 44 — e la data ripete quella dell'evento.

  ```bash
  ars-sicilia-pp-cli ddl iter 18 1153 --json
  ars-sicilia-pp-cli ddl iter 17 290 --json --select data,fase,seduta,titolo,url
  ```
- **`ddl stralci`** — Elenca i disegni di legge ricavati per stralcio da un ddl base. Il verso opposto è nel campo `stralcio` di `ddl get` e `ddl iter`, che dice da quale ddl lo stralcio proviene.

  _Durante la sessione di bilancio la finanziaria viene spacchettata in stralci che proseguono da soli: senza questo comando bisogna indovinarne i numeri, e non c'è una regola da indovinare (gli stralci del ddl 1030 sono 3030…8030, quelli del 738 sono una ventina fra 7381 e 73864)._

  ```bash
  ars-sicilia-pp-cli ddl stralci 18 1030 --json
  ```

  Il legame è **dichiarato dal portale**, non calcolato: ogni stralcio porta con sé il riferimento al ddl base (`ddl n. 1030/A Stralcio IV`). Due casi che l'output rende espliciti invece di nascondere: uno stralcio può nascere da **più ddl abbinati** (`di` con due voci), e per una parte degli atti della XVII legislatura il portale scrive l'id interno al posto del numero base — lì `base_dichiarata` è `false` e `di` resta vuoto, perché dedurre la base dalla numerazione sarebbe un'invenzione.
- **`deputato profilo`** — Aggrega in un'unica vista tutti gli atti firmati o pronunciati da un deputato: DDL, interrogazioni, interpellanze, mozioni, ordini del giorno, risoluzioni e interventi in resoconti d'aula. `--data` (range `YYYY-MM-DD:YYYY-MM-DD`) filtra per data di presentazione/seduta su tutti i sotto-archivi, per query storiche mirate senza dover alzare `--limit`.

  _Sostituisce un workflow di 7 click manuali con un'unica chiamata strutturata: pensata per agenti che rispondono a 'che ha fatto il deputato X?'._

  ```bash
  ars-sicilia-pp-cli deputato profilo "Abbate Ignazio" --legisl 18 --json --select tipo,data,titolo
  ars-sicilia-pp-cli deputato profilo "Safina" --legisl 18 --data 2024-07-01:2024-07-31 --json
  ```
- **`commissione dossier`** — Vista completa su una commissione: convocazioni in calendario, sommari lavori, DDL assegnati e pareri richiesti al Governo regionale. Accetta il codice `1`-`6`, l'ordinale (`PRIMA`..`SESTA`) o un frammento della denominazione d'archivio. Le **commissioni speciali** (Antimafia, Statuto, Unione Europea) non hanno un codice e si raggiungono solo per denominazione, che non coincide con l'etichetta d'uso corrente: `"Antimafia"` non corrisponde a nulla, la denominazione è *«Commissione d'inchiesta e vigilanza sul fenomeno della mafia e della corruzione in Sicilia»*. Un termine che non aggancia nessuna commissione non produce un dossier vuoto: l'errore elenca le denominazioni della legislatura.

  _Quando segui i lavori di una commissione specifica, questa è l'unica chiamata che dà il quadro completo invece di 3 ricerche separate._

  ```bash
  ars-sicilia-pp-cli commissione dossier "SESTA" --legisl 18 --json
  ars-sicilia-pp-cli commissione dossier "inchiesta e vigilanza" --legisl 18 --json
  ```
- **`legge cronologia`** — Partendo da una legge regionale promulgata (archivio 201), risale al DDL originario, ai pareri di commissione e al voto d'aula: l'inverso temporale di ddl iter.

  _Per ricercatori e giornalisti che partono dalla legge promulgata e vogliono raccontare come ci si è arrivati._

  ```bash
  ars-sicilia-pp-cli legge cronologia 18 26 --anno 2025 --json
  ```

### Analytics su campi strutturati
- **`analytics --group-by anno`** — Distribuzione dei documenti per anno in un archivio (aggregazione locale sul DB sincronizzato).

  ```bash
  ars-sicilia-pp-cli analytics --type ddl --group-by anno --limit 50 --json
  ```
- **`analytics --group-by cofirme`** — Quante volte ciascun deputato ha cofirmato, in diretta e senza sync (richiede `--legisl`). Es: `analytics --type ddl --group-by cofirme --legisl 18`.
- **`analytics --group-by cofirmatari`** — Mappa le alleanze legislative (coppie di co-firmatari di DDL). Richiede una **deep sync** che estragga i firmatari dalle schede di dettaglio: `ars-sicilia-pp-cli sync --resources ddl --deep`. Dopo, funziona su `--type ddl` (vedi **Known Gaps** per i costi).
- **`analytics --group-by oratore`** — Classifica gli oratori più attivi in Aula per numero di sedute in cui sono intervenuti. Gira **in diretta** sul backend `/bd/resoconti` (una richiesta per oratore della legislatura), quindi richiede `--legisl` e impiega ~1 minuto. Es: `ars-sicilia-pp-cli analytics --type resoconti --group-by oratore --legisl 18`. Se il backend non risponde per qualche oratore, la classifica esce comunque con i restanti e un `nota:` su stderr elenca i non misurati — che non vanno letti come «zero interventi». Se non risponde per nessuno, il comando fallisce invece di restituire una classifica vuota.

### Anagrafiche dal sito istituzionale
- **`gruppi elenco`** — Elenca i gruppi parlamentari di una legislatura (16, 17, 18; default 18), con lo slug per aprire il dettaglio. I nomi sono gli stessi del campo gruppo delle firme sugli atti, quindi l'elenco è anche il vocabolario per costruire la join. Con `--deputato "<nome>"` legge i dettagli di tutti i gruppi e risponde alla domanda inversa — in quale gruppo sta un parlamentare, con ruolo e collegio — a costo di una richiesta per gruppo.

  ```bash
  ars-sicilia-pp-cli gruppi elenco --legisl 18 --json
  ars-sicilia-pp-cli gruppi elenco --legisl 18 --deputato "Cracolici" --json
  ```
- **`gruppi get`** — La composizione completa di un gruppo: cariche (Presidente, Vice-Presidente, Segretario, Tesoriere), collegio di elezione, email e scheda di ogni componente. Accetta lo slug (dall'elenco) o il nome del gruppo; un nome ambiguo esce con i candidati.

  ```bash
  ars-sicilia-pp-cli gruppi get XVIII-misto --json
  ars-sicilia-pp-cli gruppi get "Partito Democratico" --legisl 18 --json
  ```

### Stato e monitoraggio
- **`ddl drift`** — Confronta lo stato dell'iter dei DDL tra due sync e segnala quelli "mossi" (da commissione ad aula, approvati, ritirati). Richiede due **deep sync** a distanza di tempo (`sync --resources ddl --deep`), perché il campo `iter` viene scritto solo dalla deep sync (vedi **Known Gaps**). Per la cronologia di un singolo DDL usa `ddl iter <legisl> <numero>`, che la legge in diretta dal documento.
- **`sync stale`** — Mostra per ognuno dei 12 archivi ARS: timestamp ultima sync, n. record locali, età della sync, eventuale segnalazione di staleness.

  _Per agenti che orchestrano sync automatico: decide se rinfrescare prima di rispondere o se i dati locali sono ancora freschi._

  ```bash
  ars-sicilia-pp-cli sync stale --json
  ```
- **`sync coverage`** — L'altra metà: fin dove arriva **la fonte**. Per ogni archivio dà la data del documento più recente che il portale espone, il ritardo in giorni rispetto a oggi e, accanto, l'ultima sync locale. Serve a leggere un `[]` per quello che è: se la notizia è del 12 agosto e l'archivio ddl è fermo al 28 luglio, la ricerca a vuoto è latenza della fonte, non un atto inesistente.

  ```bash
  ars-sicilia-pp-cli sync coverage --resources ddl --json
  ars-sicilia-pp-cli sync coverage --json   # tutti i 12 archivi, ~45 s
  ```

  Non assume l'ordinamento della fonte, che non è uniforme (`ddl` consegna dal più recente, `leggi` dal più vecchio): legge la prima pagina, verifica se le date scendono davvero e solo quando non lo fa scarica l'anno per prendere il massimo. Dove la misura non si può dare lo dice invece di inventarla — `pareri` ha le date scritte a parole e tagliate, `biblioteca` non ha colonna data, e sugli archivi `/bd/` può uscire l'errore di backend, che non è assenza di dato. `convocazioni` ha normalmente una data futura, perché annuncia sedute da tenere: il ritardo negativo è corretto e viene annotato.

## Recipes


### Sync iniziale completo XVIII legislatura

```bash
ars-sicilia-pp-cli sync --full --resources leggi,ddl,interrogazioni,mozioni,interpellanze,odg,risoluzioni,pareri,resoconti,convocazioni,sommari
```

Prima sincronizzazione di tutti gli archivi politici della XVIII legislatura — i dati restano in `~/.local/share/ars-sicilia-pp-cli/store.db`.

### Deep sync dei DDL (firmatari + iter)

```bash
ars-sicilia-pp-cli sync --resources ddl --legisl 18 --deep
```

Per ogni ddl scarica anche la scheda di dettaglio ed estrae i **firmatari** e lo **stato dell'iter** (assenti nella short-list). Sblocca `analytics --group-by cofirmatari --type ddl` e `ddl drift` (quest'ultimo richiede due deep sync a distanza di tempo). Costa ~1 richiesta extra per ddl, quindi è più lento di una sync normale.

### Iter completo di un DDL con output narrowing

```bash
ars-sicilia-pp-cli ddl iter 18 1153 --json --select fase,data,sede,titolo,oratori
```

Timeline del DDL 1153, mostrando solo i campi essenziali — riduce il payload per agenti. `titolo` fa parte degli essenziali: è ciò che dice *cosa* è successo, e senza di lui due eventi della stessa seduta escono identici.

### Network di co-firmatari su DDL

```bash
ars-sicilia-pp-cli sync --resources ddl --deep
ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 30 --csv
```

Produce un CSV con le coppie di deputati che firmano DDL insieme — pronto per import in `duckdb` o gephi. **La deep sync è obbligatoria**: senza, il comando restituisce `[]` (con un hint su stderr), perché i firmatari stanno solo nelle schede di dettaglio.

### Classifica DDL per proponente o gruppo (1 richiesta, senza sync)

```bash
ars-sicilia-pp-cli analytics --type ddl --group-by proponente --limit 20
ars-sicilia-pp-cli analytics --type ddl --group-by gruppo --json
```

Legge le viste già aggregate dal portale (`/edem/`): la classifica dei disegni di legge per deputato **proponente** (primo firmatario) o per **gruppo** parlamentare, con una sola richiesta e senza sincronizzazione. Copre la legislatura corrente; `--legisl` non filtra queste classifiche (viene ignorato con un avviso).

### Drift settimanale dei DDL

```bash
ars-sicilia-pp-cli ddl drift --since 7d --json
```

Confronta lo stato dell'iter rispetto a una settimana fa — i DDL che si sono mossi (commissione → aula, voto, ritiro) compaiono qui.

### Analytics sui cofirmatari

```bash
ars-sicilia-pp-cli sync --resources ddl --legisl 18 --deep
ars-sicilia-pp-cli analytics --type ddl --group-by cofirmatari --limit 20 --legisl 18 --json
```

Top 20 coppie di deputati che firmano insieme DDL nella XVIII legislatura — aggregazione locale sul DB sincronizzato, che va popolato con una **deep sync**: senza, il risultato è `[]`.

### Ricerca per tema (vocabolario materie)

```bash
# Scopri le materie disponibili, filtra per parola chiave
ars-sicilia-pp-cli ddl materie | grep -i "sanit\|salut\|lavoro\|ambiente"

# Tutti i DDL sull'ambiente nella XVIII
ars-sicilia-pp-cli ddl cerca --legisl 18 --materia "Ambiente" --json | \
  jq -r '.[] | "\(.data) — \(.title)"'
```

Utile per giornalisti che seguono un tema: la lista completa delle 123 materie è navigabile offline senza aprire il portale.

### Veterani del parlamento — chi dura di più

```bash
ars-sicilia-pp-cli ddl firmatari --json | \
  jq -r 'group_by(.nome)[] | select(length >= 4) | "\(length) legislature — \(.[0].nome)"' | \
  sort -rn | head -10
```

Identifica i parlamentari con la carriera più lunga: quante e quali legislature hanno coperto. Cracolici Antonino è il record attuale con 6 legislature consecutive (XIII→XVIII).

### Seguire un deputato — carriera e attività

```bash
# In quali legislature ha operato?
ars-sicilia-pp-cli ddl firmatari --search "Scoma" --json | jq -r '.[].legisl' | sort | tr '\n' ' '

# Tutti i DDL presentati nella XVIII
ars-sicilia-pp-cli ddl cerca --legisl 18 --firmatario "Scoma Francesco" --json | \
  jq -r '.[] | "\(.data) — \(.title)"'
```

### Nuovi deputati — chi è al primo mandato

```bash
ars-sicilia-pp-cli ddl firmatari --json | \
  jq -r 'group_by(.nome)[] | select(length == 1 and .[0].legisl == "18") | .[0].nome'
```

Filtra i deputati presenti solo nella XVIII — al loro primo mandato regionale.

### Iniziative parlamentari vs governative

```bash
# Tipi di iniziativa disponibili
ars-sicilia-pp-cli ddl iniziative

# DDL a iniziativa governativa nella XVIII
ars-sicilia-pp-cli ddl cerca --legisl 18 \
  --isis-query "(18.LEGISL E Governativa.FIRMAT)" --limit 50 --json | jq 'length'
```

Distingue le proposte dei deputati (parlamentare) da quelle dell'esecutivo regionale (governativa).

## Usage

Run `ars-sicilia-pp-cli --help` for the full command reference and flag list.

Per query avanzate con `--isis-query` (operatori `NOT`/`WITH`/`NEAR`/`ADJ`, qualificazione di
campo, range di date, radici) vedi la guida [docs/isis-query-syntax.md](docs/isis-query-syntax.md),
con la tabella delle sigle di campo verificate.

## Commands

### biblioteca

Catalogo Bibliografico (archivio 205) e Opere Multimediali (205multimedia).

- **`ars-sicilia-pp-cli biblioteca cerca`** - Cerca nel catalogo bibliografico per autore, titolo, soggetto o ISBN.
- **`ars-sicilia-pp-cli biblioteca multimediali`** - Cerca nelle opere multimediali.

### commissioni

Lavori delle commissioni: convocazioni (229) e sommari (230).

- **`ars-sicilia-pp-cli commissioni convocazioni`** - Convocazioni delle Commissioni.
- **`ars-sicilia-pp-cli commissioni sommari`** - Sommari dei lavori di commissione. Il backend `/bd/` tronca le risposte grandi e consegna intere quelle piccole: conviene restringere: `--numero` (numero della seduta di commissione, il filtro più selettivo), poi `--anno`, poi `--commissione`. Misurato: `--numero 270` riesce 10 volte su 10, la stessa ricerca senza filtri 2 volte su 8. Se una ricerca fallisce per troncatura, la CLI dice quale filtro aggiungere.

### ddl

Disegni di Legge (archivio 221): proposte di legge presentate all'ARS.

- **`ars-sicilia-pp-cli ddl cerca`** - Cerca disegni di legge per legislatura, anno, firmatario, materia o testo.
- **`ars-sicilia-pp-cli ddl get`** - Scarica un singolo disegno di legge.

### interpellanze

Interpellanze parlamentari (archivio 234).

- **`ars-sicilia-pp-cli interpellanze cerca`** - Cerca interpellanze.
- **`ars-sicilia-pp-cli interpellanze get`** - Scarica una singola interpellanza.

### interrogazioni

Interrogazioni parlamentari (archivio 233).

- **`ars-sicilia-pp-cli interrogazioni cerca`** - Cerca interrogazioni per legislatura, firmatario o rubrica.
- **`ars-sicilia-pp-cli interrogazioni get`** - Scarica una singola interrogazione.

### leggi

Leggi della Regione Siciliana (archivio 201): cerca e scarica le leggi regionali.

- **`ars-sicilia-pp-cli leggi cerca`** - Cerca leggi regionali per legislatura, anno, numero o testo.
- **`ars-sicilia-pp-cli leggi get`** - Scarica una singola legge regionale.

### mozioni

Mozioni parlamentari (archivio 235).

- **`ars-sicilia-pp-cli mozioni cerca`** - Cerca mozioni.
- **`ars-sicilia-pp-cli mozioni get`** - Scarica una singola mozione.

### odg

Ordini del Giorno (archivio 236).

- **`ars-sicilia-pp-cli odg cerca`** - Cerca ordini del giorno.
- **`ars-sicilia-pp-cli odg get`** - Scarica un singolo ordine del giorno.

### pareri

Pareri richiesti dal Governo regionale alle Commissioni (archivio 226).

- **`ars-sicilia-pp-cli pareri cerca`** - Cerca pareri richiesti dal Governo.
- **`ars-sicilia-pp-cli pareri get`** - Scarica un singolo parere.

### resoconti

Resoconti delle Sedute d'Aula (archivio 217).

- **`ars-sicilia-pp-cli resoconti cerca`** - Cerca resoconti per data, numero, oratore o testo.
- **`ars-sicilia-pp-cli resoconti get`** - Scarica un singolo resoconto. Non restituisce la trascrizione integrale: l'archivio Icaro ne conserva solo frammenti per punto dell'ordine del giorno e si ferma alla seduta n. 232 del 25.02.2026. Quando Icaro non ha la seduta, `get` ripiega sulla scheda del backend corrente e restituisce `pdf_url`, dove sta il resoconto stenografico completo (il PDF non viene scaricato; l'URL è stabile e citabile). La scheda non ha il campo `body` — che invece c'è sui record serviti da Icaro — e porta un campo `nota` che spiega perché: l'assenza di `body` non è «testo non disponibile». Se il backend non risponde (tronca le risposte a intermittenza) ogni lettura viene ritentata fino a 3 volte, e l'errore che esce dopo resta distinto da `nessun documento trovato`, che compare solo quando il backend ha risposto e il documento davvero non c'è. **I due percorsi hanno la stessa forma**: `legisl`, `numero`, `data`, `data_iso`, `titolo` e `fonte` stanno in radice anche sulla scheda Icaro, quindi lo stesso `--select numero,data_iso,titolo` rende su tutte le sedute. Prima quelle coordinate stavano solo dentro `fields`, e quel `--select` tornava `{}` con exit 0 su ogni seduta più vecchia della 232 — che si legge come «il documento non ha quei dati». `fields` resta dov'era: è un'aggiunta, non uno spostamento. `fonte` dice quale dei due percorsi ha risposto.

### risoluzioni

Risoluzioni parlamentari (archivio 238).

- **`ars-sicilia-pp-cli risoluzioni cerca`** - Cerca risoluzioni.
- **`ars-sicilia-pp-cli risoluzioni get`** - Scarica una singola risoluzione.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ars-sicilia-pp-cli ddl get 18 1153

# JSON for scripting and agents
ars-sicilia-pp-cli ddl get 18 1153 --json

# Filter to specific fields
ars-sicilia-pp-cli ddl get 18 1153 --json --select data,numero,title

# Dry run — show the request without sending
ars-sicilia-pp-cli ddl get 18 1153 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ars-sicilia-pp-cli ddl get 18 1153 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors **and hints** to stderr. Every command may write `hint:`/`warning:` lines there, and they are not exceptional — the seduta/date anomaly on `legge cronologia 18 1 --anno 2025`, the truncation notice on `*/cerca`, the stale-store warning on `search`. **Never pipe `2>&1` into `jq`**: the hint lands in front of the JSON and `jq` dies with a parse error and exit 5, which reads as an intermittent CLI failure and is not one. Keep stderr separate (`2>/dev/null`), or use `--envelope` and the MCP tools, which carry the same text inside the payload.
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending, **naming the backend that would actually serve it**. Archives migrated to the portal's `/bd/` backend (`resoconti`, `sommari`, `convocazioni`) report `backend: "bd"`, the POST endpoint in `would_post`, and the request split in two: `post_fields` are the fields that travel exactly as shown — backend field names, not your flags (`legisl` leaves as `$Ilegislatura`), plus the mode selectors the form always carries — while `deferred` names the filters that only resolve at request time and says what they become (`--data` is not a field at all: it turns into **one request per year** of the range — the years are listed in `anni` and counted in `richieste`, and `anno` is among the `post_fields` holding the first of them — the value the first request carries, plus a client-side filter that trims the days outside the range; `page` is among the `post_fields` and is 1 on the first request of each year, then climbs to the page count the response declares — that count arrives **inside** the response, so pages beyond the first cannot be previewed and the preview states the rule instead of inventing a number; when `--anno` falls outside the range the preview says no year is left to query; `--oratore` and `--commissione`/`--codcom` are resolved from name to id by reading the form's `<option>` list); the rest report `backend: "icaro"` with `isis_query` and `would_fetch`. The commands that query several archives (`legge cronologia`, `deputato profilo`, `commissione dossier`) list one entry per request under `requests`, in the order they would fire — on the dossier that is where you see the same argument leave as `codcom` towards `/bd/` and as the lettered ordinal (`SESTA`) towards ISIS, which explains half-empty sections without guessing
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` generic failure (including non-429 HTTP errors from the portal), `2` usage error, `3` not found, `7` rate limited, `10` config error. **There is no exit 5** — the CLI never emits it. If you see one, it came from something else in your pipeline: `jq` exits 5 on unparsable input, which is what happens when hints are folded into stdout (see the stderr note in Agent Usage).

## Health Check

```bash
ars-sicilia-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/ars-sicilia-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **I comandi `cerca` restituiscono 0 risultati ma il sito ne mostra molti.** — Verifica la legislatura: senza `--legisl` la query usa il default. Il portale ARS richiede sempre una legislatura nel criterio. Esempio: `--legisl 18` per XVIII.
- **Errore di sessione o redirect inatteso.** — Il portale resetta la sessione dopo 30 minuti di inattività. Riprova il comando: il client acquisisce una nuova `JSESSIONID` automaticamente.
- **Comando `ddl iter`, `deputato profilo`, `legge cronologia` o `commissione dossier` non trova nulla.** — Queste viste interrogano il portale **in diretta** (non richiedono `sync`): verifica `--legisl` e gli identificativi (numero DDL, nome deputato, numero legge, nome commissione). I comandi che invece leggono dal DB locale e richiedono `sync` sono solo `search`, `analytics`, `ddl drift` e `sync stale`.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**opendatasicilia/RSSdisegniLeggeAssembleaRegionaleSiciliana**](https://github.com/opendatasicilia/RSSdisegniLeggeAssembleaRegionaleSiciliana) — Shell
- [**aborruso/ars_sicilia**](https://github.com/aborruso/ars_sicilia) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
