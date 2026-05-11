# Phase 5 Acceptance Report: instagram

- Level: **Quick Check** (user-elected; full live test deferred to user's own schedule)
- Tests: **22/22 passed**
- Failures: none
- Fixes applied during Phase 5: none
- Printing Press issues for retro: none
- Gate: **PASS**

## Matrix coverage

**Help/discovery (14):** `version --json`, `--help`, `doctor --json` (parses), `agent-context --json`, `accounts --help`, `accounts get --help`, `get --help` shows `--slide-kind`, `user --help` shows `--rate`, `search --help` shows `--rerank`, `search --help` shows `--alt`, `watch` shows `add` subcommand, `highlights` shows `diff` subcommand, `auth login` shows `--sessionid`, `auth login` shows `--chrome`.

**Dry-run probes (5):** `get B0Lz_4QH --dry-run`, `user natgeo --dry-run --rate 30`, `search "test" --dry-run`, `search "test" --rerank` (correctly exits non-zero with `--rerank requires one of ANTHROPIC_API_KEY, OPENAI_API_KEY, or OLLAMA_HOST to be set`), `auth login --sessionid <value> --dry-run` (writes synthetic proof under `PRINTING_PRESS_VERIFY=1`).

**FTS smoke against empty DB (3):** `search "anything" --db <tmp>` initializes schema correctly, `whatsnew --db <tmp>` returns empty cleanly, `watch list --db <tmp>` returns empty cleanly.

## Why no live test

User declined full live IG test at the Phase 5 gate. Rationale (verbatim from the gate prose): the auth/header/parser/downloader code paths are static and verified in mock mode (shipcheck verify with `PRINTING_PRESS_VERIFY=1` writes a synthetic browser-session proof and exercises the cookie/proof read path); marginal value of one live HTTP call is low; Instagram's documented anti-scraping behaviour makes the downside (account checkpoint or ban on the user's main account) materially larger than the upside. User can run live verification independently after promote.

## Recommended follow-up

When ready to validate live:

```bash
instagram-pp-cli auth login --chrome
instagram-pp-cli doctor
instagram-pp-cli accounts get instagram --json
instagram-pp-cli user instagram --dry-run --rate 30 --json
```

If `doctor` reports session valid and `accounts get` returns a profile, the live path is healthy. Stop before running any non-dry-run `user <handle>` until you have confirmed those four commands pass against a known-public profile.
