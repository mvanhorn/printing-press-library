# Executable E2E verification

Run from the project root:

```sh
go test ./internal/source ./internal/notebook
go test ./e2e -count=1 -v
```

The executable suite builds the real CLI into a temporary directory, launches a new process for each operation, isolates configuration and storage, and serves source fixtures through a temporary loopback HTTP server. It needs permission to listen on localhost. It makes no Internet requests. `TABELOG_E2E_BINARY=/absolute/path/to/binary` selects an existing binary instead of building one.

The tests cover typed catalogs and ambiguity, highest-rated discovery, source budget parameters and native pagination, complete facts under `--agent`, projection with metadata, cache/offline behavior, notebook operations, comparisons, alternatives, audits, partial refresh, source blocks/drift, redirect/body/deadline bounds, genuine empty results, and relocated/current same-name records. A detailed snapshot must survive a later listing fetch. Every approved command has help and dry-run coverage.

`testdata/fixture-manifest.json` records capture URLs or source paths, original hashes, sanitized hashes, and transparent transformations. Fixtures preserve public source cards, tables, JSON-LD and active constraints. They remove reviewer bodies, photographs, ephemeral inputs, telemetry and unrelated scripts. `testdata/oracle.json` contains independently annotated expected facts. The explicitly named unknown variants and the inline interstitial/drift responses are controlled mutations, not claimed live captures.

The production source URLs remain canonical HTTPS English Tabelog URLs. The suite sets `TABELOG_TEST_MODE=1` and `TABELOG_TEST_BASE_URL=http://127.0.0.1:PORT` to rewrite transport only. A non-loopback override is rejected. Test-mode configuration is separate from normal usage.

Default output inherits common `fetched_at`, `source_surface`, and primary `budget_source` from metadata. Explicit row or budget values override the common value. The suite reconstructs these documented values and verifies that an explicit `--select` still returns full per-record provenance and budgets.

To retain structured subprocess test evidence:

```sh
go test ./e2e -count=1 -json > "$PROOFS_DIR/e2e-results.jsonl"
```

Live source verification uses the Printing Press runner after the replay suite passes:

```sh
cli-printing-press dogfood --live --dir "$CLI_WORK_DIR" --level full \
  --research-dir "$RESEARCH_DIR" --json \
  --write-acceptance "$PROOFS_DIR/phase5-acceptance.json"
```

The runner owns the acceptance marker. Live checks establish current source compatibility; captured fixtures establish deterministic field fidelity. Ratings, counts, venues and live result order can change, so live checks cross-check the response's current facts instead of requiring historical fixture values. No booking or account mutation is part of this suite.

The optional measurement tool needs a separate Python environment. It adds no tokenizer dependency to the shipped Go runtime:

```sh
python3 -m venv "$PROOFS_DIR/tokenizer-venv"
"$PROOFS_DIR/tokenizer-venv/bin/python" -m pip install tiktoken==0.14.0
go build -o "$PROOFS_DIR/tabelog-e2e-measurement" ./cmd/tabelog-pp-cli
"$PROOFS_DIR/tokenizer-venv/bin/python" e2e/measure.py \
  --binary "$PROOFS_DIR/tabelog-e2e-measurement" \
  --output "$PROOFS_DIR/measurement-results.json" \
  --tokenizer-cache "$PROOFS_DIR/tokenizer-cache" --samples 20
```

The first tokenizer initialization may download its public encoding data; subsequent measurements reuse that cache. The tool records `tiktoken` version and `o200k_base`, actual binary hash, output tokens/bytes, nearest-rank p95 latency/CPU/RSS, and cold/warm HTTP counts. macOS `/usr/bin/time -l` measures each CLI process; CPU values have 10 ms reporting resolution. The replay server and tokenizer are outside the measured CLI process. Measurements use sanitized replay payloads and should be read alongside the original source byte counts in the fixture manifest.

## Disposable live notebooks

Run the following from the CLI checkout before live scorecard or full dogfood. The metadata-only `pp:happy-args` annotations target `.printing-press-fixtures/live-home` relative to that checkout; they do not change visible examples or ordinary storage defaults. Both runners execute with the checkout as their working directory. The live scorecard must honor the same annotation overlay as dogfood.

```sh
mkdir -p .printing-press-fixtures
tabelog_fixture_home="$PWD/.printing-press-fixtures/live-home"
./tabelog-pp-cli find --area https://tabelog.com/en/tokyo/A1301/A130101/rstLst/ --cuisine bar --meal dinner --budget-max 5000 --limit 5 --agent --home "$tabelog_fixture_home" > .printing-press-fixtures/candidates.json
./tabelog-pp-cli show https://tabelog.com/en/tokyo/A1301/A130101/13005012/ --data-source live --agent --home "$tabelog_fixture_home"
./tabelog-pp-cli lists add tokyo-bars 13005012 --note "Ginza bar fixture" --agent --home "$tabelog_fixture_home"
./tabelog-pp-cli lists add fixture-note 13005012 --note "Fixture note" --agent --home "$tabelog_fixture_home"
./tabelog-pp-cli lists add fixture-remove 13005012 --agent --home "$tabelog_fixture_home"
```

The CLI requires an absolute home path. The runners resolve the annotation path against the checkout before invoking it. Add two other fetched IDs from `candidates.json` to `tokyo-bars`, using the same explicit `--home`; this gives comparison and alternatives real saved candidates. Setup makes two bounded public requests; saved-list actions are local. `lists refresh` performs its own bounded detail request.

The dedicated remove fixture is consumed by a successful removal. Restore that membership before another removal test. A runner that repeats the same remove for JSON fidelity must account for the already-removed state; it is not a second independent fixture. No failure code is suppressed by these annotations.

The fixture directory is ignored by Git. After live checks, remove only this disposable directory before packaging or promotion:

```sh
rm -rf -- .printing-press-fixtures
```
