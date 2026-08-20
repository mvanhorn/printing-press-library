# domain-goat Novel-Features Brainstorm (audit trail)

## Customer model

**1. Jamie, the pre-launch SaaS founder at 11pm.** Has 40 tabs open across instant-domain-search, Namecheap, Porkbun, and a Google Doc with name candidates. Loses track of which ones were available 20 minutes ago vs. which got snapped up. Weekly ritual: re-checks last week's "maybe" list on Sunday night to see if anything dropped or got cheaper; copies the survivors into yet another doc. Frustration: names that scored well in his head ("snappy, six letters, .ai") turn out to be $4,800 premiums on one registrar and $12 on another, and he doesn't find out until checkout.

**2. Priya, agency creative director on a brand sprint.** Was told Monday "we need 20 candidate names for the pitch deck by Friday." Generates seed words in a whiteboard session, then her junior types each one into Domainr and screenshots the results. By Thursday they've checked 400 names across 8 TLDs and have no structured record of why they killed 380 of them. Friday hand-off — exports a CSV to the client. Half the rows have stale availability because they were checked Monday. Frustration: no way to score 400 names mechanically, no notes layer.

**3. Marco, a drop-catcher / domain investor hunting brandables.** Runs `whois` in a bash loop, parses by hand, misses drops because his cron fires at the wrong hour. Weekly ritual: Sunday — review what dropped last week, what's redeeming this week, what's queued for next. Frustration: WHOIS output is unstructured text; he has Python regex held together by hope. No persisted timeline.

**4. Ada, an LLM agent driving a name-hunt session via MCP.** Closest equivalent today is Claude/ChatGPT making up plausible names and the user discovering half are taken. Frustration: existing CLIs return formatted-for-human text. Agent has to parse. No FTS over the user's prior shortlist.

## Candidates (pre-cut)

1. `shortlist promote --top N` (c) — KEEP — composes 3 core tables
2. `budget --max-renewal $X --years 5` (c, e) — KEEP — hits Jamie's $12-vs-$4800 frustration
3. `compare <fqdn> <fqdn>...` (c) — KEEP — Priya's Friday workflow
4. `drops timeline --tld io --days 30 --min-score 7` (b, c) — KEEP — Marco's exact question
5. `why-killed <fqdn>` (c) — KEEP — pure local FTS query
6. `pricing-arbitrage [--tld .ai]` (b, c) — KEEP — surfaces structural fact
7. `siblings <fqdn>` (b, c) — KEEP — combines absorbed primitives
8. `generate --persona founder --vibe technical --tld ai,dev,io --count 50 --available-only --max-renewal 50` (a, c) — KEEP — Jamie's 11pm session in one command
9. `portfolio import` / `portfolio health` — KILL — scope creep (user vision rules out)
10. `agent-session` REPL — KILL — scope creep, duplicates --agent
11. `trademark-risk` — KILL — no free no-auth source
12. `brandability-radar` — KILL — viz with no action
13. `drop-bid-window <fqdn>` (b) — KEEP — deterministic ICANN math from RDAP events
14. `negotiate-draft` — KILL — LLM-dependent + violates "no transaction" vision
15. `tld-affinity <seed>` (b, c) — KEEP — local-only join, grounds TLD picking
16. `session replay <date>` — KILL — duplicates audit-log

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Persona | Evidence |
|---|---|---|---|---|---|---|
| 1 | Top-N finalist promotion | `shortlist promote --top N --by combined` | 8/10 | Join candidates × pricing × rdap, rank by score-price+avail, move top-N to finalist list | Priya, Jamie | Brief workflow #5 |
| 2 | 5-year true-cost filter | `budget --max-renewal 50 --years 5 --list current` | 8/10 | `registration + 4×renewal` from pricing_snapshots, filter+sort | Jamie | Workflow #6, registrar UIs hide year-2 jump |
| 3 | Side-by-side compare | `compare <fqdn>...` | 9/10 | One-row-per-domain join across every table | Priya, Ada | Build priority §8 |
| 4 | Drop-timeline by score | `drops timeline --days 30 --min-score 7 --tld io,ai` | 9/10 | Persisted RDAP events_json + score join | Marco | Workflow #4 + events_json |
| 5 | Why-killed audit | `why-killed <fqdn>` | 7/10 | FTS5 over notes+tags + last pricing/rdap | Priya | FTS5 design |
| 6 | Pricing arbitrage radar | `pricing-arbitrage --by renewal-delta` | 6/10 | Aggregate pricing by TLD, compute deltas | Jamie, Marco | Porkbun only no-auth source with both prices |
| 7 | Drop re-release window | `drop-bid-window <fqdn>` | 7/10 | RDAP pendingDelete + 5-day grace = exact UTC drop | Marco | ICANN deterministic, no CLI surfaces it |
| 8 | Seed → TLD affinity | `tld-affinity <seed>` | 6/10 | Joins tlds × pricing × suffix-semantics + historical avail | Jamie, Priya | Workflow #2 + tlds table |

### Killed candidates

| Feature | Kill reason | Closest survivor |
|---|---|---|
| Portfolio import + health | Scope creep (user vision: identify-only, no portfolio mgmt) | #1 shortlist promote |
| `agent-session` REPL | Scope creep, duplicates --agent flag | absorbed agent flags + #3 compare |
| Trademark-risk | No free no-auth source | #5 why-killed |
| Brandability radar | Viz with no action | #1 shortlist promote |
| Negotiate-draft email | LLM-dependent + violates "no transaction" | None — out of scope |
| Session replay | Thin, duplicates audit log | #5 why-killed |
