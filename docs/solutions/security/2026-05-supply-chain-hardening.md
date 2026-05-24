# Supply-chain hardening for printing-press-library and cli-printing-press

_Finding date: 2026-05-17_

## Summary

Four real-world supply-chain attack campaigns in March–May 2026 motivated a set of PR-time defenses across this repo and its upstream generator. The defenses combine Greptile rules (judgment) with a deterministic Python scan workflow (mechanical block gates). This document captures the incident timeline, the attack shapes that drove each defense, and the rule-to-incident mapping.

The plan that landed these defenses: [docs/plans/2026-05-17-001-feat-supply-chain-hardening-plan.html](../../plans/2026-05-17-001-feat-supply-chain-hardening-plan.html). Operational pointers for contributors live in [AGENTS.md](../../../AGENTS.md) under "Supply-chain hardening".

## Incident timeline

### Axios (March 31, 2026)

A maintainer's npm account was compromised. Two malicious patch releases (1.14.1 and 0.30.4) were pushed; the only meaningful change to `package.json` was a new dependency, `plain-crypto-js@4.2.1` — a name-squat on the legitimate `crypto-js` library. The dependency was never imported by Axios code; its sole purpose was a `postinstall` script that dropped a cross-platform RAT (macOS, Windows, Linux), contacting a live C2 server and delivering platform-specific second-stage payloads.

The packages were live for ~3 hours on a dependency with 83M weekly downloads.

Microsoft Threat Intelligence and Google Threat Intelligence Group attributed the campaign to North Korean state actors (Sapphire Sleet / UNC1069).

**Defense applied here:** R5 — block any PR that adds `preinstall`, `postinstall`, or `prepare` to `npm/package.json`. The npm wrapper (`@mvanhorn/printing-press`) is the highest-blast-radius artifact in this repo; compromising it reaches every user.

