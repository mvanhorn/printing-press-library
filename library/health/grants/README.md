# 🔬 grants-pp-cli

> 🇭🇺 Nyitott kutatási pályázatok terminálból — kulcs nélkül, 3 ingyenes API-ból.
> 🇬🇧 Open research grants from the terminal — keyless, from 3 free APIs.

Created by [@laci141](https://github.com/laci141) (laci141).

| Forrás | Mit ad |
|---|---|
| **Grants.gov** | nyitott szövetségi kiírások (NIH, NSF, mind) — határidő, keret, jogosultság |
| **NIH RePORTER** | megítélt NIH grantek — "mennyit adnak erre a témára" |
| **NSF Awards** | megítélt NSF grantek |

## Build & használat
```bash
cd library/health/grants
go build -o grants.exe ./cmd/grants-pp-cli

./grants.exe doctor                                     # mindhárom API él-e
./grants.exe search "cancer immunotherapy" --rows 10    # nyitott kiírások
./grants.exe search "climate" --closing-before 2026-12-31 --min-award 500000
./grants.exe search "biosensor" --eligibility "small business" --details
./grants.exe nih "alzheimer" --year 2025 --min-amount 1000000
./grants.exe nsf "quantum computing" --min-amount 500000
```
Minden parancsnál: `--json` a nyers kimenethez. Teljes flag-lista: `./grants.exe help`.

## Tervezési szabályok (retraction-checker minta)
- **Keyless:** nincs API-kulcs, nincs .env, nincs regisztráció.
- **Nincs `exec.Command`:** minden hívás közvetlen HTTP (stdlib `net/http`), 20s timeout, 1 retry 5xx-re.
- **Stdlib-only:** zéró külső függőség.
- Szűrő-logika tiszta függvényekben (`internal/cli/filter.go`) — unit-tesztelve.

## Megjegyzés a forrásokhoz
A "nyitott kiírás + keret + jogosultság" kombináció a Grants.gov-ból jön (a
`--min-award`/`--eligibility` szűrők soronkénti részlet-lekérést igényelnek — lassabb).
A NIH RePORTER és az NSF API *megítélt* granteket ad — arra jó, hogy lásd, egy témára
ténylegesen mekkora összegeket ítélnek meg, és ki nyert.
