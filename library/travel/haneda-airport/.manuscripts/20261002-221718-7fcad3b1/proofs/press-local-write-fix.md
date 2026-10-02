# Isolated Press private-fixture matrix correction

Actual upstream v4.32.5 was copied from the Go module cache and built under this CLI's isolated Press home. No global binary, config or module cache was changed.

Original runner always previews mutator happy rows, then declares a legitimate local snapshot capture hollow. The isolated patch permits actual writes only when the caller explicitly passes --allow-destructive and the command opts into private pp:fixture-local-write. Happy args containing --file, --overwrite or --deliver stay previews. All remote mutators stay previews. Public MCP safety hints are unchanged; snapshot save remains read-only:false with open-world/destructive hints. mcp:local-write is not used for its arbitrary-file command.

Nine guard cases pass, including: default fixture approved/unapproved, explicit file, overwrite, external delivery, public-local-write without private permission, remote POST and read-only. Existing destructive-auth guard tests pass. Marker, JSON, coverage and fingerprint code remains unchanged.

The real default-cache fixture is disposable local state under the runner's isolated HOME. Provider requests remain anonymous reads; no accounts, reservations, payments or messages are mutated.
