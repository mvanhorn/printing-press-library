# eero Printing Press research brief

## Scope

Create an unofficial, read-first eero CLI from the community API model in `erikh/eero`, using Printing Press generated Go and MCP surfaces. The package is intended for shell operators and agents inspecting network health, eero nodes, connected clients, diagnostics, profiles, activity, and speed-test history.

## Reference implementations

- [dciobanu/eero-cli](https://github.com/dciobanu/eero-cli) — Go command ideas including verification, profiles, guest network, filtering, and monitoring.
- [fulviofreitas/eeroctl](https://github.com/fulviofreitas/eeroctl) — noun-first UX, JSON/noninteractive output, keyring/session-token automation, and mutation safety patterns.
- [erikh/eero](https://github.com/erikh/eero) — typed Rust API model used as the primary endpoint and response-shape reference.

## Decisions

- Keep the first public package read-only. Reboot, blocking, guest Wi-Fi, forwarding, and other mutations are deferred until live authenticated proof exists.
- Use the eero session cookie model. `EERO_SESSION_TOKEN` is the non-interactive environment variable; browser-cookie login is the local setup path.
- Do not copy reference implementation code. This package contains generated Printing Press code and cites the references only as research inputs.

## Verification boundary

The generated code and mock/spec checks can run without Erik's eero credentials. The live Phase 5 matrix is recorded as a cookie-auth harness skip when no browser session is injected. No token, cookie jar, HAR, or account-specific data belongs in this manuscript or PR.
