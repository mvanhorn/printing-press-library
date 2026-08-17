# Running Race Results

A command-line tool that looks up a runner's race result across multiple timing providers. Look up by **bib** or by **runner name** within a race, or pull an athlete's **full history across events**. Race names are fuzzy-matched, so `"berlin marathon"` resolves to the right event.

```
running-race-results-pp-cli lookup "<race name>" <bib>            # by bib
running-race-results-pp-cli lookup "<race name>" --name "<name>"  # by name, within the race
running-race-results-pp-cli athlete "<name>" [--provider athlinks|nyrr] | --racer-id <id> | --me   # cross-event history
```

```console
$ running-race-results-pp-cli lookup "berlin marathon" 73664 --year 2025
Race           BMW Berlin Marathon 2025
Runner         Sample Runner
Bib            73664
Net time       04:21:19
Gun time       04:29:35
Overall place  24556
Gender place   17968
Source         https://berlin.r.mikatiming.com/2025/?content=detail&...
```

## Providers

There is no single API for race results — each timing platform is different. This tool wraps several behind one interface.

| Provider | Coverage | Status |
|----------|----------|--------|
| **NYRR** | New York Road Runners events | ✅ live |
| **Mika Timing** | Berlin + World Marathon Majors (Boston, Chicago, London, Tokyo, …) | ✅ live |
| **Athlinks** | Aggregator (many events worldwide) | ✅ live — no token needed (`ATHLINKS_TOKEN` optional) |
| **RaceResult** | Events on `my.raceresult.com` | ✅ live |

## How it works

```
race name + bib
      │
      ▼
  catalog ──▶ fuzzy resolver ──▶ (provider, event, year)
                                        │
                                        ▼
                                 provider adapter ──▶ live API / page
                                        │
                                        ▼
                              unified Result ──▶ table / JSON
```

- **Catalog** (`internal/catalog/catalog.json`): a bundled map of known races → provider + event id + name aliases. Extend it by adding an entry — no code change needed.
- **Resolver** (`internal/resolve`): normalizes the query (drops sponsor prefixes like `TCS`/`BMW`), then scores it against the catalog with token-overlap + edit distance. `--year` / `--date` disambiguate the edition.
- **Provider adapters** (`internal/provider/*`): one per timing platform, each implementing a common `Provider` interface (`Lookup(event, bib) → Result`).
- **Result** (`internal/domain`): a unified shape (runner, bib, net/gun time, places, age group, splits, source URL). Missing fields are omitted, never faked.

## Install

Requires **Go 1.26.4+**.

```bash
# Install the CLI directly
go install github.com/mvanhorn/printing-press-library/library/other/running-race-results/cmd/running-race-results-pp-cli@latest
running-race-results-pp-cli --help
```

Or build from a clone:

```bash
git clone https://github.com/mvanhorn/printing-press-library
cd printing-press-library/library/other/running-race-results
go build -o running-race-results-pp-cli ./cmd/running-race-results-pp-cli
./running-race-results-pp-cli --help
```

## Usage

### Look up one result — by bib or by name

```bash
# By bib
running-race-results-pp-cli lookup "mini 10k" 19 --year 2026

# Disambiguate the edition by date (year is derived)
running-race-results-pp-cli lookup "berlin marathon" 73664 --date 2025-09-28

# By runner name within a race (any provider)
running-race-results-pp-cli lookup "berlin marathon" --name "Runner" --year 2025

# Machine-readable output (works on every command)
running-race-results-pp-cli lookup "berlin marathon" 73664 --year 2025 --json
```

Pass exactly one of `<bib>` or `--name`. A name that matches several runners
prints a list to refine by bib; an ambiguous race name lists the editions.

### Athlete history across events

```bash
# Athlinks (default) — all of a person's races; pick from the list if the name is common
running-race-results-pp-cli athlete "Sample Athlete"
running-race-results-pp-cli athlete --racer-id 43234281

# Your own Athlinks history — racer id read from ATHLINKS_TOKEN, no name needed
running-race-results-pp-cli athlete --me

# NYRR history (all of someone's NYRR races)
running-race-results-pp-cli athlete --provider nyrr "Sample Runner"
running-race-results-pp-cli athlete --provider nyrr --racer-id 2969961
```

`--provider` selects the history source (`athlinks` default, or `nyrr`). `--me`
is Athlinks-only (it reads the racer id from the token).

```console
$ running-race-results-pp-cli athlete --me
Date        Race                                  Distance       Net time  Overall
2025-09-21  Berlin Marathon                       Marathon       3:34:01   8917
2025-10-05  Jersey City 5K                        5K Run         18:52     187
...
```

## Configuration

Secrets are read from the environment (never hardcoded). Put them in a local `.env` (gitignored):

| Variable | Used by | Notes |
|----------|---------|-------|
| `ATHLINKS_TOKEN` | Athlinks | **Optional.** Athlinks' athlete, search, and detail endpoints are public — `lookup` and `athlete` work with no token. Set it only for `athlete --me` (your racer id is read from the token) or as a fallback if an endpoint returns 401/403. A `Bearer …` token from the Athlinks frontend; short-lived (~2h). |

```bash
# .env — optional; only needed for `athlete --me` or auth-gated endpoints
ATHLINKS_TOKEN="Bearer eyJ…"
```

## Development

```bash
go test ./...        # unit tests (offline — adapters run against recorded fixtures)
go vet ./...
gofmt -l .           # must be empty
```

- Adapters are tested against real recorded responses in `testdata/fixtures/<provider>/`, served by an in-process `httptest` server — no network in unit tests.
- Provider request/response shapes are documented in [`docs/providers/contracts.md`](docs/providers/contracts.md).
- **Adding a provider:** implement `provider.Provider` in `internal/provider/<name>`, register it in `cmd/running-race-results-pp-cli/main.go`, add catalog entries, and a fixture-backed test.

## Notes

- Lookups are **by bib**, not by runner name. Live tracking and analytics are out of scope.
- Test fixtures are anonymized recorded response shapes used purely for offline tests.
