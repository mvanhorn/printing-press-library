# Novel features brainstorm (subagent ae91b09a032178976, 2026-10-09)

Personas: Elena (first-time Tokyo visitor from Melbourne, converts JST by hand, conflicting blogs); Daniel (itinerary-agent operator, 5-10 client trips/week, needs cited machine-readable answers with explicit unknowns); Wei Ling (parent from Singapore, party of 4 incl. children 7 and 3, teamLab "few" ambiguity, SHIBUYA SKY child tickets counter-only, wants sunset).

Candidates: C1 plan (keep), C2 sunset (keep), C3 --party (keep), C4 next (kill: = onsale first row), C5 availability --cheapest (kill: fan-out, no decision change), C6 watch (kill: polling; announced date already absorbed), C7 changes (kill: needs store + polling), C8 verify claim (kill: LLM dependency), C9 cost (kill: no decision change), C10 ics --alarm (kill: calendar apps do it), C11 open (kill: Queue-it for webket; handoff URL already in sights), C12 sunset --weather (kill: external service, no D-14 forecast), C13 plan --route (kill: routing service, scope creep), C14 availability --city (kill: thin filter).

Survivors: plan 10/10 hand-code; sunset 8/10 hand-code; --party 7/10 hand-code. Full scoring in the absorb manifest.
