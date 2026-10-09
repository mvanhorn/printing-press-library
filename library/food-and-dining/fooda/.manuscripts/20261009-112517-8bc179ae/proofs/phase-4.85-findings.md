# Phase 4.85 output review (live, manual)
- week-ahead grouped multi-day delivery events by start date (past dates): fixed to use deliveryWindow service date.
- restaurants emitted capitalized keys, null fields, trailing whitespace: fixed.
- menu-search used event id instead of url S-id and only scanned today: fixed (url path, --days, --max-events).
- Residual: orders list parses only first page; whoami limited by Cloudflare on /settings/profile.
