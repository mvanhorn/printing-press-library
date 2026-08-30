# 📋 SPEC.md — grants-pp-cli (product-owner output, T-100)

> List and filter open research grants — keyless, from 3 free APIs.

## Data sources (all keyless)
| Source | API | What it gives |
|---|---|---|
| Grants.gov | `POST api.grants.gov/v1/api/search2` + `fetchOpportunity` | OPEN funding opportunities (NIH/NSF/all federal), with deadlines; award ceiling + eligibility in the details |
| NIH RePORTER | `POST api.reporter.nih.gov/v2/projects/search` | Awarded NIH grants (amount, organization) — a "how much do they give for this" benchmark |
| NSF | `GET api.nsf.gov/services/v1/awards.json` | Awarded NSF grants (amount, institution) |

## Commands
| Command | Purpose | Main flags |
|---|---|---|
| `search <keyword>` | open opportunities (Grants.gov) | `--closing-before YYYY-MM-DD`, `--agency CODE`, `--rows N`, `--details`, `--min-award N`, `--eligibility TEXT`, `--json` |
| `nih <keyword>` | awarded NIH projects | `--min-amount N`, `--year YYYY`, `--rows N`, `--json` |
| `nsf <keyword>` | awarded NSF grants | `--min-amount N`, `--rows N`, `--json` |
| `doctor` | are all three APIs up | — |

Note: on `search`, `--min-award` / `--eligibility` automatically trigger a detail fetch
(`fetchOpportunity`), because the award ceiling and eligibility are only available there.

## Data model
- **Opportunity** (Grants.gov): id, number, title, agency, openDate, closeDate (MM/DD/YYYY), status; from the details: awardCeiling, awardFloor, applicantTypes[]
- **NIHProject**: projectNum, title, org, pi, awardAmount, fiscalYear
- **NSFAward**: id, title, awardee, fundsObligated, startDate, expDate

## Acceptance criteria
1. ☐ All three commands return real rows from LIVE APIs (evidence: run output)
2. ☐ `--closing-before` demonstrably filters (fewer rows / rows within the deadline)
3. ☐ `--min-amount`/`--min-award` filter numerically (including from `$`-formatted strings)
4. ☐ No `exec.Command`, no API key, no env dependency (retraction-checker pattern)
5. ☐ `go vet` clean + unit tests for the filter logic green
6. ☐ A network error yields a meaningful error message (which API, what status), exit code 1
7. ☐ `doctor` shows the state of all three sources ✔/✘

## Non-goals (v1)
No cache, no watch, no AI synthesis, no MCP server — the full retraction-checker machinery
is overkill here; this is a lean CLI that stays readable one file at a time.
