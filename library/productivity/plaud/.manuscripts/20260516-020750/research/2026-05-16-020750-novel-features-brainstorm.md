# Novel Features Brainstorm — Plaud

> Full subagent output from Phase 1.5 Step 1.5c.5 (novel-features subagent). Audit trail per the subagent contract.

## Customer model

**Persona 1 — Maya, the Founder-Operator**
- **Today (without this CLI):** Records every 1:1, customer call, and exec sync on her Plaud. Opens `app.plaud.ai` after each meeting, skims the AI summary, mentally notes follow-ups, sometimes copies a sentence into Notion. Forgets ~half within 48 hours.
- **Weekly ritual:** Sunday-night planning. Tries to remember "what did I promise people this week?" Scrolls Plaud's recording list by date, opens each one, ctrl-F's the transcript for "I'll" or "I will." Gives up after 4 recordings. Falls back to her inbox.
- **Frustration:** Plaud knows every commitment she made but can't surface them as a list. The app is built for one-recording-at-a-time consumption; her question is cross-recording.

**Persona 2 — Jordan, the Account Executive**
- **Today (without this CLI):** Records discovery + demo + renewal calls. Has 80+ recordings tagged by customer (filetag). Lives in HubSpot for pipeline but truth-of-conversation is in Plaud. When prepping a renewal call he opens the customer's filetag, plays back the last call at 2x, takes notes.
- **Weekly ritual:** Pre-call prep, 4-8 times a week. For each upcoming call: "What did this person say last time, on this topic? Did I commit to anything? Have I been consistent?"
- **Frustration:** No way to ask "what did Sandra say about pricing across our last 5 calls?" Speaker diarization exists per-recording but not as a query plane. Recording IDs are not the unit he thinks in — people are.

**Persona 3 — Sam, the PhD/Researcher**
- **Today (without this CLI):** Records every interview, lab meeting, and advisor sync. Exports to Obsidian via leonardsellem's tool. Tags recordings by project. Grep's the vault.
- **Weekly ritual:** Monthly thematic review — "what topics are emerging in my advisor conversations vs. fading? Which interview subjects keep returning to the same idea?"
- **Frustration:** Obsidian + grep gives him word-frequency, not temporal trajectory. He can find every mention of a term, but can't see whether it's growing or dying as a topic over time, or which speakers (subjects) anchor it.

**Persona 4 — Priya, the Manager**
- **Today (without this CLI):** Records every 1:1 with her 7 reports. Plaud's summary gives her this-meeting decisions. She maintains a separate doc per report with running threads.
- **Weekly ritual:** Before each 1:1, skims the last 1:1 with that person. Quarterly: "who haven't I been intentional with? Who am I overlooking?"
- **Frustration:** No silence detector. No "Bob and I haven't talked in 3 weeks, here's what was open last time we did." The recording she needs is the absence of one.

## Candidates (pre-cut)

[See parent SKILL.md output — 20 candidates, C1-C20, with kill/keep verdicts inline]

## Survivors (8) and kills (12)

8 survivors, all >= 7/10. Killed 12 candidates with one-line reasons recorded. See transcendence table in the absorb manifest for the survivors. Killed candidates table preserved in the parent transcript:

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C10 Decision ledger | Thin projection of `summaries.decisions` already populated by sync — one SQL query; users can ask via `sql`. | S1 commitments |
| C11 Action-item inbox | Same pattern as C10 — projection of `summaries.action_items` already in store. | S4 forgotten + S1 commitments |
| C12 Markmemo digest | One-flag filter (`is_markmemo=1`) — better as `recordings list --memos` flag. | Absorbed list/filter |
| C13 Scene-mix audit | Dashboard widget; weekly-use unclear; persona pull weak. | S2 topic / S5 themes |
| C14 Speaker rename merger | Maintenance/write command, not a query — folded as flag of `speakers list`. | None (operational) |
| C15 Quote finder | Duplicate of absorb #15 `search` with FTS5 phrase syntax. | Absorbed `search` |
| C16 Calendar agenda | External service (system calendar) outside Plaud API. | None |
| C17 Followup daemon | Scope creep — persistent process; one-command version is S4. | S4 forgotten |
| C18 Thought-partner replay | Speculative; depends on Phase 1.7 sniff. Defer. | S3 about |
| C19 Person-graph dump | Visualization-shaped; data reachable via `sql`. | S7 silence + raw `sql` |
| C20 Question extractor | Speculative persona pull; semantically redundant with S3. | S3 about |
| C9 With-person ledger | Mostly duplicates S3+S7 in combination. | S3 about (empty topic) |
