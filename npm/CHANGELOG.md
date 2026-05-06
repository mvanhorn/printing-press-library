# Changelog

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
