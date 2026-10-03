# Carstay CLI maintenance

Read README.md and SKILL.md for user workflows. This project is a read-only public station planner; external bookings, payments, account changes and publication require separate user instructions.

Preserve original Japanese names, stable IDs, canonical URLs, observation times and source language. Exclude activity-only records from overnight workflows. Date results are provider-filtered candidates, prices are starting references and vehicle acceptance is unknown. Keep missing booleans/dimensions/fees unknown and preserve facility notifications beside source flags. On-site and nearby facilities remain separate.

Hand-written logic is in internal/carstay; wiring is in internal/cli/carstay_commands.go and carstay_features.go. Use the public client with its AdaptiveLimiter, typed429 errors and command-bound context. Whitelist detail fields; orders, reviews, private notes and environment/session data are excluded. Bound returned rows, scan effort, detail IDs and source text separately. Keep JSON arrays non-null and agent envelopes flat.

For relevant changes, run `go test -count=1 ./internal/carstay ./internal/cli` and `go build ./...`; broaden tests after failures or broad changes. Verify changed source assumptions against the public provider contract. Keep template-owned shared packages unchanged and record systemic generator findings in run evidence. Preserve user-customized install targets and global configurations.

Press docsync can remove the inactive installer comment close and reinsert unsupported capability marketing. After its final run, restore the closing comment immediately after the canonical installer sentinel, assert balanced HTML comments and active planning sections, and restore the concrete evidence introduction. Rebuild both CLI/MCP companions and bundle after source work; benchmark the actual staged release binary. Keep phase5 markers binary-owned and current.
