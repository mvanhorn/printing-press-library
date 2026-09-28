# Novel Features Brainstorm (inline, 2026-09-27)

Run inline rather than as a Task subagent: this build ran inside a delegated session that may not spawn subagents. Same passes: customer model, candidates, adversarial cut.

## Customer model
1. Ada, AI agent builder. Wires search into an agent loop and wants grounded, cited context from several verticals in one call, at a small, predictable credit cost.
2. Sol, SEO consultant. Checks where client domains rank for a keyword list in specific countries, and needs to see what moved since last week.
3. Rin, analyst or journalist. Tracks a topic across web, news and papers and wants a readable brief with sources, not three raw JSON blobs.

## Candidates (pre-cut)
| # | Candidate | Source | Verdict | Long Description |
|---|-----------|--------|---------|------------------|
| C1 | rank: position of a domain for a query and location | Sol | KEEP | Use this command to find where one domain ranks for a query. Do NOT use it to list all results; use 'web' instead. |
| C2 | serp diff: entered/left/moved URLs versus the last stored run | Sol, serply-inc/notifications | KEEP | Use this command to see what changed in a SERP since the last run. Do NOT use it for a one-off search; use 'web' instead. |
| C3 | research: web + news + scholar fan-out into one cited brief | Ada, Rin | KEEP | Use this command for a multi-source cited brief on a topic. Do NOT use it when one vertical is enough; call 'web', 'news' or 'scholar' directly. |
| C4 | bulk rank over a keyword file | Sol | KILL | none |
| C5 | geo compare: same query across several proxy locations | Sol | KILL | none |
| C6 | credit meter: local count of calls spent | Ada | KILL | none |
| C7 | People Also Ask harvester | Sol | KILL | none |

## Survivors and kills
### Survivors
| # | Feature | Command | Score | Persona | Buildability | Proof | Long Description |
|---|---------|---------|-------|---------|--------------|-------|------------------|
| 1 | Domain rank check | rank | 8/10 | Sol | hand-code | One /v1/search call plus a domain match over results[].link; no single API call answers "what position is my domain". | Use this command to find where one domain ranks for a query. Do NOT use it to list all results; use 'web' instead. |
| 2 | SERP change diff | serp diff | 8/10 | Sol | hand-code | Needs a locally stored prior snapshot of the same query and location; the API is stateless. | Use this command to see what changed in a SERP since the last run. Do NOT use it for a one-off search; use 'web' instead. |
| 3 | Cited research brief | research | 7/10 | Ada, Rin | hand-code | Three concurrent vertical calls, URL dedupe, and one numbered citation list; no endpoint combines verticals. | Use this command for a multi-source cited brief on a topic. Do NOT use it when one vertical is enough; call 'web', 'news' or 'scholar' directly. |

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| bulk rank over a keyword file | Loops over many queries spend credits fast; a shell loop over rank covers it | rank |
| geo compare across locations | One call per location multiplies cost; rank --location covers the single case | rank |
| credit meter | Serply's dashboard already shows balance; local counts drift from the billing truth | none |
| People Also Ask harvester | Response field is not documented as stable across verticals | web |
