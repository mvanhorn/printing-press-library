# Fix 45: pagination progress guard

Adds a shared 100-page pagination guard that rejects zero-new-ID pages and repeated cursors. Send-check, imports blame, line-type estimates, exports, and generated all-page reads now stop safely instead of relying only on short-page termination.
