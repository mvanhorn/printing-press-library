# Novel Features Brainstorm: rechtspraak-pp-cli

## Customer model

**Persona A — Mira, the appellate litigator (Hoge Raad practice).**
Boutique cassation lawyer in Den Haag. Lives in the appeal-chain: when a new HR decision lands, she needs the conclusie A-G, the gerechtshof decision under cassation, and the eerdere aanleg from the rechtbank — yesterday. Today she clicks through rechtspraak.nl tabs by hand or asks a paralegal to assemble the chain. Wants: `chain ECLI:...` that walks `dcterms:replaces` / `psi:cassatie` / `psi:conclusie` / `psi:eerdereAanleg` and prints the full tree; `dossier 22/00155` to pull every ECLI sharing the zaaknummer; citation export (`vindplaatsen`) for her brief's voetnoten.

**Persona B — Joris, the legal-tech researcher / NLP-er at a Dutch university.**
PhD candidate building a Dutch jurisprudence corpus. Needs bulk extract by court+subject+year, deterministic incremental sync, FTS over `inhoudsindicatie`, and clean JSON-LD. Today he duct-tapes `basm92/get_documents.sh` with rechtspraak-extractor's `lido.db`. Pain: no single tool gives him "all bestuursrecht decisions from RBAMS 2020-2024 with summaries, queryable, agent-pipeable."

**Persona C — Sanne, the legal journalist at NRC/FD.**
Covers tax and competition rulings. Needs "what did the HR publish on belastingrecht this week" without writing code, and wants to be alerted to new decisions touching a zaaknummer she's tracking. Today: refreshes rechtspraak.nl by hand. Wants `watch --court HR --subject belastingrecht --since 7d` and `dossier <zaaknummer> --watch`.

**Persona D — Anouk, the civic-tech / AI-agent builder.**
Builds Claude/MCP workflows for a legal-aid NGO. Needs the CLI to behave as an agent backend: `--json` everywhere, ECLI parsing, court-code lookup that works offline, stable exit codes. Pain: every existing lib is a Python/JS/Java SDK — none are an MCP-friendly binary. No MCP server exists for rechtspraak.

## Candidates (pre-cut)

### (a) Persona-driven / competitor gaps
- C1 chain ECLI walker
- C2 citations/vindplaatsen extractor
- C3 dossier-by-zaaknummer
- C4 watch poll
- C5 drift snapshot diff
- C6 court-code dictionary
- C7 bulk archive ingest
- C8 full-text plain export

### (b) Persona-specific
- C9 conclusie pairing
- C10 cited-by reverse
- C11 ECLI parse/validate
- C12 stats aggregate
- C13 random sample
- C14 diff two decisions
- C15 recent sugar
- C16 open in browser
- C17 watch --quiet cron mode

### (c) Cross-CLI patterns
- C18-19 doctor / --select (already absorbed)
- C20 mcp serve
- C21 shell completions
- C22 cache management
- C23 export bundle

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Appeal-chain walker | `chain ECLI:NL:HR:2024:1 [--depth N] [--json]` | 10/10 | hand-code | Recursively calls content endpoint, parses relations (cassatie/conclusie/eerdereAanleg/replaces), walks both directions, prints tree or graph JSON. Caches visited ECLIs. | Brief P2; Mira workflow #3; java-rechtspraak gap. |
| 2 | Vindplaatsen extractor | `citations ECLI:... [--format=bibtex\|csv\|json]` | 9/10 | hand-code | Extracts `psi:vindplaats` list, parses {journal, year, page, annotator}, emits BibTeX/CSV. | rechtspraak-js calls them out as a data quality issue; Mira voetnoot workflow. |
| 3 | Dossier-by-zaaknummer | `dossier 22/00155 [--court ...] [--json]` | 9/10 | hand-code | Local SQLite lookup by `zaaknummer` after sync; live fallback. Returns every ECLI sharing the case across instances. | Brief workflow #4; no competitor has it. |
| 4 | Watch / incremental poll | `watch [--court HR] [--subject ...] [--since 7d] [--quiet] [--json]` | 9/10 | hand-code | Filters search, diffs by ECLI against SQLite, prints only new ones. Updates sync cursor. | Brief P2; Sanne ritual; openstate has server-grade version. |
| 5 | Conclusie pairing | `conclusie ECLI:NL:HR:2024:1` | 8/10 | hand-code | Given an HR ECLI, returns the matching A-G conclusie via `psi:conclusie` relation (and reverse). | Mira's daily move; no competitor. |
| 6 | Bulk archive ingest | `sync --archive <yyyy-mm-dd>` | 8/10 | hand-code | Downloads weekly full-corpus ZIP, bulk-inserts to SQLite. Order of magnitude faster than Atom paging for backfills. | Brief Data Layer note; Joris backfill; Spijkervet ES indexer proves demand. |
| 7 | Court-code dictionary | `code RBAMS` / `code "Rechtbank Amsterdam"` | 7/10 | hand-code | Bidirectional offline lookup against the Instanties cache; returns code, name, parent, PSI URI. | Brief P2; Anouk agent-friction reducer. |
| 8 | MCP server mode | `mcp serve` | 9/10 | spec-emits | Generator emits MCP wrapper; flipping it on yields the first MCP server for Dutch case law. | Brief Product Thesis; Anouk-defining. |
| 9 | ECLI parse/validate | `ecli parse ECLI:NL:HR:2024:1` | 6/10 | hand-code | Pure local regex parse into country/court/year/sequence; validates. Bonus `ecli url <ECLI>`. | Anouk ergonomics; load-bearing for chain/citations. |

### Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| C5 drift | Low verifiability; scope creep (versioned snapshots). | C1 chain |
| C8 full-text export | Just a flag on `get --full`. | (absorbed #7) |
| C12 stats | Borderline reimplementation; weak persona pull vs chain/watch. | C4 watch |
| C13 random sample | `search ... \| shuf -n N` works. | — |
| C14 diff two decisions | Chain view does this better. | C1 chain |
| C15 recent | Alias for `search --from`. | (absorbed #1) |
| C16 open in browser | Trivial; not differentiating. | — |
| C17 watch --quiet | Subsumed by C4 as a flag. | C4 watch |
| C21 completions | Cobra emits automatically. | — |
| C22 cache mgmt | Generator default. | — |
| C23 export bundle | Scope creep. | C6 archive |
| C10 cited-by | Borderline scope creep; overlaps chain (which walks both directions). | C1 chain |
