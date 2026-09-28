# Source integration

Public English behavior was verified in September 2026 with plain Go HTTP, without cookies or a browser. The source spec and executable fixtures retain concrete routes; re-verify changed behavior against the source.

- Suggestions require public AJAX JSON Accept, X-Requested-With XMLHttpRequest, and the English homepage Referer. A 400 without these is not an auth requirement.
- Suggestions mix geography, stations, restaurants and taxonomy levels. Numeric id_in_datatype differs from site_name and area codes. Preserve type and ambiguity.
- More filters loads dynamically. Its budget controls are absent from static markup. Map user yen thresholds to source indexes and verify active constraints.
- Follow actual Next links, retaining filters and validating English Tabelog URLs. Station vicinity can cross administrative area paths; validate effective source selection.
- Parse per card. Review counts, ranks and award years differ. Meal icons distinguish budgets. Missing values remain unknown.
- Detail JSON-LD uses ratingCount in observed pages. The Address table row can be more complete. Listed and review-based budgets differ; access prose may mention another station.
- Source IDs can have seven digits, as observed in Hokkaido. Explicit relocated/closed notices identify historical pages even when retrieval is fresh; absence of a notice does not establish that a venue is open.
- Preserve a stored full-detail snapshot and its original retrieval time when a later listing summary arrives. A new validated detail fetch replaces that snapshot completely; avoid mixing old fields into a new snapshot.
- Validate completely before replacing stored snapshots. Recognized zero-results pages, drift, challenges and unrelated HTML have different outcomes.

Runtime stays independent of browser discovery tools. Fixtures omit ephemeral tokens, telemetry and unrelated review bodies while retaining structural evidence.
