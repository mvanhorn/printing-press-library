# 🔬 grants-pp-cli

> Open research grants from the terminal — keyless, from 3 free APIs.

Created by [@laci141](https://github.com/laci141) (laci141).

| Source | What it gives |
|---|---|
| **Grants.gov** | open federal opportunities (NIH, NSF, all) — deadline, award ceiling, eligibility |
| **NIH RePORTER** | awarded NIH grants — "how much do they give for this topic" |
| **NSF Awards** | awarded NSF grants |

## Build & usage
```bash
cd library/health/grants
go build -o grants.exe ./cmd/grants-pp-cli

./grants.exe doctor                                     # are all three APIs up
./grants.exe search "cancer immunotherapy" --rows 10    # open opportunities
./grants.exe search "climate" --closing-before 2026-12-31 --min-award 500000
./grants.exe search "biosensor" --eligibility "small business" --details
./grants.exe nih "alzheimer" --year 2025 --min-amount 1000000
./grants.exe nsf "quantum computing" --min-amount 500000
```
Every command accepts `--json` for raw output. Full flag list: `./grants.exe help`.

## Design rules (retraction-checker pattern)
- **Keyless:** no API key, no .env, no registration.
- **No `exec.Command`:** every call is direct HTTP (stdlib `net/http`), 20s timeout, 1 retry on 5xx.
- **Stdlib-only:** zero external dependencies.
- Filter logic lives in pure functions (`internal/cli/filter.go`) — unit-tested.

## A note on the sources
The "open opportunity + award ceiling + eligibility" combination comes from Grants.gov (the
`--min-award`/`--eligibility` filters require a per-row detail fetch — slower).
The NIH RePORTER and NSF APIs return *awarded* grants — useful for seeing what amounts are
actually awarded for a topic, and who won them.
