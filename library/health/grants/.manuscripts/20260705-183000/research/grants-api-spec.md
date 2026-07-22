# 📋 SPEC.md — grants-pp-cli (product-owner kimenet / output, T-100)

> 🇭🇺 Nyitott kutatási pályázatok listázása és szűrése — kulcs nélkül, 3 ingyenes API-ból.
> 🇬🇧 List and filter open research grants — keyless, from 3 free APIs.

## Adatforrások / Data sources (mind keyless)
| Forrás | API | Mit ad |
|---|---|---|
| Grants.gov | `POST api.grants.gov/v1/api/search2` + `fetchOpportunity` | NYITOTT pályázati kiírások (NIH/NSF/összes szövetségi), határidővel, keret + jogosultság a részletekben |
| NIH RePORTER | `POST api.reporter.nih.gov/v2/projects/search` | Megítélt NIH grantek (összeg, szervezet) — benchmark "mennyit adnak erre" |
| NSF | `GET api.nsf.gov/services/v1/awards.json` | Megítélt NSF grantek (összeg, intézmény) |

## Parancsok / Commands
| Parancs | Cél | Fő flagek |
|---|---|---|
| `search <kulcsszó>` | nyitott kiírások (Grants.gov) | `--closing-before YYYY-MM-DD`, `--agency KÓD`, `--rows N`, `--details`, `--min-award N`, `--eligibility SZÖVEG`, `--json` |
| `nih <kulcsszó>` | megítélt NIH projektek | `--min-amount N`, `--year YYYY`, `--rows N`, `--json` |
| `nsf <kulcsszó>` | megítélt NSF grantek | `--min-amount N`, `--rows N`, `--json` |
| `doctor` | mindhárom API él-e | — |

Megjegyzés: `--min-award` / `--eligibility` a search-nél automatikusan részlet-lekérést
(`fetchOpportunity`) von maga után, mert a keret/jogosultság csak ott érhető el.

## Adatmodell / Data model
- **Opportunity** (Grants.gov): id, number, title, agency, openDate, closeDate (MM/DD/YYYY), status; részletekből: awardCeiling, awardFloor, applicantTypes[]
- **NIHProject**: projectNum, title, org, pi, awardAmount, fiscalYear
- **NSFAward**: id, title, awardee, fundsObligated, startDate, expDate

## Elfogadási kritériumok / Acceptance criteria
1. ☐ Mindhárom parancs ÉLŐ API-ból ad valós sorokat (bizonyíték: futtatási output)
2. ☐ `--closing-before` bizonyíthatóan szűr (kevesebb/határidőn belüli sorok)
3. ☐ `--min-amount`/`--min-award` numerikusan szűr ($ formátumú stringekből is)
4. ☐ Nincs `exec.Command`, nincs API-kulcs, nincs env-függés (retraction-checker minta)
5. ☐ `go vet` tiszta + unit tesztek a szűrő-logikára zölden
6. ☐ Hálózati hiba = értelmes hibaüzenet (melyik API, milyen státusz), exit code 1
7. ☐ `doctor` mindhárom forrás állapotát mutatja ✔/✘

## Nem-célok / Non-goals (v1)
Nincs cache, nincs watch, nincs AI-szintézis, nincs MCP-szerver — a retraction-checker
teljes gépezete overkill ide; ez lean, egyfájlonként áttekinthető CLI.
