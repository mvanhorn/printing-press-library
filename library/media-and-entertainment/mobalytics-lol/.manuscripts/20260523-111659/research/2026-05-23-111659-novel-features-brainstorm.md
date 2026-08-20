# Novel Features Brainstorm — mobalytics-lol-pp-cli

Full audit trail of the Phase 1.5 Step 1.5c.5 subagent output.

## Customer model

**Persona 1: Daria "DariusDive" — Plat 2 top-laner, hard-stuck climbing to Diamond**

*Today.* Daria is 24, works a sales job in Hamburg, queues 3-5 ranked games each evening on a 90Hz monitor. She mains Darius and Aatrox top with a Garen pocket pick, sits at Platinum 2, and has been hovering there for two splits. She treats laning as a solved problem ("hit 6, dive, snowball") but loses 60% of her side-lane matchups vs grasp Sett and conqueror Fiora because she doesn't know which keystone they're running this patch.

*Weekly ritual.* On patch day she opens four tabs — Mobalytics for her champ's runes, op.gg for the tier list, lolalytics for the matchup table, u.gg for duo synergy with her jungler — and stitches the answer together in a Notion page she shares with her duo. Half the time the four sites disagree (sample sizes vary by rank/region) and she picks whichever is most flattering to Darius.

*Frustration.* "I want one place to ask 'this patch, Diamond+, EUW, Darius vs Fiora top — what runes do *they* run and what's the WR delta vs last patch?'" Every site shows aggregate WR but none of them show the rune-page Fiora mains are actually pressing in lane against her this week.

**Persona 2: Marco "ARAMonly" — Gold-equivalent ARAM-only player, queue-dodger of Singed bridges**

*Today.* Marco is a 37-year-old dad who plays ARAM during his kid's nap. He never plays Summoner's Rift and doesn't care about ranked. He rerolls aggressively and wants to know within 3 seconds of seeing his teammate's champion: "is this an early-spike comp or a late-game comp, and who should be frontline?"

*Weekly ritual.* In champ select he tabs to mobalytics.gg/aram/{champ}, copies the build into the in-game item-set, then alt-tabs back. He repeats this 5 times per game (once per teammate plus opponents). His ARAM build folder has 60 stale item-sets from past patches because Championify stopped updating in 2019 and nothing replaced it.

*Frustration.* "ARAM tier and build data is treated as a footnote on every site. I want to dump 10 champion ARAM builds into my client folder in one command before I start a queue session, and I want power-spike phase shown next to each pick so I know who frontlines."

**Persona 3: Sora "FlexCoach" — Part-time coach for a tier-3 amateur team, draft analyst**

*Today.* Sora is 28, coaches a 5-stack that scrims twice a week and plays a regional amateur league on weekends. She watches replays, builds draft cheat-sheets in Google Sheets, and her team pays her 200 EUR/month. Her players have a confirmed champion pool of 4-6 picks each; she needs to know — given those pools and the enemy team's confirmed pool — who is a flex pick, who counters whom, and what the bot-lane duo synergy looks like.

*Weekly ritual.* Pre-scrim she pulls 30 mobalytics pages by hand, copies counter and synergy tables into a sheet, and computes pool-vs-pool overlap manually. Patch day breaks her entire spreadsheet because tier/WR moved and she has no automated way to diff "what shifted since last week."

*Frustration.* "Counter-pool analysis is just SQL — champion X in our pool counters champions Y,Z in their pool, with sample > 1000. But Mobalytics shows me one champion at a time and I'm doing the join in my head."

**Persona 4: Theo "AIBuilder" — Software engineer building a Claude-powered League coaching agent**

*Today.* Theo is 31, builds LLM agents for a living, plays Emerald on a smurf, and is prototyping a Discord bot that answers "should I pick X into Y?" for his clan. He's tried wiring Claude to op.gg via headless browsers — too slow, too fragile — and to Riot's API directly, which gives him match data but no aggregator opinion.

*Weekly ritual.* Reads MCP server announcements, evaluates which APIs are agent-accessible. He's already integrated the Riot MCP servers, found they only expose match/profile data, and is stuck reimplementing the tier-list crawl himself.

