# Novel features brainstorm (subagent output, condensed audit trail)
## Customer model
- Dana, homelab importer: weekly CSV imports from two non-bank-sync banks; frustration = post-import cleanup (duplicate txns, payee spelling variants), unsure backups restore.
- Marcus & Priya, shared-envelope household: #template/#goal notes; frustration = "what changed since Sunday", template targets vs budgeted invisible.
- Lee, scripting/agent power user: Node scripts break each release; 150+ rules, half suspected dead; wants no-Node SQL.
## Candidates (pre-cut)
C1 audit payees; C2 audit rules; C3 audit schedules; C4 ledger duplicates; C5 categorize suggest; C6 templates status; C7 changes since; C8 ledger sql; C9 budget overspent; C10 forecast; C11 mirror backup --keep; C12 mirror diff; C13 ledger uncleared; C14 audit transfers; C15 audit splits; C16 audit all.
## Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| budget overspent | filter over report budget-vs-actual; name collides with generated budgets | templates status |
| forecast | needs recurrence engine >200 LoC, no evidence (4/10) | audit schedules |
| mirror backup --keep | thin wrapper over mirror pull + pruning | changes since |
| mirror diff | needs two snapshots; coarser than CRDT log | changes since |
| ledger uncleared | expressible via ledger sql; no evidence | ledger sql |
| audit transfers | niche single canned query | ledger sql |
| audit splits | no evidence; hard to verify | ledger duplicates |
| audit all | aggregator, no new logic | audit payees |
