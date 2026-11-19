Book tags are plain arrays; highlight tags remain paginated objects.

Use an empty book-tags response-path hint and preserve sync decoder plain-array handling before envelope decoding. Highlight tags retain results unwrapping. The existing decoder already accepts both; keep behavioral tests of live output and sync storage.

These are endpoint-specific API contracts from the 2026-11-18 drift report. Keep this intent when reprinting; do not replace the generic shared machinery.
