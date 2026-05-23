# Novel Features Brainstorm — Dice FM Partners API CLI

## Customer model

**Persona 1: Marco, the Independent Promoter**

Today Marco books 20-40 shows a year at mid-sized venues. He manages everything through the Dice web dashboard — pulling up event pages one at a time, manually screenshotting ticket counts, copying fan emails into a spreadsheet for his Mailchimp list. Every Monday he spends an hour doing this before sending his weekly newsletter.

Weekly ritual: Before every show, Marco exports the guest list from Dice's web UI (a PDF or CSV buried in the dashboard), cross-references it against his own spreadsheet to see who hasn't picked up tickets, and builds a "text your fans" list filtered to opt-in only.

Frustration: There is no single view across all his events. He can't see which fans bought tickets to more than one of his shows, and there's no way to quickly pull "give me all opted-in fans from my last five shows in London" without doing it venue by venue, manually.

---

**Persona 2: Priya, the Festival Marketing Manager**

Priya works for a mid-sized festival with 8-12 events per season. She is responsible for post-event audience segmentation and reporting to sponsors. Currently she relies on Audience Republic to sync Dice data every 90 minutes, waits for the sync, then exports from Audience Republic's UI. The 90-minute lag is a real problem when the festival is live.

Weekly ritual: Every Monday she pulls a revenue report per event for the CFO — total tickets sold, total gross, total fees paid to Dice, net revenue. She has to do this per-event in the dashboard and sum it in a spreadsheet.

Frustration: She cannot get financial totals across multiple events in one place without a paid integration or manual aggregation.

---

**Persona 3: Sam, the Venue Access Manager**

Sam works door at a busy venue and is responsible for reconciling the door list on the night of the show. He uses Dice's app for scanning but needs a pre-show "who's coming" list.

Weekly ritual: 2-3 hours before each show, Sam builds a door-check list: all valid ticket holders minus returned tickets, plus any transferred tickets mapped to the new holder's name.

Frustration: Dice's dashboard shows ticket holders and returns in separate tabs. There is no combined "valid holders as of now" view.

---

**Persona 4: Keisha, the Data-Driven Talent Buyer**

Keisha works at a booking agency. She wants to know which genres are drawing the best sell-through rates on Dice, which fans are buying tickets to multiple shows in the same genre, and which events have anomalously high return rates.

Weekly ritual: After each on-sale announcement, Keisha watches ticket velocity for the first 72 hours to forecast whether an event needs additional promotion.

Frustration: No programmatic access means no automation. Every query requires opening a browser, navigating the dashboard, and manually recording numbers.

---

## Candidates (pre-cut)

| # | Candidate | Command | Source | Kill check |
|---|-----------|---------|--------|------------|
| A | Door list with transfer resolution | `door list --event <id>` | (a) Sam persona | CLEAR |
| B | Cross-event repeat buyer report | `fans repeat --since <date> --min-events 2` | (a)(c) Marco/Keisha | CLEAR |
| C | Revenue summary per event and cross-event | `revenue summary --event <id>` | (a)(c) Priya | CLEAR |
| D | Ticket velocity: cumulative sales over time | `velocity show --event <id>` | (a)(c) Priya/Keisha | CLEAR |
| E | Opt-in fan list builder with geography filter | `fans optin --event <id> --country cc --csv` | (a)(b) Marco | CLEAR |
| F | Genre affinity per fan | `fans genres --all` | (b)(c) Keisha | SPECULATIVE — soft kill |
| G | Return rate anomaly report | `returns anomalies --threshold 0.05` | (a)(c) Keisha/Priya | CLEAR |
| H | Stale / orphan reconciliation | `reconcile --event <id>` | standard pattern | WRONG PERSONA — soft kill |
| I | Top spenders per event | `fans top --event <id> --n 20` | (a)(c) Marco/Priya | CLEAR |
| J | Event state pipeline view | `events pipeline` | (b) | THIN WRAPPER — kill |
| K | Sellthrough rate comparison | `events sellthrough --genre genre` | (c) Keisha | CAPACITY FIELD UNCONFIRMED — kill |
| L | Transfer chain: who transferred to whom | `transfers chain --ticket <id>` | (a) Sam | LOW FREQUENCY — kill |
| M | Fan profile: full history for one fan | `fans profile --email email` | (a)(c) Marco/Sam | FOLLOW-UP ONLY — kill |
| N | Extras/add-on revenue per event | `extras revenue --event <id>` | (a)(c) Priya | SUBCASE OF C — kill |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Door list with transfer resolution | `door list --event <id>` | 9/10 | Three-table SQLite join (tickets LEFT JOIN returns LEFT JOIN transfers) produces valid-holder list with new-holder names resolved | Brief workflow #1: "valid tickets vs. returned/transferred"; Sam persona; no single API call or dashboard view covers this |
| 2 | Cross-event repeat buyer report | `fans repeat --since <date> --min-events 2` | 8/10 | SQLite GROUP BY fans.email across distinct event_ids in tickets table, summing spend per fan | Brief workflow #5 "repeat buyers, high-value fans"; Audience Republic does this with 90-min lag; no API equivalent |
| 3 | Revenue summary (per-event and cross-event) | `revenue summary --event <id> [--from <date>]` | 9/10 | SQLite SUM over orders fields per event or across date range | Brief workflow #3 "revenue, commissions, fees, net"; Priya's explicit Monday ritual; no API aggregation endpoint |
| 4 | Ticket velocity (cumulative sales over time) | `velocity show --event <id> [--bucket day\|hour]` | 8/10 | SQLite bucketing of orders.purchasedAt relative to event on-sale date, day-by-day cumulative count | Brief workflow #4 "ticket velocity"; Keisha's explicit use; Dice dashboard shows only current snapshot |
| 5 | Opt-in fan list with geography filter | `fans optin --event <id> [--country cc] [--city str] --csv` | 8/10 | SQLite join of fans (optInPartners=true) against orders.ipCity/ipCountry, CSV output | Brief workflow #2 "demographics, geography, Mailchimp"; Marco's Monday ritual; geography join not in dashboard export |
| 6 | Return rate anomaly report | `returns anomalies [--threshold 0.05]` | 7/10 | SQLite COUNT(returns)/COUNT(orders) per event, filter above threshold, ranked | Brief workflow #3 "refund/return rates"; Keisha pricing-problem detection; no API analytics endpoint |
| 7 | Top spenders per event or across events | `fans top --event <id> [--n 20]` | 7/10 | SQLite SUM(orders.purchasePrice) GROUP BY fan_id JOIN fans ORDER BY total DESC | Brief workflow #5 "high-value fans"; Marco VIP; Priya sponsor deck; no API leaderboard |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|------------|--------------------------|
| F — Genre affinity | Speculative weekly use; cross-event genre-per-fan join may produce misleading results | fans repeat (B) |
| H — Reconcile | Wrong persona — promoters don't do data engineering; low User Pain | door list (A) |
| J — Event state pipeline | Thin GROUP BY wrapper, no cross-entity join, replicable with events list --json | revenue summary (C) |
| K — Sellthrough rate comparison | Capacity field not confirmed in spec; denominator missing without it | returns anomalies (G) |
| L — Transfer chain | Per-ticket trace, not a weekly ritual; subsumed by door list | door list (A) |
| M — Fan profile | Follow-up query to repeat buyer discovery, not standalone weekly workflow | fans repeat (B) |
| N — Extras revenue | Narrow sub-case of revenue summary; absorbed as a column | revenue summary (C) |
