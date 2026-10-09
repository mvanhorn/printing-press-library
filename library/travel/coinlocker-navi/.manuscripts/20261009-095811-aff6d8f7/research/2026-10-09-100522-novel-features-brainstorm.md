# Novel-features brainstorm (subagent a372ded4e3598e748, 2026-10-09)

## Customer model
- Mei, first-time visitor on checkout-day itinerary (28-inch suitcase, no Japanese, Suica). Frustration: learns whether a large box is free only at the locker bank; translated pages show 情報なし as blank.
- Kenji, itinerary planner using an agent with ecbo-cloak-pp-cli and yamato-pp-cli. Frustration: joins coinlocker-navi static facts with Multi Ekicube live data by hand.
- Aya, day-tripper to Maihama (Tokyo Disney) and Umeda. Frustration: live map has empties but no hours/price; search page has hours/price but no empties.

## Candidates (pre-cut)
C1 near --live (cross-source live join) | C2 near --open-during (hours window) | C3 near --gate inside|outside | C4 drop (ranked decision command) | C5 bag-dimension fit | C6 total-cost estimate | C7 capacity summary | C8 compare stations | C9 vacancy watch | C10 ecbo/yamato fallback hint | C11 --lang en vocabulary | C12 --require known fields.
Inline kills: C5 (no dimension data), C6 (billing rules not in data), C9 (scope creep), C11 (free-text translation needs LLM).

## Survivors and kills
### Survivors
| # | Feature | Command | Score | Buildability |
|---|---|---|---|---|
| 1 | Live join on nearest lockers | near ... --live | 9/10 | hand-code |
| 2 | Hours-window filter | near ... --open-during HH:MM-HH:MM [--date] [--strict] | 8/10 | hand-code |
| 3 | Gate side across both sources | near ... --gate inside|outside | 8/10 | hand-code |

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| C4 drop | near + flags; padding vs "less is more" | near --live --open-during |
| C5 bag-dimension fit | no dimension data in either source | near size filter |
| C6 total-cost | billing rules absent; would fabricate | near per-size yen |
| C7 capacity summary | caller can sum box counts; weaker than live | near --live |
| C8 compare | two near calls; no join | near --live |
| C9 vacancy watch | polling loop is scope creep | vacancy |
| C10 fallback hint | hard-coded string; service_type + walk-up filter suffice | near |
| C11 --lang en | free-text translation needs LLM | near --live (English station names) |
| C12 --require | explicit nulls + --select already cover it | near --open-during --strict |
