# cloro-pp-cli polish

Run inline with the deterministic Printing Press tools, not the forked polish skill.

- `tools-audit`: no findings (every command has a usable Short; read commands carry `mcp:read-only`).
- `polish --remove-dead-code --dry-run`: lists generated framework helpers that this CLI does not call (for example `retainCLIQueryParams`, `resolveRead`). Left as is: they are template code shared by every printed CLI, and removing them would make a later regen drift.
- verify 42/42, scorecard 92/100 before and after (no source change in this phase).
- Docs: README and SKILL text was checked for the engines this CLI covers; no other engine is named.

ship_recommendation: ship
