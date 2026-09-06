# Scope-based itinerary reads and model evaluation

Complete reviews now read full selected days once; overview is optional orientation for focused questions. Related day reads share context and reuse route/check data. Skill, references, README, agent guide and focused task purposes agree. No itinerary fields were removed.

Paired GPT-5.6-Luna Max pilot: 12 sessions, three synthetic tasks, two repetitions per arm. Mean model-reported input+output tokens (including cached input) fell 20.7% for a three-day review, 31.0% for a seven-day review and 27.7% for creation. Uncached input rose 10.4% and 7.9% for reviews and fell 32.3% for creation; no billing savings are claimed. All exact proposals validated, but full finding coverage passed only 9/12 reports (68/72 criteria): baseline 5/6, refined 4/6. Overnight-flight reporting and one vehicle-availability constraint were omitted despite being present in CLI output. Two targeted checklist trials still omitted the flight boundary; that extra prose was reverted. This evidence does not establish lossless model reasoning or superiority across models.

All Go tests passed. Refreshed live acceptance: 272 passed, zero failed, 183 skipped/unverified. All 12 publication checks and skill verification passed; accepted source hashes match. No release version changes, global installation or publication. Raw model transcripts and local artifacts remain outside this public patch.

## Publication review hardening

Greptile identified that missing source or document identity could satisfy the acknowledgement predicate. Both identities must now be present and match the current session and target. Regression coverage rejects incomplete, blank, mismatched and unknown expected identities. Focused and full Go tests pass. A real synthetic insert and explicit journal undo both passed the stricter checks and restored the day snapshot exactly. Planning read schemas and evaluation workloads are unchanged.
