# Seykota CLI — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Persona A — "Devon", the systematic trend-following trader (Buffet-System operator).**
*Today:* Runs the Gold Futures tab (already encodes a Seykota EMA-crossover signal). When he wants to sanity-check a rule he opens seykota.com, fights its 1990s frameset, often lands on the wrong FAQ month. Keeps PDFs of *Market Wizards* excerpts on disk because the site is hard to navigate.
*Weekly ritual:* Weekend position review + reading one or two strategy primary sources to decide whether to tighten stops. Seykota's risk essay and the "heat" FAQ entries are on rotation. Copies equity/entry/stop into a scratch spreadsheet to compute risk-per-trade and portfolio heat by hand.
*Frustration:* The canonical source for trend-following risk control is unsearchable, and its math (Kelly, Uncle Point, Lake Ratio, heat) is prose, not a calculator. Retypes fixed-fraction arithmetic weekly; can't cite a specific FAQ month in a write-up.

**Persona B — "Mara", the *Market Wizards* student / strategy researcher.**
*Today:* Building a research note on how the Schwager traders differ on position sizing. For Seykota she reads across 20 years of FAQ pages (day-pages pre-2010, month-pages after). The site's Google search box returns mirror sites and dead links as often as seykota.com itself.
*Weekly ritual:* Pulls primary-source quotes — exact wording + citation (which FAQ month, which TSP section). Maintains a "what each Wizard said about X" doc; needs source URL + date next to every quote.
*Frustration:* No way to ask "every place Seykota discussed whipsaws, ranked, with dates" — she scrolls. TSP rules (EA crossover, SR) spread across ~10 sibling pages with no index of what changed when. No clean list of FAQ contributors or topics.

**Persona C — "Sam", the agent/automation builder inside Buffet-System.**
*Today:* Wires strategy tabs and the Genie meta-tab. When a tab wants "Seykota's rule for this situation" as a tooltip / `why` string, there's no machine-readable source — it's a static HTML site. Hardcodes a paraphrase.
*Weekly ritual:* Adds/tweaks `extract_actionable()` outputs and tooltip copy; wants every claim to point at a citeable source. Pipes CLI JSON into scripts; expects `--json`/`--select`/exit codes to behave.
*Frustration:* No offline, structured, scriptable way to pull a Seykota quote + source URL into a pipeline. Generic web Kelly calculators aren't CLIs and don't match Seykota's exact formulation (Timid/Bold, Uncle Point) — sizing automation reinvents the math with no provenance.

## Survivors (→ novel features)

| # | Feature | Command | Score | Persona |
|---|---------|---------|-------|---------|
| 1 | Concept chronology | `timeline <query>` | 8/10 | Mara, Devon |
| 2 | Coin-Toss simulator | `risk coin-toss --win-rate --payoff --bet-fraction --trials --runs [--seed]` | 9/10 | Devon |
| 3 | Lake Ratio calculator | `risk lake-ratio --equity-curve <file\|->` | 9/10 | Devon, Sam |
| 4 | FAQ contributor index | `faq contributors [<name>]` | 7/10 | Mara |
| 5 | Metric explainer | `risk explain <metric>` | 8/10 | Devon, Mara, Sam |
| 6 | Citation-formatted search | `cite <query> [--style faq\|tsp\|risk] [--bibtex] [--json]` | 8/10 | Mara, Sam |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `risk timid-bold` | Thin — `risk uncle-point` + `risk heat` + a static lookup; band logic risks misrepresenting the essay | `risk explain` |
| `faq topics` (standalone) | Already inside absorb manifest #2 (`faq` browse-by-topic) | `faq contributors` |
| `tsp updated` | Minor — a `--sort` flag on `tsp list` | (folded into `tsp list`) |
| `tsp diff <slugA> <slugB>` | Fragile HTML overlap-detection; no surfaced demand | `timeline` |
| `risk worksheet` | Redundant with `risk heat --positions` (manifest #5) | `risk lake-ratio` |
| `risk kelly --fractional` | A flag on an already-absorbed command | `risk coin-toss` |
| `read <url-or-id>` | Generic "print any page" — convenience over `faq/tsp/risk show` | `risk explain` |
| `index build --granular` | Brief defers per-Q&A parsing as fragile, post-MVP | `cite` |
| `related <year> <month>` | "More like this" without embeddings is weak; overlaps `timeline`/`search` | `timeline` |
| `stats` | Nice-to-have corpus summary; no weekly pull | `cite` |

(Full pre-cut candidate list with rubric verdicts: see subagent transcript in run log.)
