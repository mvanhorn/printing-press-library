<!-- slop-gate: off -->
# Novel features brainstorm (subagent audit trail)

Subagent ran Pass 1 (customer model: founder-operator, community ops agent, course author),
Pass 2 (15 candidates), Pass 3 (cut). Survivors: spaces apply 10, spaces pull 7, course push 9,
course pull 8, seats 9, founding 8, onboard 9, show-tell schedule 8, show-tell recap 8, inbox 10.
Killed: members quiet (no evidence, 4/10), course progress (no read endpoint), quota (no usage
endpoint; local counter would be wrong), post draft (duplicates course push / recap --post),
revenue (sibling of seats).

Gate decision: owner pre-approved list (plan 2026-11-05) = spaces apply, course push/pull, seats,
founding, onboard, show-tell schedule/recap, inbox, sync/sql/search (framework). `spaces pull`
survived the cut but is not on the approved list; deferred, reported to owner.

API facts surfaced: event duplicate takes only space_id, so each occurrence needs PUT starts_at;
live rooms have no event foreign key (match by live_at window or --room); Circle post default
status is published, so recap --post must send status draft.
