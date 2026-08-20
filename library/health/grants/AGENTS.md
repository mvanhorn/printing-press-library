# Research Grants Finder Printed CLI Agent Guide

This directory ships `grants-pp-cli`, a keyless, stdlib-only Go CLI over three
public research-funding APIs: Grants.gov Search2 (open federal opportunities),
NIH RePORTER (awarded NIH projects), and NSF Awards (awarded NSF grants). It
follows the printed-CLI conventions of
[CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).

## Local Operating Contract

Start by checking upstream availability, then discover commands from the CLI
itself:

```bash
grants-pp-cli doctor
grants-pp-cli help
```

Commands:

```bash
grants-pp-cli search <keyword>   # open Grants.gov opportunities
grants-pp-cli nih <keyword>      # awarded NIH RePORTER projects
grants-pp-cli nsf <keyword>      # awarded NSF grants
grants-pp-cli doctor             # live check of all three sources
```

All commands accept `--json` for machine-readable output. No API keys, no
config files, no local state — every invocation is a direct HTTPS call.

Behavioral notes agents should know:

- `search --agency` sends the Grants.gov `agencies` field as a **plain string
  code** (e.g. `HHS-NIH11`). The array form silently matches nothing.
- `search --closing-before` filters client-side within the fetched page and
  warns on stderr when the page was full (`--rows` reached), because matching
  deadlines beyond the page are not fetched.
- `search --min-award` filters on `awardCeiling` with an `estimatedFunding`
  fallback, since Grants.gov often reports a zero ceiling.
- NIH/NSF return **awarded** grants (funding benchmarks); open calls come from
  the `search` command. NSF keyword matching is loose full-text OR upstream.
- Flags may appear anywhere on the command line (the dispatcher re-parses
  around positional arguments).

## Local Customizations

If you modify this code, record each change under `.printing-press-patches/`
(one JSON file per patch, parallel to `.printing-press.json`) so future
regeneration work preserves the intent. See the repo root `AGENTS.md` for the
patch entry shape.

For install, usage recipes, and product guidance, read `README.md` and
`SKILL.md`.
