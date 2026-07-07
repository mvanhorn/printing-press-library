# Running Race Results Printing Press Port Brief

This package ports the existing `github.com/jiahongc/running-race-results` Go CLI at commit `96d242428e70076fed078f078f20399fbff2acb9` into a Printing Press publishable layout.

The CLI resolves race names through a bundled catalog, dispatches to provider adapters for NYRR, Mika Timing, Athlinks, and RaceResult, and renders unified results as tables or JSON. The port keeps the source behavior intact while renaming the binary to `running-race-results-pp-cli` for public-library compatibility.

Auth is optional. Athlinks anonymous lookup and athlete-history paths work without a token; `ATHLINKS_TOKEN` remains available for `athlete --me` and as a fallback if an endpoint returns 401/403.
