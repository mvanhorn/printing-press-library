# Only complete unfiltered full Reader walks reconcile absent documents and descendants atomically.

Reader provides no tombstones; partial, filtered, capped and explicitly no-prune walks cannot establish absence.

This local delta protects Readwise mirror semantics after reprint. The snapshot, search, and backup changes also belong upstream in Printing Press; retain these guards until the generator provides the same contract.

Round 2: refuse malformed, empty, under-count and mass-removal snapshots with a sync_warning. Only sync exposes an explicit mass-prune override. Prune permission is required at each call site.

Prune unseen children only when the full snapshot returns at least one parent_id. Otherwise emit an explicit skipped-child-pruning summary. Book tombstones cascade existing highlights and discard nested live-looking highlights.
