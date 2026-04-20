---
description: "Printing Press CLI for Yahoo Finance. Quotes, charts, fundamentals, options chains, and a local portfolio/watchlist tracker against Yahoo Finance — no API key, with Chrome-session fallback for rate-limited IPs Trigger phrases: 'install yahoo-finance', 'use yahoo-finance', 'run yahoo-finance', 'Yahoo Finance commands', 'setup yahoo-finance'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
---

Invoke the `pp-yahoo-finance` skill with the user's arguments: $ARGUMENTS

If the user passes no arguments or just `help`, show `yahoo-finance-pp-cli --help`. If the user's arguments start with `install`, follow the skill's install flow. Otherwise, map the arguments to the best `yahoo-finance-pp-cli` command and execute.
