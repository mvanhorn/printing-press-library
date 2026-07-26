# subject-pp-cli — adversarial review round

All six claims from the shipcheck writeup held under independent verification, including the one that mattered most.

## Claims verified
- **All default mirrors carry planet data.** The reviewer probed all three live with California, Zurich, and Sydney bounding boxes; every one returned JSON. The earlier blank results were transient HTML overload, not regional coverage. No second regional extract hiding in the list.
- `nwr` + `out center` with `center` parsing — live LA results are majority `way/`.
- HTTP 200 + HTML detected as overload; an empty body also fails closed.
- Per-mirror timeout is genuinely per-request.
- `route` is honest that its corridor is a bounding box, not a road buffer.
- An unknown `--type` fails loudly with the full list.

**No Overpass QL injection exists.** Tags come only from the hardcoded catalogue, `--type` must survive `Lookup`, and place names go to Nominatim with only `float64` values reaching QL via `%.6f`.

## Bugs fixed
1. **`CorridorBBox` ignored the antimeridian.** Two Fijian islands 31 km apart yielded `West:-180 East:180` — a 360°-wide band around the planet — and `route` then reported lighthouses in Brazil as stops on that drive. Anchorage→Tokyo spanned 290°. A crossing is now split into two boxes and searched as a union; `Area` takes `BBoxes` and `BuildQuery` unions them.
2. **The one test aimed at this certified the bug as correct.** It produced a whole-planet box and passed, because it only asserted the box stayed inside world bounds. Replaced with a test that pins the *span*: a 31 km crossing must not exceed 5° of longitude across all boxes, and each endpoint must fall inside one.
3. **`ParseDistance` treated unknown unit suffixes as kilometres.** `25miles` became 25 km — 62% of the area asked for, presented as complete. `inf` produced a literal `nwr(around:+Inf,...)`. Unrecognised units and non-finite values now error.
4. **A `remark` arriving with partial results was discarded.** Overpass truncates server-side under load and says so; the CLI rendered a clean table the user would read as complete. The remark now prints on both stdout and stderr, marked INCOMPLETE.
5. **A `0,0` sentinel stood in for "no coordinates given".** `near --longitude -118.24` with no `--latitude` searched the South Pacific and reported "0 found". Presence is now tested with `Flag.Changed`, the two flags must be given together, and both are range-checked.
6. **The mirror test asserted a two-entry denylist**, not the planet-coverage invariant its comment claimed — a fourth regional mirror would have passed. It now asserts an explicit vetted allowlist, so adding a mirror is a deliberate act that requires verifying coverage on two continents first.
7. The overall failover budget is capped at 150 s so a flag documented as a request timeout cannot become a three-minute wall-clock wait.

## Confirmed not a bug
The `lat==0 && lon==0` element filter does not drop legitimate equator or prime-meridian results — only exact Null Island.

## After
7/7 shipcheck legs · 12 Go test packages green · live `near` returns 25 water towers near Los Angeles with the correct way/node mix.
