# Changelog

## 0.1.3

- Drop the `auth env vars: …` line from `install` output. The data was a bare list of env var names without the surrounding context (where to get the token, how to set it, what command verifies it) — that context lives in each CLI's `--help`, `doctor` command, and authenticated-error messages, which is the natural moment to discover auth requirements. JSON output no longer carries `authEnvVars` either; consumers that genuinely need a structured env-var list can read `mcp.env_vars` directly from `registry.json`.

## 0.1.2

- CI fix: pin the npm version used for Trusted Publishing to `npm@11.5.1`. The previous `npm install -g npm@latest` step is flaky on Actions runners — npm overwrites itself mid-install and the global install ends up with a missing `promise-retry` module. v0.1.1 was tagged but never reached npmjs.com because of this; this is the first published release on the OIDC pipeline.

## 0.1.1

- Rename binary from `pp` to `printing-press`. The previous two-letter name overlapped with our `pp-*` skill namespace, our `*-pp-cli` binary convention, and Perl's `pp` (PAR::Packer).
- Add bundles: `printing-press install starter-pack` installs `espn`, `flight-goat`, `movie-goat`, and `recipe-goat` together.
- Multi-name install: pass several names in one command, e.g. `printing-press install espn linear dub`. Bundle names and CLI names can mix freely.
- Add `--cli-only` and `--skill-only` flags so you can install just the Go binary (e.g. on a CI machine with no agent) or just the focused skill (relying on lazy binary install via the skill's prose). Mutually exclusive; both work with bundles.
- Switch the publish workflow to npm Trusted Publishing (OIDC). No long-lived `NPM_TOKEN` in repo secrets; releases mint short-lived tokens per workflow run and emit verifiable provenance attestations.
- Declare MIT license, repository, homepage, bugs URL, author/contributors, keywords, and `publishConfig` for npm discoverability.

## 0.1.0

- Initial scaffold for `@mvanhorn/printing-press`.
- Add `pp install`, `pp update`, `pp list`, `pp search`, and `pp uninstall`.
- Install per-CLI skills from `cli-skills/pp-<name>` via `skills@latest`.
