# Phase 4.95 correctness and bounded I/O repairs

Preserve the generated CLI fixes that keep command-selected `--db` stores
isolated during auto-refresh, reconcile complete NameThatUI component/style
enumerations without deleting immutable snapshots, and allow `identify
--data-source live` on a fresh machine. Keep all emitted ask suggestions
POSIX-shell quoted and keep doctor/help/API-denial language accurate for a
public no-auth site.

Preserve the bounded-resource safeguards: decoded client response limits,
bounded MCP SQL time/rows/bytes (including a per-connection SQLite value limit
before Scan), capped feedback stdin, context-aware feedback delivery,
HTTPS-only public webhook validation at resolution and dial time with proxies
disabled and redirects rejected, and secure atomic file delivery using
exclusive temporary files. NameThatUI HTML responses must describe public-site
policy or bot protection rather than credentials or expired sessions. Sync
must finish fetching and parsing each authoritative component/style set before
one atomic mirror, snapshot, reconciliation, and sync-state update.
Do not loosen the existing MCP `--deliver` block or route automatic refresh to
the default store when a command explicitly selected another database.
