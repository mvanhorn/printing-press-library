# Publish review fixes, round 1

Greptile review of the initial publication head identified two product comparison gaps and one HTTP header deadline gap. All three were reproduced or directly confirmed and fixed.

- Hayatoku3 now excludes non-Green classes for routes wholly within Tokaido/Sanyo. Pure Kyushu ordinary classes and unresolved Sanyo–Kyushu transfer conditions retain confirmation requirements.
- Requested train categories are checked against every resolved route, including multiple corridors. Tokyo–Hakata Tsubame is excluded; Kyushu segment coverage still does not confirm a through service.
- The MCP HTTP listener has a five-second header-read deadline. A real TCP regression sends incomplete headers and verifies server closure before the independent eight-second client deadline, with no handler execution.

Focused domain and MCP tests passed. Ten route/class boundary cases preserve unknown discount prices, inventory and remaining eligibility. Full publish live acceptance is refreshed after these source changes; its automatic marker is the authoritative source binding.
