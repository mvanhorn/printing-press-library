# Fix 32: partial API exits and verification consistency

Preserve underlying HTTP errors after partial metadata is emitted by imports blame and sibling partial-result commands, so documented status exit codes survive. Reject line-type verification batches unless `done == (remaining == 0)` before counting the batch. Regression tests cover HTTP 401/403/404/500 mappings and the contradictory terminal response.
