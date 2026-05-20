# Novel Features Brainstorm — granola-pp-cli

## Customer model

**Damien — the MEMO operator (founder/CEO running a daily meeting pipeline).**
- *Today:* `granola.py extract <id>` every workday against `~/Library/Application Support/Granola/cache-v6.json`; MEMO meeting-analyzer agent reads the resulting `full_<id>.md`, `summary_<id>.md`, `metadata_<id>.md` triple. Granola GUI open + MEMO repo open + terminal where he reruns `preflight` until the transcript flushes. Cannot answer "which meetings this week are still missing transcripts" without scripting per-meeting.
- *Weekly ritual:* For every recorded meeting, extract the three-file artifact, hand to analyzer, fold into Obsidian / nurture / build-session-takeaway. Once weekly a "what did we discuss with X / what's stale" sweep across last 7-14 days.
- *Frustration:* Gap between "Granola said the meeting is done" and "transcript actually exists in cache" — warm, wait, preflight, retry. Cross-meeting questions require shelling out per meeting.

**Trevin/Zac — the sales+CS attendee analyst (account lead / CSM).**
- *Today:* Granola web, scroll list, open each meeting with target attendee, copy-paste notes. "Every meeting we've had with `@acme.com` in last 60 days" = by hand.
- *Weekly ritual:* Pre-call prep on Monday + Thursday: every meeting with upcoming-call attendees, scan AI panel summary, action items, last-touch talking points.
- *Frustration:* No grep across meetings. No "who attends with whom." No way to ask "for this attendee, what AI-panel template was used and what did the last summary say" without clicking meeting-by-meeting.

**Sarah — the consultant doing weekly retros and time-defrag.**
- *Today:* CSV exports by hand or eyeballed calendar. Community Python MCP has ASCII charts but she wants raw numbers piped into spreadsheets.
- *Weekly ritual:* Friday retro — frequency by client, talk-time by participant (am I dominating?), recurring meeting cadence, "did I run the Discovery recipe on all new-prospect calls."
- *Frustration:* No "show me every meeting from last week that did NOT have the Discovery panel applied" or "for every meeting tagged `client-foo`, give me the action-items panel output as ndjson."

[Pass 2 + Pass 3 output preserved verbatim from subagent response]

(See subagent response in conversation log for full Pass 2 candidates and Pass 3 force-answers; survivors and kills are reflected in the absorb manifest's transcendence table.)
