# Phase 4.85 output review: WARN (3 warnings, plus rule-predict sample failure)
Scope: empty-state output only. No populated output, relevance or ranking assessed; rerun after a real sync. Reviewed by the fork itself, not an independent agent.
1. digest and newcomer read as all clear on a never-synced store.
2. topology --agent returns {"consoles":[]} with no sync hint, indistinguishable from an empty network.
3. Raw Go durations in user output (168h0m0s for --since 7d).
4. rule-predict: blank known-zones list and no sync hint on an empty mirror; `--dst-zone wan` example may not resolve, since matching is exact id or name and the fixture calls the zone External. Live confirmation needed.
Disposition: fixes queued to the code agent, policy is fix now for 1-3 file bugs.