*Frustration.* "There's no MCP for aggregator data. I want a CLI that exposes tier-list, build, counter, and meta-shift as agent-callable tools so my Discord bot can answer draft questions without me running a headless Chrome."

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source |
|---|------|---------|-------------|---------|--------|
| C1 | Counter-pool analysis | `counter-pool --our <c1,c2,c3> --their <c4,c5,c6>` | Given two champion pools, return the matrix of WR/sample for every our×their pair, ranked by lane-WR delta. | Sora | (a)+(c) |
| C2 | Patch meta-shift | `meta-shift --since-patch 14.10 --role top --rank diamond+` | Diff two `tier_snapshots` and list champions that gained/lost ≥1 tier or ≥2% WR, with sample-size guard. | Daria, Sora | (b)+(c) |
| C3 | Head-to-head champion compare | `compare <c1> <c2> [--role X]` | Side-by-side: tier, WR/PR/BR, power-spike phase, top-3 counters/synergies, build overlap (% items shared). | Daria, Sora | (b)+(c) |
| C4 | Counter-rune surfacing per matchup | `champion <c> matchup <opp> --runes` | When sample ≥ threshold, return the opponent's most-pressed rune page IN this specific matchup (not aggregate). Lolalytics-style, applied to Mobalytics's matchup table. | Daria | (b) |
| C5 | ARAM batch item-set export | `item-set --to client --aram <c1,c2,c3,...>` | Write LoL client item-set JSON for N champions in one call (ARAM mode), refreshing all stale files. | Marco | (a)+(b) |
| C6 | Power-spike-by-phase filter | `power-spike --phase early --role jungle --top 20` | List champions ranked by early/mid/late spike rating, role-filtered, with sample. | Marco, Sora | (b) |
| C7 | Duo-finder with pool awareness | `duo-finder --bot <c> --supports-from <c1,c2,c3>` | Best support pairings for a given ADC, restricted to a candidate-support pool with WR+sample. | Sora | (b) |
| C8 | One-trick build divergence | `one-trick <c> --diff` | Show how a one-trick's build differs from the aggregate Mobalytics build (LeagueOfGraphs signal, applied to delta). | Daria | (b) |
| C9 | MCP mode for agent draft tools | (binary served as MCP server: tier-list, build, counters, meta-shift) | Expose top commands as MCP tools so Claude/Discord bots can answer draft questions agentically. | Theo | (a)+(b) |
| C10 | Patch-stale check on local cache | `stale --tier-snapshots` | Report which `tier_snapshots` rows are older than the latest live patch and need `sync`. Local-data only. | Theo, Sora | (b) |
| C11 | Flex-pick detector | `flex --rank diamond+ --min-roles 2` | Champions with ≥ A-tier in 2+ roles this patch (joins tier_snapshots over role). | Sora | (c) |
| C12 | Pre-game team-comp lint | `team-lint --our <c1,c2,c3,c4,c5> --their <c1,c2,c3,c4,c5>` | Linter that runs counter-pool + duo-finder + spike-phase on a 5v5 lockin, flagging "no frontline," "no engage," "early-game team into late-game team." | Sora | (b)+(c) |
| C13 | Personal champion-pool tier digest | `pool-digest --pool <c1,...> --rank diamond+` | Single command: my pool, current tiers, WR delta since last patch, top counter for each, top synergy. Daily check-in. | Daria | (a)+(c) |
| C14 | Item-build path SQL | `item-path <item> [--builds-into]` | What core items build INTO Stridebreaker? What builds FROM Phage? Pure local SQLite over items.builds_from/into. | Sora, Theo | (c) |
| C15 | Champion-name fuzzy + alias search | `search "wukong"` / `search "the wandering caretaker"` | FTS over champion name + title + skin aliases. Helps agents resolve user typos. | Theo | (b) |
| C16 | Region-split tier delta | `tier-list --compare-regions kr,euw,na --role mid` | Same patch, same rank, three regions side-by-side. Shows pick-priority drift. | Sora | (b)+(c) |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Persona served |
|---|---------|---------|-------|--------------|--------------|----------------|
| 1 | Counter-pool analysis | `counter-pool --our <c1,c2,c3> --their <c4,c5,c6>` | 9/10 | hand-code | SQL join across `champion_matchups` for the cartesian product of two pools, ordered by WR delta with sample-size floor. No single Mobalytics page shows pool×pool — they show one champion at a time. | Sora |
| 2 | Patch meta-shift | `meta-shift --since-patch <N> [--role X] [--rank Y]` | 9/10 | hand-code | Diff two `tier_snapshots` rows (current vs --since-patch) on tier and WR, with sample-size floor and trend direction. Mobalytics shows current tier only, not deltas. | Daria, Sora |
| 3 | Head-to-head champion compare | `compare <c1> <c2> [--role X]` | 8/10 | hand-code | Side-by-side join across `tier_snapshots`, `champion_builds`, `champion_matchups` for two champion IDs; computes item-overlap %. No site renders two champions on one page. | Daria, Sora |
| 4 | ARAM batch item-set export | `item-set --aram --to client <c1,c2,c3,...>` | 8/10 | hand-code | Reads `champion_builds` rows where mode=aram for each champion, serializes to LoL client item-set JSON schema, writes to client config folder. Championify did this in 2019 and died; absorb #20 covers single-champ, this batches it. | Marco |
| 5 | Duo-finder with candidate-pool filter | `duo-finder --bot <c> --supports-from <c1,c2,c3>` | 7/10 | hand-code | SQL filter over synergy data: `WHERE champion=<bot> AND partner IN (<pool>) ORDER BY wr DESC`. u.gg has duo-finder but no pool restriction — Sora's pool is fixed by her players. | Sora |
| 6 | Personal pool tier digest | `pool-digest --pool <c1,...> [--rank X]` | 7/10 | hand-code | Single composite query: for each champ in pool, return current tier, WR delta since last patch, top-1 counter, top-1 synergy. Acts as Daria's morning ritual replacement for her 4-tab process. | Daria |
| 7 | Power-spike phase filter | `power-spike --phase <early\|mid\|late> [--role X] [--top N]` | 7/10 | hand-code | Filter `champion_builds.power_spikes` (Mobalytics's signature data) ranked by spike strength for the chosen phase. Absorb #15 returns spikes for one champion; this inverts to "give me everyone who spikes early." | Marco, Sora |
| 8 | Flex-pick detector | `flex --rank <X> --min-roles 2` | 6/10 | hand-code | `tier_snapshots` self-join: champions appearing as ≥A-tier in 2+ roles for the same patch/rank. Single SQL no site exposes — they index by role first. | Sora |
| 9 | Region-split tier compare | `tier-list --compare-regions kr,euw,na [--role X]` | 6/10 | hand-code | Multi-region `tier_snapshots` pivot: same patch, three regions side-by-side per champion. Mobalytics defaults to one region per page-load. | Sora |

### Killed candidates

| Feature | Kill reason | Closest-surviving sibling |
|---------|-------------|---------------------------|
| C4 Counter-rune per-matchup | Verifiability + buildability risk. Per-matchup rune WR may not exist on Mobalytics's matchup endpoint — only on lolalytics. Flagged in kill/keep pass. | C3 `compare` |
| C8 One-trick build divergence | Absorb #18 already ships `champion mained`; the `--diff` framing is a thin layer better as a future flag. | Absorb #18 `champion mained` |
| C10 `stale` patch check | Plumbing, table-stakes hygiene, not novel-per-CLI. | C2 `meta-shift` |
| C12 Team-comp lint | Scope creep / heuristic-flavored. Users can pipe primitives themselves. | C1 + C6 + C7 |
| C14 Item-path SQL | Convenience over absorbed `items` table; doesn't pass the "only we can do this" bar. | Absorb #3 `items list --select builds_from,builds_into` |
| C15 Fuzzy champion search | Argument parsing, not a feature. Belongs in the shared `<champ>` resolver. | (resolver behavior across champion commands) |
| C9 MCP mode | Table-stakes printing-press infrastructure, not novel-per-CLI. | (MCP layer wrapping all surviving commands) |
