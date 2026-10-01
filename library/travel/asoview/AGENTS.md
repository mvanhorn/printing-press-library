# Asoview agent guide

This is a focused read-only Asoview CLI built from public first-party HTML/GET contracts. Keep business behavior in `internal/asoview` and command adaptation in `internal/cli`. The Press framework is generator-owned; record essential generated-tree customizations under `.printing-press-patches/` so reprints retain their invariants.

Before changing source contracts, read `evidence/research.md` and `evidence/scope.md`. For live verification, use `scripts/live_check.py`; recorded output and synthetic tests are separate evidence categories. Read `evidence/FINAL.md` for the canonical staging/library paths and acceptance/review receipts.

Preserve source IDs/Japanese labels, canonical first-party URLs, nulls, exact units and fetch times. Advertised prices, dated bands, computed subtotals and confirmed quotes are distinct. General ticket validity, admission windows and reserved entry slots are distinct. Public stock is a snapshot; request-only and unknown are never sold out by inference.

Every business command uses a total bounded context through `sourceClient`, anonymous allowlisted GETs, bounded cache and compact JSON. `--data-source auto` uses fresh public cache then network, `live` bypasses cache, and `local` fails on a fresh-cache miss. Reference inventory refresh remains explicit. Declare `// pp:data-source auto` on network/cache commands and `computed` on derivations; output metadata records actual provenance.

After source changes, run targeted deterministic domain tests and meaningful live read-only assertions. Full checks are `go test -count=1 ./...`, `go vet ./...`, build, Press shipcheck and live dogfood. Source schemas changing is a typed failure, never a fixture fallback. Build verification caches and evidence stay project/run scoped. The build is local; publishing and registry installs require a separate authorized task.
