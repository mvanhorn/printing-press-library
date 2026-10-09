# japan-timed-tickets Absorb Manifest (2026-10-09)

Tools found: no CLI, MCP server, npm/PyPI package or GitHub script for Ghibli Museum, SHIBUYA SKY or teamLab ticket sales. One web tool: japanactivity.com "Japan Booking Deadline Planner" (13 bookings; JST->local; ICS; checklist; precision labels; teamLab only "start checking ~2 months out").

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Next-release date per attraction for a visit date (exact rule) | japanactivity.com Japan Booking Deadline Planner (web) | japan-timed-tickets-pp-cli onsale | Rule confirmed against live official text each run; source URL + fetched_at; explicit exceptions |
| 2 | JST -> traveler local time conversion | japanactivity.com planner | (behavior in japan-timed-tickets-pp-cli onsale) --tz IANA zone adds local time beside JST | Scriptable, any IANA zone |
| 3 | Trip-range timeline of all releases | japanactivity.com planner | (behavior in japan-timed-tickets-pp-cli onsale) --from/--to over multiple sights, sorted by action time | Agent JSON, one call |
| 4 | Calendar file (.ics) of releases | japanactivity.com planner | (behavior in japan-timed-tickets-pp-cli onsale) --ics output | Standard ICS with source URL in each event |
| 5 | Precision labels (exact rule vs planning reminder) | japanactivity.com planner | (behavior in japan-timed-tickets-pp-cli onsale) confidence field: exact / announced / estimated / unknown | teamLab gets the announced date from source instead of "check ~2 months out" |
| 6 | Source links + checked date | japanactivity.com planner | (behavior in japan-timed-tickets-pp-cli onsale) sources[] with url + fetched_at | Live fetch, not a static checked date |
| 7 | Official per-date calendar status (open / few / sold out / closed) | teamLab ticket sites, DMM store calendar (web UI) | japan-timed-tickets-pp-cli availability | Several venues and dates in one call |
| 8 | Official per-slot inventory | teamLab ticket sites, DMM time list (web UI) | (behavior in japan-timed-tickets-pp-cli availability) --slots | stock counts, few/sold-out explicit, fetch timestamp |
| 9 | Admission prices / dynamic price | official sites | (behavior in japan-timed-tickets-pp-cli availability) per-slot adult price; (behavior in japan-timed-tickets-pp-cli sights) static price table | Web vs door price for SHIBUYA SKY |
| 10 | Museum closure calendar | ghibli-museum.jp calendar (web) | (behavior in japan-timed-tickets-pp-cli availability) closed days reported for Ghibli | Closed is distinct from sold out and unknown |
| 11 | Where/how to buy (channel, membership, phone needs) | official ticket pages, Lawson Ghibli page | japan-timed-tickets-pp-cli sights | Bilingual names, channel requirements, handoff URL |
| 12 | Source health check | printing-press convention | japan-timed-tickets-pp-cli doctor | Shows Queue-it/login boundaries as expected states, not errors |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Trip decision grid (book now / wait until X / drop date, per sight x date) — score 10/10 | (behavior in japan-timed-tickets-pp-cli onsale) state + reason fields (was `plan`; folded into onsale at Gate 1.5) | hand-code | Joins three different sale rules (Ghibli 10th 10:00 JST + Lawson 市民デー exceptions; SHIBUYA SKY D-14 00:00 JST; teamLab Calendar_Period + unpublished_calendar_notice; DMM release note) with live day status (teamLab stocks-statuses, DMM calendar classes, Ghibli closure calendar) into one action state per cell | Use this command to decide, per sight and visit date in a trip range, whether to book now, wait for a sale moment, or drop the date. Do NOT use it for 30-minute slot stock; use 'availability --slots' instead. Do NOT use it to export sale moments to a calendar; use 'onsale --ics' instead. |
| 2 | SHIBUYA SKY sunset slots with price tier and sale moment — score 8/10 | (behavior in japan-timed-tickets-pp-cli onsale) sunset object on shibuya-sky rows (was `sunset`; folded into onsale at Gate 1.5) | hand-code | Solar calculation for Shibuya joined with the 20-minute slot grid, the 15:00 price split (web 2,700/3,400; door 3,000/3,700) and the D-14 00:00 JST rule; availability explicit unknown (Queue-it) with handoff URL | Use this command to choose a SHIBUYA SKY entry slot around sunset and to learn when that date goes on sale. Do NOT use it for teamLab or Ghibli slot status; use 'availability' instead. |
| 3 | Party-size fit — score 7/10 | (behavior in japan-timed-tickets-pp-cli availability) --party adults=N,children=N,infants=N | hand-code | Compares teamLab stock_count / DMM instock with party size; applies SHIBUYA SKY child counter-only same-day rule and Ghibli age rules | none |

Killed: next, availability --cheapest, watch, changes, verify, cost, ics --alarm, open, sunset --weather, plan --route, availability --city (reasons in 2026-10-09-101047-novel-features-brainstorm.md).

### Stubs
None.

## Gate decision (Phase Gate 1.5)

Approved 2026-10-09 by the orchestrator (the user delegated absorb-gate approval to the orchestrator on 2026-10-09): option 1 with cuts (a), (b), (c), plus `sunset` folded into `onsale`. `--ics` kept.

### Approved build scope (source of truth for Phase 3)
| # | Feature | Our Implementation |
|---|---------|-------------------|
| A1 | Sight registry, bilingual names, channels, rules, handoff URLs | japan-timed-tickets-pp-cli sights |
| A2 | Sale moment per sight x visit date, JST + --tz local, --from/--to, confidence (exact/announced/estimated/unknown), sources[] + fetched_at | japan-timed-tickets-pp-cli onsale |
| A3 | Folded plan state per row once window is open: book-now / few / opens-at / closed / sold-out / no-general-sale / unknown (+reason) | (behavior in japan-timed-tickets-pp-cli onsale) state + reason fields |
| A4 | SHIBUYA SKY rows: computed sunset time, target 20-min slot(s), price tier at 15:00 split, D-14 00:00 JST sale moment, availability unknown (Queue-it) + handoff URL | (behavior in japan-timed-tickets-pp-cli onsale) sunset object on shibuya-sky rows |
| A5 | ICS export, one event per sale moment with source URL | (behavior in japan-timed-tickets-pp-cli onsale) --ics |
| A6 | Per-date status (open/few/sold_out/closed/unknown), Ghibli closure days separate | japan-timed-tickets-pp-cli availability |
| A7 | 30-min slot stock for teamLab venues only | (behavior in japan-timed-tickets-pp-cli availability) --slots |
| A8 | Party fit filter (stock >= party size; SHIBUYA SKY child counter-only flag; no price totals) | (behavior in japan-timed-tickets-pp-cli availability) --party |
| A9 | Source reachability with expected Queue-it/login boundaries | japan-timed-tickets-pp-cli doctor |

teamLab venues: teamlab-planets (DMM), teamlab-borderless (Azabudai), teamlab-kyoto (Biovortex), teamlab-botanical-osaka, teamlab-okinawa (Future Park). Dropped: Forest Fukuoka, Field host, Ticket Pia, separate `plan` and `sunset` commands, Ghibli price totals.
Lawson Ticket: honest identifying non-browser User-Agent; never spoof a browser. No Browser Use runtime, no venv.
