# Polish (lean pass, performed inline)
Before/after: shipcheck 7/7 both; scorecard 78 -> 77 (B); dogfood live 131/131.
Fixes: G104 unhandled tabwriter flush errors, G304 nosec with reason, mcp:read-only annotations on read commands (tools-audit now clean), gofmt on which.go.
Unresolved gosec findings are in generator-owned files (retro candidates).
Live matrix: exercised (cookie session via FOODA_CONFIG).
ship_recommendation: ship
