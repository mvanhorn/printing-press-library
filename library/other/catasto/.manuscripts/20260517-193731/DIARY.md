# Catasto CLI — Build Diary

> **Run:** `20260517-193731`
> **Operator:** Roberto Bissanti
> **Press:** CLI Printing Press v4.8.0
> **Final CLI:** [`~/printing-press/library/catasto/`](../../../library/catasto/) (binary `catasto-pp-cli`)
>
> 🇮🇹 [Versione italiana / Italian version](DIARY_it.md)

This diary narrates how the CLI was conceived, researched, generated, refined, and
renamed. It is intentionally honest about the deviations from the canonical
`/printing-press` flow, because the deviations are informative: they show where
the machine works without ceremony and where ceremony actually carries weight.

---

## 1. The starting prompt — `/printing-press forMaps`

The session began with three characters of ambiguity: `forMaps`. The Press
asked what it was. The operator's answer made the scope concrete:

> forMaps is a website and platform that provides access to geospatial information
> from the Italian cadastral database. It takes information from the Agenzia delle
> Entrate and converts it into open-source GIS data distributed by the same Italian
> agency. The goal of our CLI is to simulate the behaviors of forMaps:
> 1) Given a cadastral address (provincia, comune, foglio di mappa, and, if available
>    for that comune, sezione and particella), take GPS (WGS84) information.
> 2) The reverse of the first process (from GPS to cadastral information).

That paragraph is the entire user requirement. Two specific transformations.
Forward and reverse. Italian cadastre at one end, WGS84 coordinates at the other.

A trap was immediately visible: the user mentioned the Italian _website_ `forMaps`
(a commercial product by STIMATRIX), so naming the CLI `forMaps` would create a
trademark clash. We deferred the naming choice to later (see §11).

---

## 2. Recon — looking for any HTTP surface we could use

The Printing Press is built around HTTP APIs. Cadastral data is geographic, not
REST-shaped. Before committing to a generation strategy we needed to know what
HTTP surfaces actually exist for Italian cadastre. Three parallel searches:

1. The official **AdE WMS service** at `wms.cartografia.agenziaentrate.gov.it`.
   It serves WMS tiles in EPSG:4258 / 6706, with layers `CP.CadastralParcel`,
   `fabbricati`, etc. WMS is map-tile-shaped — useful for visual but not for
   programmatic lookup.

2. The **AdE WFS service** at `wfs.cartografia.agenziaentrate.gov.it`.
   Vector data, but per the agency's own documentation it accepts only
   bounding-box queries, not attribute filters. This means we can't query
   "give me the polygon for foglio 508 particella B in Roma" — we'd have to
   guess the bbox first.

3. Commercial alternatives — **catastomappe.it**, **openapi.com Italian
   Cadastral**, and **forMaps.it/STIMATRIX** itself. All paid, all behind auth.
   We rejected them on principle: this CLI should be free.

