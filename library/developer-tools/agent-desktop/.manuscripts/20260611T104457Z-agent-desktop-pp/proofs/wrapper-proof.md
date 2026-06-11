# Agent Desktop Printing Press Wrapper Proof

Run ID: `20260611T104457Z-agent-desktop-pp`

The wrapper proof is scoped to the Printing Press bridge. It does not claim live desktop automation coverage; the real Rust CLI owns UI automation behavior.

Planned quick checks:

1. `agent-desktop-pp-cli --help`
2. `agent-desktop-pp-cli --version`
3. `agent-desktop-pp-cli info`
4. `agent-desktop-pp-cli install --dry-run`

The publish validator separately runs Go build, Go vet, govulncheck, help/version smoke tests, and SKILL.md static verification.
