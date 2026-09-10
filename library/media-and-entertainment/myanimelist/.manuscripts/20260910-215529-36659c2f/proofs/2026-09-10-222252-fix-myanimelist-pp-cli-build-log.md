Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# MyAnimeList CLI — Phase 3 build log

Run: `20260910-215529-36659c2f` · module `github.com/mvanhorn/myanimelist-pp-cli`
(The count above is the live value; it was updated as each row landed. All seven transcendence rows and every hand-built absorbed row are implemented — nothing was deferred and there are no stubs.)

## Priority 0 — foundation

- The generator emitted the data layer, sync/search/SQL path, config, client, cache freshness, MCP server, learn loop, and all 48 typed endpoint commands from `research/myanimelist-spec.yaml` (15 resources).
- `myanimelist-pp-cli doctor`, `sync`, `search`, `analytics`, `tail`, and the MCP search+execute pair all pass generation-time gates (`go test`, `go vet`, `govulncheck`, `go build`, runnable-binary probe).

## Priority 1 — absorbed features

Every absorbed row from the absorb manifest is covered. 38 endpoint commands ship from the spec; the rows that needed typed or stateful behavior were built by hand:

| Feature | Implementation |
|---|---|
| Typed title detail | `internal/malhtml.ParseDetail`, reached through `anime get` / `manga get` |
| Typed statistics | `internal/malhtml.ParseStats` behind `anime stats` / `manga stats` |
| Episode table | `internal/malhtml.ParseEpisodes` behind `anime episodes` |
| Cast + staff | `internal/malhtml.ParseCharacters` behind `anime characters` |
| Seasonal chart | `internal/malhtml.ParseSeason` behind `season list` / `season current` / `season later` / `season schedule` |
| Rankings | `internal/malhtml.ParseRanking` behind `ranking anime` / `ranking manga` |
| Person filmography | `internal/malhtml.ParsePerson` behind `person get` |
| Airing schedule | `airing --days N` (reads the site's own timezone-aware schedule page) |
| Franchise ordering | `watch-order <id> --depth N` |
| Local list management | `track add\|progress\|rate\|note\|list\|drop\|remove` (SQLite `mal_library`) |
| Continue watching | `next` |
| Suggestions | `suggest --min-score N --limit N` |
| MAL import bridge | `export --format mal-xml\|json\|csv` |

## Priority 2 — transcendence (all hand-code)

| # | Command | Status |
|---|---|---|
| 1 | `anime divisive <id>` | built — parses the 1–10 vote bars; conflict-mass index, not raw enthusiasm |
| 2 | `drift [<id>] --since 30d --record` | built — `mal_snapshots` table; leaderboard mode when no id |
| 3 | `week` | built — local library × JST broadcast slots, converted to local time, collision flags |
| 4 | `anime drop-risk <id>` | built — status distribution + episode count → risk band |
| 5 | `anime consistency <id>` | built — episode poll averages, first-3 vs last-3 slope, worst/best episode |
| 6 | `adaptation <anime-id>` | built — anime ↔ related manga join, reported as a band |
| 7 | `franchise gap` | built — relation edges for every locally tracked franchise, diffed against the library |

## Deliberately deferred

- **Playback** (`curd`, `anipy-cli`, `ani-cli` territory) and **multi-site aggregation** (AniList/Kitsu/Shikimori) are recorded as explicit non-goals in the manifest, per the Phase 1.5 approval.
- **Authenticated MyAnimeList surfaces** — the user chose anonymous-only at the auth gate, so there is no server-side list write path. `export --format mal-xml` is the deliberate bridge.

## Generator limitations found

1. **`profile` is a reserved resource name.** The spec's `profile` resource was rejected at parse time ("would overwrite internal/cli/profile.go"); it was renamed to `member`.
2. **A resource named `search` collides with the framework `search` command.** The instant-search resource was renamed to `instant` (command: `instant search`).
3. **No `response_format: xml`.** RSS and sitemap endpoints had to use `binary`, so those commands emit raw XML rather than a parsed shape.
4. **`http_transport: standard` is required for this shape.** The generated client rejects HTML responses unless the caller passes `X-Printing-Press-HTML-Response: true`; the hand-written fetch helper sets it, but the default path for a `response_format: html` endpoint is otherwise a hard error ("expected JSON, API returned HTML instead of JSON").
5. **Cross-spec `--force` regen dropped framework files.** Regenerating after the spec changed removed `internal/cli/export.go` and left `root.go` referencing `newExportCmd`; the command was re-implemented by hand against the local library.
6. **The generator emits a novel scaffold per research.json feature but cannot resolve commands that collide with a generated resource parent** (warning: `novel feature command "anime" maps to generated command path; skipping novel stub`). Harmless here because the leaf scaffolds still attach to the generated parent.

## Parser risk and mitigation

MyAnimeList answers an unknown sub-route with the base page and HTTP 200, so a wrong URL is indistinguishable from a right one by status code. Every parser therefore asserts on observed structure and returns a typed error naming the route form to check. `internal/malhtml` carries table-driven tests over trimmed real markup, and `TestParseStatsRejectsBasePage` pins the failure mode explicitly.

## Verification performed during the build

- `go build ./...`, `go vet ./...`, `go test ./...` all green; `internal/malhtml` has 8 table-driven tests.
- Live smoke tests against the real site: `anime divisive 5114` (broadly loved, 7.9), `anime drop-risk 21` (elevated, 8.1% dropped), `anime consistency 52991` (slope +0.4, improves), `adaptation 52991`, `watch-order 5114`, `airing`, `franchise gap` (found Frieren S2 as an unstarted Sequel), `track add/list`, `next`, `week` (IST conversion of a JST slot verified), `drift --record`, `suggest`, `export --format mal-xml`.
