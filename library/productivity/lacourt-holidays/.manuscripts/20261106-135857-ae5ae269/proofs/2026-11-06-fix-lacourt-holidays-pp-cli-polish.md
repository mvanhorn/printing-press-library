# Polish (printing-press-polish, forked)
Scorecard 86 -> 88; verify 100 -> 100; dogfood WARN -> PASS; gosec hand-written 2 -> 0; tools-audit 0 pending.
Fixes: ics --out-dir perms 0750/0600; removed dead helpers (endpointPositionalArgs, retainCLIQueryParams + 5 orphaned); removed unread --max-age flag; removed duplicate README troubleshooting block; patch record dead-code-and-ics-perms.json.
Skipped: holidays prints upstream MM/DD/YYYY (generated mirror); Farmworkers Day 2027 = 4/9 per court data; 54 gosec in generated framework files; structural scorecard gaps.
ship_recommendation: ship
