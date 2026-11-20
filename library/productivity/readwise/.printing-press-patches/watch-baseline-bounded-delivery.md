# Safe watch initialization and bounds

Baseline all current matches at creation, including collections larger than a delivery batch. Explicit --include-existing leaves them eligible. Limit document batches to 200, report deferred matches, and never mark deferred IDs. Encoded webhook bodies above 256 KiB fail rather than exporting an unbounded body. This is downstream saved-watch policy.
