# Approved focused OMAKASE manifest
User preauthorized focused read-only scope and ordinary briefing/absorb gates. No bookings, payments, account writes or purchases. Exactly one independent reviewer, no brainstorming worker per explicit user instruction.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Area/cuisine/name discovery | Public catalogue | omakase-pp-cli restaurants find | Bounded cards, strict name relevance, next source page |
| 2 | Restaurant, courses, rules and Japanese names | Public detail | omakase-pp-cli restaurants show | Lazy localized detail, normalized JPY and raw terms |
| 3 | Filter vocabularies | Public search form | omakase-pp-cli filters | Exact wire values |
| 4 | Member-only features and costs | Public Premium page | omakase-pp-cli membership | Fresh public source text; account eligibility unknown |

## Transcendence
| # | Feature | Command | Buildability | Why useful | Long Description |
|---|---|---|---|---|---|
| 1 | Course price/fee inspection | courses | hand-code | Preserve minimum/range/tax and separate charges | none |
| 2 | Release schedule inspection | release | hand-code | Distinguish scheduled/TBD/irregular from seat availability | none |
| 3 | Date/party availability boundary | availability | hand-code | Unknown/login required without implying sold out; request/waitlist/release distinct | none |
| 4 | Bounded restaurant comparison | compare | hand-code | Public course/fee/cancellation/release fields with per-ID partial errors | none |
| 5 | Explicit local summary inventory | inventory | hand-code | Bounded refresh, offline search and freshness/coverage status | none |

## Gated scope
Exact date/party seats, Premium search/calendar and user eligibility are unavailable without login/membership and not shipped as live seat tools. Availability is a useful source-inspection command with a truthful unknown result, not a fabricated calendar. No auth acquisition is included.
