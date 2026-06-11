# Agent Desktop Printing Press Absorb Manifest

## Source Signals

- Existing project: `lahfir/agent-desktop`
- Existing binary name: `agent-desktop`
- Existing npm distribution: `agent-desktop`
- Existing release automation: GitHub release assets plus npm postinstall
  download and checksum verification.

## Catalog Shape

- API slug: `agent-desktop`
- Binary: `agent-desktop-pp-cli`
- Category: `developer-tools`
- Auth: none
- Commands: `install`, `doctor`, `info`, `run`

## Non-goals

- Do not reimplement the Rust command surface in Go.
- Do not silently download or update the real binary during pass-through use.
- Do not require desktop permissions for Printing Press catalog validation.

## Accepted Proof Scope

Phase 5 proof covers the wrapper behavior: buildability, install dry-run,
diagnostics, help/version output, and static skill verification. Live desktop
automation remains covered by the upstream `agent-desktop` project.
