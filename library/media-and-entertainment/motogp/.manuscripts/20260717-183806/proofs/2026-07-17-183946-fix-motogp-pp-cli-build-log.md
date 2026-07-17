# MotoGP CLI Build Log

## Generated (spec-emitted, 19 absorbed commands)
seasons, events, categories, sessions (+get), classification, grid, entry,
standings (list/files/bmwaward), livetiming, broadcast-categories,
broadcast-events (+get), riders (list/get/stats/statistics), teams,
plus framework: sync, search, sql/analytics, doctor, export, agent-context.

## Hand-built (7 transcendence commands + shared resolver)
- internal/cli/motogp_resolve.go — live UUID resolver (year→season, class→category,
  name→event, token→session, name→rider) + shared response models. Handles the two
  rider response shapes (name+surname vs full_name).
- results — race classification from human names
- title-race — round-by-round championship points progression (verified vs 2024 season)
- h2h — two-rider career stat comparison (ambiguity detection for same-surname riders)
- circuit-history — winners at a circuit across seasons (verified Mugello 2022-2026)
- career — rider profile + career stats
- since — finished rounds catch-up (+ optional --winners)
- calendar — season calendar view + --ics export (filters to kind==GP)

## Notes / deferred
- Name resolution covers current-season riders only (documented). Retired riders
  need the raw `riders get <uuid>` / `riders stats <legacy-id>` endpoints.
- livetiming returns empty between sessions (feed only live during a session) — documented.
- Expensive multi-round commands (title-race, circuit-history) curtail to 2 rounds
  under the dogfood matrix via cliutil.IsDogfoodEnv; calendar --ics gates on IsVerifyEnv.
- No stubs shipped.