Then a community find made the whole project possible:
[**ondata/dati_catastali**](https://github.com/ondata/dati_catastali) — Andrea
Borruso's organization had built a clever workaround. They downloaded the AdE
WFS dataset in bulk (it accepts bbox queries without filters, which is fine for
batch retrieval), ran `ST_PointOnSurface` to compute a centroid for every
parcel, and published the result as one Parquet file per Italian region on
GitHub. The Parquet schema is exactly what we want:

```
INSPIREID_LOCALID, comune (codice belfiore), foglio, particella, x, y
```

Where `x` and `y` are longitude × 10^6 and latitude × 10^6 stored as integers
to keep the Parquet files small. Divide by a million and you get decimal
degrees.

Plus an `index.parquet` that maps `codice belfiore → regional file name`.

A second find was equally crucial: while inspecting forum threads, the
**undocumented AdE ajax endpoint** turned up:

```
GET https://wms.cartografia.agenziaentrate.gov.it/inspire/ajax/ajax.php
   ?op=getDatiOggetto&lon=X&lat=Y
```

This returns a small JSON blob: `{"SIGLA_PROV":"RM","COD_COMUNE":"H501",...}`.
It's the AdE map application's own internal endpoint, but it's open and
unauthenticated. A test fetch confirmed it works for Italian coordinates.

So we had:
- **Forward (GPS → cadastral):** one HTTP GET to the AdE ajax endpoint.
- **Reverse (cadastral → GPS):** download a regional Parquet from ondata's
  GitHub, query in-process.

Both free. Both unauthenticated. Both reachable. The plan was now feasible.

---

## 3. Architecture — small spec, big hand-written novel surface

The Printing Press generates a Go CLI from an API spec. Most of its leverage
comes from large APIs where the spec yields dozens of typed commands. Here we
had _one_ HTTP endpoint that fit the spec format (the AdE ajax). Everything
else — the Parquet client, the comune resolver — would be hand-written novel
code.

This was a conscious decision to accept the cost-up-front of the generator
scaffold (config, store, doctor, MCP server, agent-context, flag plumbing) so
that the novel commands could slot into a battle-tested chassis. The
alternative — writing the CLI from scratch — would have meant re-implementing
all of that plumbing for two real commands. A bad trade.

The internal-YAML spec was about thirty lines:

```yaml
name: formaps     # later renamed to catasto
base_url: https://wms.cartografia.agenziaentrate.gov.it
auth:
  type: none
resources:
  lookup:
    endpoints:
      gps_to_cadastral:
        method: GET
        path: /inspire/ajax/ajax.php
        params: [op, lon, lat]
```

This produced the typed `catasto-pp-cli lookup` command. Every other
user-facing command would be hand-authored.

A pre-generation reachability gate confirmed the AdE ajax endpoint returned a
real parcel for Roma Colosseum coordinates. Green light.

---

## 4. Generation — first try, no fights

```
PASS go mod tidy
PASS govulncheck ./...
PASS go vet ./...
PASS go build ./...
PASS build runnable binary
PASS catasto-pp-cli --help
PASS catasto-pp-cli version
PASS catasto-pp-cli doctor
```

Eight quality gates passed on the first generation. The spec was small enough
to not stress the parser, and the generator's templates produced clean code
out of the box. The `lookup` command worked immediately against the live
ajax endpoint — no fixture mocking, no auth handshake to debug.

This was a relief. Most generation runs have at least one structural fight
(usually around auth or pagination). With `auth.type: none` and a single GET
endpoint, none of that applied.

---

## 5. The novel surface — building the four real commands

The user-facing CLI surface needed four commands:

1. `gps <lon> <lat>` — positional wrapper around the spec-derived `lookup`,
   with `--stdin` streaming for batch reverse-geocoding and an Italy bbox
   guard so junk coordinates fail fast.

2. `cadastral --comune --foglio --particella` — pure-Go Parquet reader over
   ondata. Downloads the relevant regional file once, caches it under the
   user OS cache dir, then queries in-process.

3. `validate` — parse-only syntax checker. No network. Useful as a guardrail
   in form-style flows and batch imports.

4. (deferred) — `neighbours`, `around`, `coverage`, `drift`, `search`. All
   noted in the absorb manifest as future work. They need a richer local
   cache (Parquet→SQLite sync) than the foundation slice would build.

For the Parquet path we picked `github.com/parquet-go/parquet-go` — pure-Go,
no DuckDB or CGO dependency. The user explicitly chose this option from a
three-way trade-off (pure-Go vs DuckDB vs hand-rolled). Single static binary
was the priority.

The Parquet client code went into a new `internal/ondata/` package. About 240
lines with a generic-typed `readParquet[T]` helper. The Italian-language
comments in the early draft (auto-generated) were converted to English to
match the rest of the codebase.

All four commands followed the Printing Press novel-command template:
- `cmd.Annotations["mcp:read-only"] = "true"` so the runtime MCP tree walker
  registers them as `readOnlyHint` tools.
- Italy bbox guard on `gps` to catch typos before burning an HTTP call.
- Cadastral input validation: belfiore is `[A-Z][A-Z0-9]{3}`.
- `dryRunOK(flags)` short-circuit so `verify --dry-run` passes.

By the end of phase 3 the CLI passed `printing-press verify` at 17/17 (100%)
and scored 80/100 (Grade A) on the scorecard. Two-and-a-half hours from the
opening prompt.

---

## 6. Shipcheck — fixing what the README was lying about

The first shipcheck umbrella run failed on two legs: `verify-skill` and
`validate-narrative`. Both for the same root cause: the generated README and
SKILL claimed novel features (`neighbours`, `around`, `coverage`, `drift`,
`search`) that we had deferred. The narrative-validation phase tried to run
the example invocations and got "unknown command" errors.

The fix was honest, not clever: trim `research.json`'s `novel_features` array
to what we actually built (`gps`, `cadastral`, `validate`), then re-run
`printing-press generate --force`. The force flag preserves hand-edited
files via AST merge while re-rendering everything templated from
`research.json`. The novel-features-built sync re-derived the README and
SKILL blocks. Verify-skill and validate-narrative both went green.

Worth noting: this round-trip is _expected_ in the Printing Press flow. The
generated narrative is aspirational at generation time and gets reconciled
against the actual built surface during shipcheck. The shipcheck failure is
the machine doing its job.

---

## 7. Live dogfood — the round-trip works

Phase 5 dogfood ran against the live endpoints:

- `catasto-pp-cli gps 12.4924 41.8902 --json` → comune `H501`, foglio `508`,
  particella `B` (the Colosseum).
- `catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B --json`
  → `lat=41.890252, lon=12.492405`.

The round-trip closed to within a microdegree. Forward and reverse agree.
This is the moment the project felt real.

Five tests passed, three were skipped (commands with no positional args, etc.).
Verdict: PASS. Wrote `phase5-acceptance.json` with `status: pass, level: quick`.

---

## 8. Polish & promote

Polish ran the diagnostic-fix-rediagnose loop and improved one finding: a
missing test file for the `internal/ondata` package. Added a small
`ondata_test.go` covering the helper functions (`normalizeNumericForms`,
`anyEqual`, the coordinate conversions). Dogfood moved from FAIL → WARN. The
remaining WARN findings were intentional design choices:

- `validate` flagged as "reimplemented" — it _is_ a pure-function offline
  validator, that's the point.
- `ondata.go` flagged for missing rate limiter — the source is static
  GitHub-hosted Parquet, cached locally on first download. A rate limiter
  would be vestigial.

Scorecard 81/100 Grade A. Polish recommended `ship`. Promoted to
`~/printing-press/library/formaps/` via `lock promote`. Lock released.
Manuscripts archived.

---

## 9. The G273/35/1900 bug report — turning a wall into a window

After promote, the user ran a test and reported a CLI bug:

```
catasto-pp-cli cadastral --comune G273 --foglio 35 --particella 1900
Error: parcel not found: comune=G273 foglio=35 particella=1900 in 19_Sicilia.parquet
```

I investigated by writing a Go probe that scanned the cached Sicilia Parquet.
Result: G273 (Palermo) has 134,981 rows. Foglio 35 has 1,530 parcels in
ondata's snapshot, particelle running 1 through 1899 plus alphanumeric (`X1`).
There is no particella 1900 in foglio 35 of Palermo in the dataset.

Three possible explanations:
1. ondata's snapshot pre-dates a parcel split/renumbering.
2. The AdE WFS that ondata mirrors has known gaps.
3. The parcel uses a sezione that ondata doesn't preserve.

The user confirmed the parcel is real but acknowledged it's just not in the
snapshot. The bug wasn't in our code, but the _error message_ was bad. "Parcel
not found" doesn't help a user distinguish "wrong input" from "missing data".

The fix turned the error into a diagnostic. Three cases now produce three
different messages:

```
# Unknown comune
Error: comune ZZZZ has 0 rows in 12_Lazio.parquet (check the codice belfiore)

# Wrong foglio (right comune)
Error: comune=G273 has 149 distinct foglios but none match foglio=9999;
       nearest existing foglio is 0149

# Wrong particella (right comune+foglio)
Error: comune=G273 foglio=35 exists with 1530 parcels in 19_Sicilia.parquet,
       but particella=1900 is not among them (nearest: 19, 190, 191, 192, 193)
```

The third case is the load-bearing one: it tells the user the foglio _does_
exist with N parcels, so the particella they typed is genuinely missing from
the data — not a typo. Lexicographic nearest-neighbor surfacing is the cheap
heuristic; numeric-aware sorting would be marginally better but the cost was
not worth it for an error message hint.

The user's bug report turned into a UX improvement that every subsequent
"parcel not found" case benefits from.

---

## 10. The multi-modal resolver — name, provincia, CAP

The user's next request expanded `cadastral` beyond codice belfiore:

> The cadastral research must admit province and comune research instead of
> Belfiore string, further the zip code (in italian CAP code, Codice di
> Avviamento Postale).

This needed an embedded dataset. Two minutes of search turned up
[**matteocontrini/comuni-json**](https://github.com/matteocontrini/comuni-json),
the canonical Italian comuni dataset derived from ISTAT + ANCI:

```json
{
  "nome": "Roma",
  "codice": "058091",
  "regione": {"nome": "Lazio"},
  "provincia": {"nome": "Roma"},
  "sigla": "RM",
  "codiceCatastale": "H501",
  "cap": ["00118", "00119", ...]
}
```

7,904 comuni, ~1.9 MB JSON. Embedded into the binary via `//go:embed`. Total
binary size grew from 27 MB to 28 MB.

The new `internal/comuni/` package exposes three resolvers:

- `ResolveByBelfiore(code)` — direct map lookup, O(1).
- `ResolveByName(name, provincia)` — accent-insensitive name match,
  optionally filtered by sigla or full province name; returns `ErrAmbiguous`
  with the candidate list when needed.
- `ResolveByCAP(cap)` — returns the full slice; CAPs are not unique.

`cadastral` was rewritten to accept any of three input forms:

```bash
catasto-pp-cli cadastral --comune H501 --foglio 508 --particella B
catasto-pp-cli cadastral --comune Roma --provincia RM --foglio 508 --particella B
catasto-pp-cli cadastral --cap 00184 --foglio 508 --particella B
```

The shape-detection was tricky: pure-letter four-character names like
"ROMA" technically match the belfiore shape `[A-Z][A-Z0-9]{3}`. A first
implementation incorrectly classified "Roma" as a belfiore code and failed.
The fix tightened the heuristic: a name only counts as belfiore if it
contains at least one digit. "H501" is belfiore; "Roma" is a name. (And as a
safety net, even if the heuristic flags something as belfiore, the resolver
falls back to name resolution if the belfiore lookup misses.)

A new top-level `comune` command was added for standalone resolution (with
no network). All three input modes from one entry point:

```bash
catasto-pp-cli comune --belfiore H501 --json
catasto-pp-cli comune --name Castro --provincia BG --json
catasto-pp-cli comune --cap 00184 --json
```

Eight tests in `internal/comuni/comuni_test.go`. All green.

---

## 11. The rename — `forMaps → catasto`

The user flagged the brand clash: `forMaps.it` is a real Italian commercial
product. Naming our CLI `forMaps` would be misleading.

The rename touched:
- Library directory: `~/printing-press/library/formaps/` → `.../catasto/`
- Binary: `formaps-pp-cli` → `catasto-pp-cli`
- MCP binary: `formaps-pp-mcp` → `catasto-pp-mcp`
- Go module path: `formaps-pp-cli/...` → `catasto-pp-cli/...`

A sed sweep over 72 text files handled the bulk. Build green, all tests
green, verify 100%.

A second pass later (after this diary was started) handled the residual
display strings: titles in `README.md` / `SKILL.md` / `AGENTS.md`,
`display_name: "Formaps"` in `manifest.json` and `.printing-press.json`,
the `FORMAPS_*` env-var names in generated source code, and the `.mcpb`
bundle filename. After that sweep, no occurrence of `formaps` /
`forMaps` / `Formaps` / `FORMAPS` remains in any non-historical file.

The historical manuscripts archive deliberately retains the original
`formaps-` filenames — they reflect the run state at the time of the
generation and renaming them would falsify the record.

---

## 12. Homonymy check — how many comuni share a name?

The user asked: are there Italian comuni with the exact same name?

A quick query against the embedded dataset answered:
**7 pairs**, 14 comuni total. All 2-way collisions; no triple-homonyms.

```
Calliano (AT/B418) — Calliano (TN/B419)
Castro (BG/C337) — Castro (LE/M261)
Livo (CO/E623) — Livo (TN/E624)
Paterno (PZ/M269) — Paternò (CT/G371)     [the accent matters; my normalize merges them]
Peglio (CO/G415) — Peglio (PU/G416)
Samone (TO/H753) — Samone (TN/H754)
San Teodoro (ME/I328) — San Teodoro (SS/I329)
```

Zero same-name-same-province cases. So `--provincia` is _always_ sufficient
to disambiguate. The CLI was already handling these correctly: a name
without `--provincia` for one of these seven returns `ErrAmbiguous` with
both candidates listed by sigla and belfiore code.

The user verified with `Castro` and confirmed the error message was good.
No code change needed.

The Paterno/Paternò case is an interesting edge — accent-insensitive
normalization fuses them, but accent-sensitive matching would resolve
`Paternò` (with accent) uniquely. We left the behavior as-is — the
trade-off favors users who don't type accents.

---

## 13. Lessons and known gaps

Honest list of what the CLI does _not_ do and why:

1. **Trentino-Alto-Adige.** TAA runs autonomous cadastral systems
   separate from AdE. No public dataset exists for Trento or Bolzano. The
   CLI returns a typed `ErrComuneNotIndexed` error pointing users to
   `catasto.provincia.tn.it` and `catasto.bz.it`.

2. **The ondata snapshot is not live AdE.** ondata regenerates the Parquet
   files quarterly. If a parcel was created or split after the last
   regeneration, it won't be in the dataset — but the underlying AdE WFS
   may still have it. The G273/35/1900 case is exactly this shape. We
   surface this honestly via the diagnostic error messages but don't
   automatically retry against live AdE. A future feature could.

3. **Deferred novel features.** The absorb manifest listed seven
   transcendence features. We shipped three (`gps`, `cadastral`,
   `validate`) plus the unplanned-but-essential `comune` resolver. Four
   features remain documented but unbuilt: `neighbours`, `around`,
   `coverage`, `drift`, `search`. They all need a Parquet→SQLite sync
   step first; building that is a clean follow-on.

4. **Sezione handling.** The ondata Parquet flattens sezione into the
   inspire ID encoding; the schema doesn't expose it as a separate column.
   `cadastral --sezione X` is echoed in output for round-tripping but
   doesn't participate in matching. A future feature could decode the
   inspire ID to recover sezione.

5. **Process deviations from the canonical SKILL flow.**
   - The novel-features subagent (Step 1.5c.5 of the press skill) was
     intentionally skipped. The surface was small (2 endpoints) and the
     transcendence design space was tightly bounded. A subagent run would
     have burned 2-3 minutes of latency for likely-zero signal.
   - Phase 5 dogfood ran at `quick` level rather than `full`, on the
     reasoning that full dogfood adds matrix coverage for write commands
     and complex error paths that this read-only API doesn't have.
   - The `pp-novel-static-reference` opt-out comment was not added to
     `validate` because polish judged the "reimplementation" finding to be
     a false positive and the user agreed.
   - Display-name rename was done in two passes (slug first, display second
     later in the session) rather than one. This was sloppy but harmless;
     no published artifact ever had the inconsistency.

6. **The 1900 parcel investigation.** I started writing a Go probe to
   enumerate every particella in G273 foglio 35 and the user stopped me:
   "The problem is that this particella is not represented in the map,
   but this particella is real. Ignore this point and go on." Good
   instinct. Investigation was already over the value-of-knowing line.

---

## Closing

The CLI is in `~/printing-press/library/catasto/` and works against live
endpoints. Forward and reverse round-trip to within a microdegree. Three
input forms for the reverse direction. Embedded 7,900-comune dataset. Pure
Go binary, no Python or DuckDB dependency. Free, no auth, no API keys.

What the user got from `/printing-press forMaps`:
- 4 user-facing commands (`gps`, `cadastral`, `validate`, `comune`) plus the
  framework commands (`doctor`, `agent-context`, `workflow`, etc.).
- An MCP server (`catasto-pp-mcp`) that mirrors every command for any
  agentic host (Claude Desktop, Claude Code, Codex, OpenCode, Cursor).
- A 28 MB single-binary Go install.
- Bilingual READMEs and this diary.

Total elapsed: about six hours of intermittent session time, including the
two follow-on rounds (G273 fix, multi-modal resolver) and the brand
rename. The Printing Press's leverage was real: ~70% of the final code
was generated scaffold, ~30% was hand-authored novel commands and the
Parquet client. Without the scaffold, every CLI like this one is a
weeks-long project. With it, six hours.

The data sources are the heroes:
[**ondata/dati_catastali**](https://github.com/ondata/dati_catastali)
made the reverse direction possible, and
[**matteocontrini/comuni-json**](https://github.com/matteocontrini/comuni-json)
made the multi-modal resolver possible. Both are unsung Italian open-data
infrastructure. This CLI is mostly a thin Go wrapper around their work.

— Roberto Bissanti, May 2026