Primary sources:
- [Datadog Security Labs — Compromised axios npm package delivers cross-platform RAT](https://securitylabs.datadoghq.com/articles/axios-npm-supply-chain-compromise/)
- [Microsoft Security Blog — Mitigating the Axios npm supply chain compromise](https://www.microsoft.com/en-us/security/blog/2026/04/01/mitigating-the-axios-npm-supply-chain-compromise/)
- [Google Cloud Threat Intelligence — North Korea–nexus threat actor compromises widely used Axios npm package](https://cloud.google.com/blog/topics/threat-intelligence/north-korea-threat-actor-targets-axios-npm-package)
- [Axios GitHub — Post-mortem issue #10636](https://github.com/axios/axios/issues/10636)

### TanStack mini-Shai-Hulud (May 11, 2026)

84 TanStack npm artifacts were compromised. The attacker exploited the GitHub Actions `pull_request_target` "Pwn Request" pattern — a workflow with `pull_request_target` trigger that also checks out the PR head ref runs untrusted PR code with the base context's secrets and OIDC tokens. Cache poisoning across the fork-to-base trust boundary, OIDC token extraction from runner process memory, and publication of malicious versions with valid SLSA L3 Sigstore attestations followed.

The payload (`router_init.js`, 2.3 MB obfuscated) harvested credentials from GitHub Actions, AWS (IMDS, Secrets Manager, SSM), HashiCorp Vault, and Kubernetes; persisted via `.claude/` and `.vscode/` directories; and self-propagated through OIDC federation to mint additional npm publish tokens.

The campaign reached devices of two OpenAI employees, forcing macOS updates.

**Defenses applied here:**
- R1 — block any new `pull_request_target` workflow that overrides the checkout ref to `github.event.pull_request.head.*` or `refs/pull/<n>/{merge,head}`.
- R2 — block `id-token: write` in any workflow outside the `npm-publish.yml` allowlist. (`npm-publish.yml` uses OIDC Trusted Publishing — the correct posture; anywhere else is a leak vector.)
- U4 Greptile rule — flag any Go source that writes to `~/.claude/` or `~/.vscode/`, the TanStack persistence sinks.

**Current state audit:** the repo's existing `pull_request_target` workflow (`.github/workflows/greptile-policy-gate.yml`) is safe — it makes API calls only and does not check out PR head code. The generator repo's existing `pull_request_target` workflow (`cli-printing-press/.github/workflows/conversation-resolution-check.yml`) follows the same safe pattern. The new rules are forward-looking gates.

Primary sources:
- [Socket.dev — TanStack npm packages compromised: Mini Shai-Hulud supply chain attack](https://socket.dev/blog/tanstack-npm-packages-compromised-mini-shai-hulud-supply-chain-attack)
- [The Hacker News — TanStack supply chain attack hits two OpenAI employee devices](https://thehackernews.com/2026/05/tanstack-supply-chain-attack-hits-two.html)

### node-ipc (May 14, 2026)

An expired-domain account takeover gave attackers control of the `node-ipc` maintainer account. An obfuscated IIFE was appended after `module.exports`, harvesting 113+ categories of credential files (`~/.aws`, `~/.ssh`, `~/.kube`, browser session storage, env-var caches) and exfiltrating via DNS TXT records through the Session P2P network (`filev2.getsession.org`).

**Defense applied here:** U4 Greptile rule — flag any CLI's Go source that reads home-directory credential paths or sensitive env vars when the CLI's documented purpose doesn't plausibly explain it. Judgment-only, not mechanical; legitimate auth CLIs (AWS-shaped, GitHub-shaped) still pass.

Primary sources:
- [Socket.dev — TanStack mini-Shai-Hulud writeup (cross-incident references node-ipc shape)](https://socket.dev/blog/tanstack-npm-packages-compromised-mini-shai-hulud-supply-chain-attack)

### BufferZoneCorp (May 2026)

A "sleeper packages" campaign: initially clean Ruby gems and Go modules were published under the `BufferZoneCorp` GitHub account, masquerading as recognizable names (`activesupport-logger`, `devise-jwt`, `go-retryablehttp`, `grpc-client`, `config-loader`). After a dwell period of days to weeks, the maintainers pushed payload-bearing updates that:

- Used `replace` directives to redirect Go module resolution to attacker forks.
- Set `GOPROXY` / `GONOSUMCHECK` environment variables in CI to bypass checksum verification.
- Planted fake Go wrappers to intercept commands.
- Established SSH persistence by adding attacker keys to `authorized_keys`.

Packages were eventually yanked from RubyGems and blocked from the Go module proxy.

**Defenses applied here:**
- R3 — tier `replace` directives in `library/**/go.mod` by target shape. Remote targets (host segment with a dot, or any URL scheme) hard-fail; local paths (`./...`, `../...`) emit advisory notices but do not block (legitimate local vendoring exists in `library/food-and-dining/ordertogo/go.mod`).
- R4 — block `GOPROXY`, `GOFLAGS`, `GONOSUMCHECK`, `GOSUMDB`, or `GONOSUMDB` overrides in any `env:` block under `.github/workflows/**`.
- R6 — block `module` directive drift on an existing `library/**/go.mod` (must continue to start with `github.com/mvanhorn/printing-press-library/library/`). Renaming a published CLI is a generator-repo operation; manual go.mod edits to the `module` line are the BufferZoneCorp install-redirect attack shape applied to this catalog's structure.

Primary sources:
- [The Hacker News — Poisoned Ruby Gems and Go Modules Exploit CI Pipelines for Credential Theft](https://thehackernews.com/2026/05/poisoned-ruby-gems-and-go-modules.html)
- [Security Arsenal — BufferZoneCorp Supply Chain Attack: Poisoned Ruby & Go Modules Targeting CI Pipelines](https://securityarsenal.com/blog/bufferzonecorp-supply-chain-attack-poisoned-ruby-and-go-modules-targeting-ci-pipelines)

## Defense architecture

Two layers, applied identically across both repos.

**Greptile rules** (`greptile.json` in each repo) — judgment-heavy review. Greptile reads the CLI's purpose (SKILL.md, README.md, `.printing-press.json` display name) and decides whether a credential read is incongruous, whether a `replace` directive's stated purpose is plausible, whether a `pull_request_target` workflow combines with checkout-PR-head in a dangerous way. Findings post as P0 / P1 comments with rationale.

**`verify-supply-chain.yml`** — deterministic mechanical gate. Python scan at `.github/scripts/verify-supply-chain/scan.py` regex-matches each touched file from the PR diff against the signal catalog in `signals.py`. Block-severity findings exit non-zero and produce GitHub Actions error annotations. Advisory findings produce notices without affecting exit code. Run locally with `python3 .github/scripts/verify-supply-chain/scan.py --base-ref origin/main`.

Mechanical gate is authoritative for block decisions; Greptile is for rationale and judgment. False positives in Greptile do not block; false positives in the scan can be addressed by adjusting `signals.py` and adding a test case.

## Defense-to-incident mapping

| Defense | Rule ID | Layer | Primary attack |
|---|---|---|---|
| `pull_request_target` + PR-head checkout | R1 | Greptile + scan | TanStack |
| `id-token: write` outside `npm-publish.yml` | R2 | Greptile + scan | TanStack |
| `replace` directive → remote target | R3 (block) | Greptile + scan | BufferZoneCorp |
| `replace` directive → local path | R3 (advise) | Greptile + scan | BufferZoneCorp (precaution) |
| `GOPROXY` / `GOFLAGS` / `GONOSUMCHECK` env override | R4 | Greptile + scan | BufferZoneCorp |
| `postinstall` / `preinstall` / `prepare` in `npm/package.json` | R5 | Greptile + scan | Axios |
| `module` directive drift on existing CLI | R6 | Greptile + scan | BufferZoneCorp (catalog variant) |
| Go-source credential-path reads in incongruous CLI | (U4 rule) | Greptile only | node-ipc, TanStack |

## What is not covered

- Real-time runtime sandboxing of `go install`, `npx`, or any user-machine execution.
- Sigstore/Fulcio signing changes (the npm wrapper already uses OIDC Trusted Publishing).
- Mandatory 2FA enforcement.
- Slack/Discord alerting on scan findings.
- Real-time dependency-age checks against the npm registry API.
- Retroactive sweep of already-published CLIs for any pattern the new scan flags.
- SHA256 IOC matching against known malicious payload hashes.
- Base64-array obfuscator heuristics.
- Egress blocking of known C2 hosts at the org perimeter.

Several of these are documented as future considerations in the plan.

## Rollout posture

The `verify-supply-chain` workflow runs informationally for one week. After a green window with no false-positive miscalibration on real PRs, it is promoted to a required check via branch protection. The published-library PR lands first; the `cli-printing-press` mirror follows after the published-library deployment stabilizes — calibration on the larger repo (more PR throughput, more diverse touch surface) reduces the chance of duplicating any miscalibration into the upstream generator.
