# Shodhganga CLI Build Log

Manifest transcendence rows: 5 planned, 5 built. Phase 3 gate passed (5/5 novel features resolve).

## What was built

**Generated framework (from `shodhganga-spec.yaml`, `response_format: html`):**
- config, client, local SQLite store + FTS, doctor, root, MCP surface (5 tools), output helpers, README/SKILL scaffolding.
- Baseline endpoint commands: `thesis search`, `thesis get`, `browse list`, `university list`, `university get`.

**Hand-authored DSpace client (`internal/dspace/`, ~430 LoC + tests):**
- `dspace.New()` HTTP client with `cliutil.AdaptiveLimiter` pacing and `*cliutil.RateLimitError` on exhausted 429 retries.
- `Search()` (parses `/simple-search` result rows + total count), `Item()` (parses item-page Dublin Core `<meta>` tags into a typed `Thesis`), `Browse()`, `NormalizeID()`.
- Pure parsers `ParseSearch`/`ParseItem` + `classifyPublishers`/`normalizeAbstract`, unit-tested (7 test funcs, all passing) with HTML fixtures — no network in tests.

**Hand-authored commands:**
- `thesis get <id>` — **rewritten** to use `dspace.Item` → full DC record (researcher, guides, university, department, place, keywords, date, language, URI). Replaced the generated `page`-mode baseline which only captured title + nav links.
- `thesis search --query` — **rewritten** to use `dspace.Search` → clean `{hits, total}` (replaced links-mode baseline with a spurious `image` field).
- `harvest <query>` — NEW. Search → **parallel** (8-worker pool) item enrichment → store full `Thesis` records; partial-failure accounting (`fetch_failures`), records sync-state so default `search` prefers local.
- `guide <name>` — store join over `DC.contributor`.
- `university stats <name>` — aggregate: count, year range, departments, top subjects.
- `trends [--subject]` — group by completion year.
- `similar <handle>` — subject-keyword overlap ranking (store, live fallback for the target).

All hand-authored commands: verify-friendly RunE (help-only/dryRunOK/usageErr branches), `IsVerifyEnv()` guard on live commands (mock-mode = no network), `IsDogfoodEnv()` curtailment on harvest, `boundCtx` timeout boundary, `mcp:read-only` annotations, `// pp:data-source` annotations, missing-mirror guard on store commands.

## Intentionally deferred / out of scope
- **Full-text PDF download** — login-gated on Shodhganga; excluded (not a stub).
- OAI-PMH / REST / OpenSearch paths — disabled server-side (404/400); not used.
- Abstracts — Shodhganga stores most abstracts as PDFs (`"Abstract Available newline"` placeholder); parser normalizes those to empty. Real text abstracts, when present in the meta tag, are extracted.

## Generator notes (retro candidates)
- `html_extract mode: page` does not capture arbitrary `DC.*` / `DCTERMS.*` meta tags — only title + links. For DSpace/Dublin-Core sites this makes the generated item command low-value; hand-authored extraction was required.
- Dead generated helper `hasChangedLocalFlags` (helpers.go) flagged by dogfood (WARN) — pre-existing generator artifact.
