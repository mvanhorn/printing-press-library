## Customer model

**Mira — macro strategist at a mid-sized asset manager (NYC).**

*Today (without this CLI):* She keeps a Bloomberg terminal tab open for BLS prints, plus a personal `~/notes/cpi_series.txt` listing the BLS series IDs she's memorized over six years (CUUR0000SA0, CUSR0000SA0L1E, CES0000000001). On release mornings she opens FRED in a browser for the chart, then hits api.bls.gov from a Jupyter notebook to pull the underlying values into an Excel model her PM consumes by 9:15 AM.

*Weekly ritual:* Every Tuesday she refreshes a ~25-series macro dashboard (headline + core CPI, CPI components, payrolls, JOLTS openings, ECI, PPI finished goods) — pull latest two years, compute YoY and MoM, paste into the model. Wednesday-Friday she answers ad-hoc PM questions like "what did Cleveland CPI services do last month."

*Frustration:* Looking up a series ID she hasn't memorized. "Los Angeles CPI all items NSA" requires opening bls.gov/cpi/tables/supplemental-files/ in a browser, scrolling, copy-pasting CUURA421SA0 back into the notebook. Half her notebook's runtime is series-ID archaeology, not analysis.

**Devin — economics reporter at a national newspaper (DC bureau).**

*Today (without this CLI):* On embargo mornings he refreshes the BLS news-release HTML page at 8:30:00 sharp, ctrl-F's for the headline number, then for context queries he opens FRED. For "is this the highest unemployment rate since X" he Google-searches and trusts whatever blog comes up.

*Weekly ritual:* Monday he reviews the upcoming week's BLS release calendar (which prints when), drafts a pre-write skeleton for each release. Tuesday-Friday on release days he writes 600-word reactions in <30 minutes, needing the headline number, prior month, YoY change, and one historical comparison.

*Frustration:* The release calendar is HTML-only on bls.gov, frequently reshuffled by shutdowns. He keeps missing release reshuffles and showing up to write a story two hours early or one hour late. Second frustration: the BLS press-release page gives the headline but not the deeper components — he has to bounce to api.bls.gov for the breakdown.

**Sam — RAG/agent engineer building a "macro copilot" at a fintech.**

*Today (without this CLI):* Has hand-rolled a Python tool layer over api.bls.gov for the agent. Each tool is bespoke: `get_cpi(area, item, period)`, `get_unemployment(state)`. Every new survey is a new tool. Series-ID resolution is currently a hardcoded JSON dictionary of ~80 series IDs the team curated by hand.

*Weekly ritual:* Maintaining the tool layer. Each week a PM asks "can the agent also answer questions about JOLTS hires by industry?" and Sam writes another bespoke wrapper. Agent eval suite catches that the agent hallucinates series IDs whenever the question falls outside the curated 80.

*Frustration:* The agent can't discover series IDs. Without an MCP-native, search-backed tool, every new question requires Sam to extend the curated dictionary. He wants ONE generic tool: "find me the BLS series for X" that returns a real ID the live API will accept.

**Priya — labor-economics PhD student writing a dissertation chapter on regional wage dispersion.**

*Today (without this CLI):* Downloads QCEW annual flat files from download.bls.gov (when Akamai lets her), wrestles them in R with `blscrapeR`. For OEWS occupation-level wages she pulls one MSA at a time via the API because the series IDs are opaque (OEUM0049340000000000004).

*Weekly ritual:* Pulling a different cross-section of QCEW + OEWS + LAUS for a new regression. She regenerates her dataset ~weekly as the chapter evolves.

*Frustration:* The packed series ID format is a barrier. She knows what she wants — "average weekly wages, NAICS 23 construction, all California counties, 2019-2024" — but translating that to ~58 OEUS-prefixed series IDs is a manual lookup-table exercise every time.

## Candidates (pre-cut)

(Pasted verbatim from subagent output — 16 candidates C1-C16, four cut before Pass 3.)

## Survivors and kills

### Survivors

1. **Series search** — `series search "<query>" [--survey CPI] [--area "Los Angeles"]` — 10/10. FTS5 over local series catalog. Mira/Sam/Priya weekly.
2. **Macro snapshot** — `snapshot macro [--csv]` — 9/10. Curated ~15-series cross-survey batch with calculations. Mira weekly.
3. **Release calendar** — `releases next/watch` — 8/10. Local curated table; HTML-only on bls.gov. Devin weekly.
4. **Footnote decoder** — `footnotes decode <code...>` — 7/10. Flat-file footnote table join.
5. **Historical extremum** — `series extremum <id> --since <yr>` — 7/10. SQLite scan over cached observations.
6. **SA/NSA compare** — `series compare-sa <stem>` — 6/10. Decode packed-ID position 3.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Series ID builder | Already in absorb manifest #10 | C1 series search |
| Release watch (standalone) | Fold into `releases next --watch` | C4 releases next |
| Inflation adjustment | Already in absorb manifest #8 | — |
| QCEW cross-area pull | Fold into existing `qcew` resource (absorb #11) | absorb #11 |
| YoY/MoM passthrough | Flag on series get, not feature | C3 macro snapshot |
| Surveys stats | Doesn't pass weekly-use bar | — |
| Macro indicator alias | Subsumed by snapshot + search | C3 macro snapshot |
| Agent-mode resolver | Subsumed by MCP auto-exposure + `series search` | C1 series search |
| Release-day diff | More general extremum beats narrow diff | C8 historical extremum |
| Revision tracker | Verifiability low (needs weeks of data) | C8 historical extremum |
