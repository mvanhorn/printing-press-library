# Final cache-selection review fix

Automatic selection preserves a previously validated compatible pair if a later unrelated file hits the 64 MiB budget, becomes unreadable/missing, or contains malformed data. The comparison remains between complete source observations. Output explicitly reports cache_selection_complete:false and cache_selection_notes; a newer compatible pair may remain unexamined. Explicit paths select an exact pair. A stopped scan without any known pair still returns an error.

Source full tests and vet, all seven actual shipcheck legs, and fresh 171-case full live dogfood pass. The runner-owned acceptance marker was refreshed for this final Go tree. The same independent reviewer verified the Go regression and seven separate offline CLI checks, plus README/SKILL wording, and returned PASS with no remaining finding. Prior covered-scope, facility/map, origin and newest-pair ranking regressions remain passing.
