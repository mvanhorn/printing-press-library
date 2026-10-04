# JAPAN47GO agent guide

Use README.md for runtime semantics and SKILL.md for the agent workflow. The four domain commands are services discover, inspect, compare and saved. Keep source reads anonymous and read-only; do not add booking, payment, messaging or browser dependencies.

Preserve bounded Japanese originals and separate source/retrieval clocks. Do not export raw provider payloads, staffing counts, guide ages, personal contacts or profiles. Missing price units, expense qualifiers, multiple deadlines, office closures and old hiatus stay explicit. Source closed=false does not prove open-now; availability remains unknown.

Discovery caps pages separately from output and reports matching inventory, scanned records and continuation. Compare only published-rule compatibility for at most five records. Saved reads stay read-only, at most 200 observations, with normal SQLite transactions for saves. Newer instants must survive delayed older saves.

For source changes, run focused service tests first and full checks once stable. Rebuild matched CLI/MCP peers and the bundle; inspect actual MCP schemas/calls and extracted bundle hashes. Keep file-scope pp:data-source markers aligned with Cobra annotations. The runtime Cobra mirror owns normalized service tools; generated raw handlers must not take precedence.

Before discovery, follow the recall/teach guidance in SKILL.md. Learning stays local; never teach personal contacts or private source payloads. Use --no-learn for deterministic verification.

Record local customization guards under .printing-press-patches. New runtime versions remain 0.0.0-dev until the public library release workflow stamps them. Preserve release ledgers; never edit generated registry.json or cli-skills mirrors. Publishing and PR writes require direct user authorization; do not merge.
