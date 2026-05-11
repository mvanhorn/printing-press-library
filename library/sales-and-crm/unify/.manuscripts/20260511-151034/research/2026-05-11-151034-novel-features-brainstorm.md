# Novel Features Brainstorm — Unify CLI

> Source: Phase 1.5 Step 1.5c.5 subagent (general-purpose), invoked 2026-05-11.
> Inputs: brief at `2026-05-11-151034-feat-unify-pp-cli-brief.md`, rubric at
> `references/absorb-scoring.md`, prior research: none (first print).

## Customer model

### Persona 1: Nate — Revenue Ops engineer running account qualification builds at Gladly

**Today (without this CLI):** Nate runs the retail account-qualification scoring pipeline against Unify. To check whether a score build is sane, he writes one-off Python that loops over a CSV of domains, calls `find_unique` for each, dumps to JSON, then opens it in jq or Excel. To know "which retail companies have employee_count >= 200 with an opportunity in the last 30 days," he has no API path — he writes a Python script that hammers `find-unique` per known domain and joins client-side. Tabs open: api.unifygtm.com docs, the Python SDK GitHub readme, a Salesforce report, a Notion doc with the current scoring rules, and a terminal running a scratch `.py`. He cannot answer "what changed in our Unify schema since last Friday?" without diffing two JSON dumps by eye.

**Weekly ritual:** Monday 9:30 CT meeting with Emily and the Unify team. Before that meeting he runs the score build, spot-checks 5–10 retail accounts, and prepares the "what's the delta vs last week" report. Across the week he ingests CSVs of target accounts from marketing into Unify via upsert.

**Frustration:** There is no `LIST` and no `SEARCH` for records. Every read workflow is a hand-rolled script. He cannot answer ad-hoc "show me companies where X and Y" questions in the meeting — he has to go away, write Python, come back.

### Persona 2: Emily — Salesforce-aligned sales-ops admin

**Today (without this CLI):** Emily owns the Salesforce-mirrored object schema in Unify (account, contact, lead, opportunity, etc.). When she adds a new SF field, she needs to mirror it as a Unify attribute and propagate to scoring. Today she clicks through the Unify web UI, copies attribute names into a spreadsheet, and pings Nate in Slack: "I added 4 fields, can you re-run scoring?" She has no way to answer "what attributes did I change last week?" without remembering or re-clicking.

**Weekly ritual:** Adds/edits Salesforce-mirrored attributes in Unify, validates select-option pick-lists match the Salesforce picklists, and hands off to Nate for the score build. Mondays she has to enumerate the week's schema changes verbally.

**Frustration:** No schema-change audit log. She can't prove what changed between two points in time, and Nate can't act on her changes without a re-discovery pass.

### Persona 3: An AE / outbound operator running the ~3,400/day sequencing rig

**Today (without this CLI):** AEs use Unify to push contacts into sequences. They need to know "is gladly.com already a Unify company? does it have an opportunity? who owns it?" before they touch a sequence. Today they alt-tab to the Unify UI and search; if the contact isn't there, they have no fast way to upsert from a CSV without bothering Nate.

**Weekly ritual:** Daily — vet a target list of ~200 domains before launching a sequence. Spot-check 5–10 by hand. Once a week, push a fresh batch of 1–2k contacts via CSV upsert.

**Frustration:** The vetting step is one-domain-at-a-time clicking. A batch "do these 200 domains exist as Unify companies, and which have opportunities" answer is impossible without engineering help.

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C9 Upsert merge-mode shortcut (`--mode`) | Already in absorb manifest #21 — duplicate, not novel | #7 import-csv --plan |
| C10 `--ref` polymorphic shorthand | Thin wrapper picking among three already-supported request shapes; weekly use is "depends, only on record-create-with-refs"; fits Priority 3 polish | #7 import-csv --plan |
| C11 Effective validation mode in dry-run | Output polish on an existing flag; fails wrapper-vs-leverage and weekly-use checks | #5 schema diff |
| C12 Stale records report | Pure subset of `unify sql "... WHERE last_activity_at < ..."`; coverage already surfaces matched-but-stale | #2 sql + #3 coverage |
| C15 Coverage by segment (standalone) | Merged into #3 coverage as a `--by` flag | #3 coverage |
| C16 NL-to-SQL helper (`unify ask`) | LLM dependency per kill-check rubric; mechanical reframe is exactly `unify sql` | #2 sql |
