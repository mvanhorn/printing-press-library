# Maintaining Tabelog CLI

Separate source facts from user notes and membership. A refresh replaces a complete validated snapshot; merging absent fields from an older snapshot would mislabel stale facts as current. Failed refreshes retain valid snapshots and report failure.

Source changes require public English evidence and an executable regression case. Preserve unknown versus not-fetched states, separate budget sources, exact identity, source order, effective geography and retrieval times. Station vicinity is not a geometric radius. Drifted or blocked pages must not become empty success.

For extraction/transport changes, read [docs/source-contract.md](docs/source-contract.md). For notebook behavior, read [docs/saved-lists.md](docs/saved-lists.md). The normalized model is in `internal/domain`; command help owns flag definitions.

[e2e/README.md](e2e/README.md) owns executable replay/live procedures. Keep resource and token bounds alongside factual correctness. Prefer independent source oracles and meaningful failures over tests that repeat selectors.

Each handwritten command declares one `pp:data-source` strategy with a matching Cobra annotation. Honor or reject explicit source modes. Projection retains essential provenance and meaningful facts.

Drain and close SQLite rows before another query. Use one transaction for related writes; generated upsert helpers start their own transaction. Propagate persistence failures.

Preserve custom modules and integration hooks across reprints; record required customizations in `.printing-press-patches/`. Keep metadata-derived descriptions synchronized through `research.json`. Publishing owns release version stamping.
