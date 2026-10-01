<!-- slop-gate: off -->
# Phase 4.85 agentic output review

Status: WARN (3 warnings, 0 errors). Live-check sampled 10 novel-feature examples with credentials loaded; 8 passed and were reviewed. The 2 excluded samples (routes approve/unapprove on `self`) ran without --yes, so they only planned; they failed the probe's literal query-token match because the output showed the device name, not "self".

Findings and fixes:
1. devices expiry labelled rows by OS hostname, so a phone showed as "localhost" and two machines shared one name. Fixed: devices are labelled by MagicDNS machine name (unique, matches the admin console); JSON gains `machine`.
2. policy restore --list printed "-" as the tailnet. Fixed: prints "the credential's own tailnet".
3. A device with key expiry disabled showed a stale past expiry date. Fixed: human table prints "disabled"; JSON omits the stale date.
Also: route plans now echo the user's `selector` so agents see "self" in output.
