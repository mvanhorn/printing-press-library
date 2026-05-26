# Draft pick query family

The ESPN CLI does not surface draft order through any of its commands:

- `espn-pp-cli news <sport> <league> --agent` returns the live news feed but draft articles are intermittent. Even immediately after a lottery, the headline feed may not carry the result. The 2026-05-25 dogfood confirmed: post-lottery, the NBA news feed had zero draft headlines visible to the CLI surface.
- `espn-pp-cli search "draft lottery"` returns empty. The search index doesn't cover draft articles consistently.
- There is no `espn-pp-cli draft` command and no `--draft` flag on existing commands.

## The reliable path: WebFetch espn.com/$SPORT_PATH/draft/rounds

For each major US sport, the draft order lives at one of these URLs:
- NBA: `https://www.espn.com/nba/draft/rounds`
- NFL: `https://www.espn.com/nfl/draft/rounds`
- MLB: `https://www.espn.com/mlb/draft/rounds`
- NHL: `https://www.espn.com/nhl/draft/order`

Fetch the page, scan for the first round; pick 1 is the team you want. For lottery-driven leagues (NBA, NHL) the order also documents which lower-seeded team won the lottery to claim the #1 spot.

The `/draft/rounds` URL is the canonical path for NBA/NFL/MLB; NHL uses `/draft/order`. Try `/draft/rounds` first, fall back to `/draft/order` if 404.

## CRITICAL: Do NOT teach the year-specific answer

The specific answer (which team holds #1) decays with each draft cycle. The 2026-05-25 dogfood observed an agent fire `teach-playbook` with `notes "Wizards/2026 #1 pick"` despite this rule. That note will mislead the 2027 draft session because recall has no concept of answer-decay TTL.

Rule: amend ONLY for ROUTING improvements (e.g., "ESPN's draft URL changed from /draft/rounds to /draft/order in October", "the lottery results page now lives at /draft/lottery"). Use the user-facing answer to report the year-specific result.

If you want to encode the answer for retrieval, use auto-memory or a session-local note, not playbook notes. Playbooks are shared across years and sessions.

For seasonal accuracy: cross-check the published-at date on the espn.com page before quoting the order. Mock drafts and prediction articles can show "projected" orders that differ from official lottery results.

## Why no resource teach

The answer isn't an ESPN event/team/news ID. Teaching `--resource-type other --resource draft-2026` would create a synthetic id that always trips `resource_not_in_store` and provides no shortcut on retrieval. Notes-only playbook is the right shape here.

## Cross-sport applicability

Same routing logic applies to NFL Draft (April), MLB Draft (July), and NHL Draft (June-July). The $SPORT_PATH slot in the URL template resolves to `nba`/`nfl`/`mlb`/`nhl` based on which league the query references.
