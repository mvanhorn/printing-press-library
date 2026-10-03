## Customer model

- **Personal errand delegator:** Today they hand an assistant a task such as calling a restaurant, then keep the returned ID and check status/transcript. Weekly ritual: delegate an occasional phone errand and verify its outcome. Frustration: creation and outcome review can feel like separate API operations.
- **Bland agent operator:** Today they send task prompts and inspect Bland call logs while iterating on an agent. Weekly ritual: run calls, monitor completion, and review transcripts. Frustration: tracking a set of calls by task wording is awkward if the work is spread across API and local notes.

## Candidates (pre-cut)

| Candidate | Source | Verdict |
|---|---|---|
| Task-to-outcome runner | Persona-driven | Keep narrowly as an opinionated create/wait/result flow; overlaps official `--wait`, hence modest score. |
| Local task recall | Persona-driven | Keep narrowly for locally cached calls; usefulness is plausible but evidence is limited. |
| Watch with repeated event updates | Service workflow | Kill; official CLI events and MCP wait cover the core, and a long-running watcher expands scope. |
| Search all historical account calls | Cross-entity | Kill; implies account-wide sync beyond the requested scope. |
| Outcome analytics dashboard | Persona-driven | Kill; no demonstrated weekly reporting need and too broad for this print. |
| Failure-only report | Persona-driven | Kill; overlaps outcome analytics and lacks demonstrated demand. |
| Stop by natural-language task | Service workflow | Kill; selecting a potentially consequential live call ambiguously is unsafe. |
| Compare task prompt variants | Persona-driven | Kill; no evidence user wants prompt experimentation. |

## Survivors and kills

### Survivors

| Feature | Command | Score | Persona served | Buildability proof | Buildability | Long Description |
|---|---|---:|---|---|---|---|
| Task-to-outcome runner | calls task run | 5/10 | Personal errand delegator | Uses POST /v1/calls then polls GET /v1/calls/{call_id} and stores the real returned record, without external dependencies. | hand-code | none |
| Local task recall | calls search-task | 5/10 | Bland agent operator | Searches locally cached task text and returned call details in SQLite, without external dependencies. | hand-code | none |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Watch with repeated event updates | Official CLI/MCP already cover waiting and events; persistent watching increases scope. | calls task run |
| Search all historical account calls | Requires unrequested account-wide sync. | calls search-task |
| Outcome analytics dashboard | No demonstrated reporting need; too broad. | calls search-task |
| Failure-only report | Overlaps an unsupported analytics workflow. | calls search-task |
| Stop by natural-language task | Could stop the wrong live call. | none |
| Compare task prompt variants | No user evidence for experimentation. | calls task run |
