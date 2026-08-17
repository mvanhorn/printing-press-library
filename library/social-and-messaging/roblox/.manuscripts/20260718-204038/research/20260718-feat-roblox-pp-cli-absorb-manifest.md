# Roblox Absorb Manifest

## Absorbed

| Feature | Our implementation | Added value |
|---|---|---|
| Typed public resource lookup | `(generated endpoint) users get` and the generated Roblox resource tree | JSON, dry-run, structured errors, retry and caching |
| Agent integration | `(behavior in roblox-pp-cli agent-context) Cobra-tree MCP mirroring` | Agent-discoverable commands and compact output |
| Local collection | `(behavior in roblox-pp-cli sync) SQLite-backed resource sync` | Offline retention and repeatable snapshots |
| Local discovery | `(behavior in roblox-pp-cli search) full-text local search` | Cross-resource lookup without repeated HTTP calls |
| Local aggregation | `(behavior in roblox-pp-cli analytics) group and count synced records` | Scriptable summaries over retained data |

## Transcendence

| # | Feature | Command | Buildability | Why only this CLI |
|---|---|---|---|---|
| 1 | User investigation bundle | `investigate user` | hand-code | Correlates public identity records across Roblox hosts |
| 2 | Group due diligence | `investigate group` | hand-code | Joins group and public owner context |
| 3 | Creator catalog footprint | `catalog creator-footprint` | hand-code | Correlates creator-attributed games and catalog records |
| 4 | Game ecosystem map | `ecosystem game` | hand-code | Connects universe, creator, badges, and media |
| 5 | Relationship overlap | `network overlap` | hand-code | Joins two users' locally retained relationships |
