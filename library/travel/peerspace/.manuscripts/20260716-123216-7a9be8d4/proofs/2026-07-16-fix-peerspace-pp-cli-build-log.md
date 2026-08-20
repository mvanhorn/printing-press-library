Manifest transcendence rows: 15 planned, 15 built. Phase 3 will not pass until all 15 ship.

Status: all novel TODOs removed; venuex + 15 commands + venues get shipped.

## Implementation summary

All 15 novel transcendence commands + `venues get` implemented against local SQLite (`resources`) via `internal/venuex` pure helpers.

### Novel commands (TODOs replaced)
1. scout budget
2. scout capacity
3. scout gaps
4. scout multi-city
5. shortlist compare
6. shortlist delta
7. shortlist drift
8. shortlist export
9. shortlist similar
10. markets pulse
11. markets neighborhoods
12. projects scout
13. sync coverage
14. pulse
15. venues recommend

### Also
- venues get (new file)
- venues parent unhidden + get wired
- sync coverage subcommand wired on sync parent

### Packages
- `internal/venuex/` — ParseListing, ExpandResourceData, BandPrices, BandCapacity, ScoreTechFit, GapChecklist, Median, PulseByCity, Neighborhoods, Similar, MultiCityTop, snapshots, favorites
- `internal/cli/venuex_store.go` — shared RO/RW open, loadListings, loadFavoriteIDs, loadCoverage, findListingByID

### Verify commands (run after this write)
```bash
cd /Users/nico/printing-press/.runstate/cli-printingpress-371d8ee3/runs/20260716-123216-7a9be8d4/working/peerspace-pp-cli
GOTOOLCHAIN=auto go test ./internal/venuex/ ./internal/cli/ -count=1
GOTOOLCHAIN=auto go build -o peerspace-pp-cli ./cmd/peerspace-pp-cli
./peerspace-pp-cli scout budget --city Paris --activity meetup --dry-run
./peerspace-pp-cli shortlist compare --dry-run
./peerspace-pp-cli venues recommend --guests 40 --budget-max 180 --dry-run
./peerspace-pp-cli venues get --dry-run
rg 'TODO: implement novel' internal/cli/
```
