# Sunflower Ferry CLI agent guide

Read README.md and SKILL.md for the verified public Kansai–Kyushu planning scope. The typed commands and their MCP command mirrors are the supported planning interface. Local source is in internal/ferry and the owned CLI constructors. Generated framework/template issues belong in the Printing Press retro ledger.

## Runtime truth

```bash
sunflower-ferry-pp-cli doctor --json
sunflower-ferry-pp-cli which quote --json
sunflower-ferry-pp-cli quote --help
sunflower-ferry-pp-cli routes list --agent
```

Use future Japan service dates within the source booking window. Keep Osaka Terminal1 and Terminal2 distinct, preserve source school-stage/vehicle categories, explicit calendar coverage and E daytime cruises, and retain source observation timestamps. Quotes do not prove inventory, final payable price or eligibility.

## Read-only boundary

HTTP planning stops at the anonymous Reserve1020 fare table. Reserve1020/MoveNext, cabin selection, standby registration, personal data, accounts, holds, reservations, cancellation and payment paths must remain unreachable. Source cookies/anti-forgery fields stay only in ephemeral memory. Use exact first-party hosts and the existing allowlist, request/body/time caps and limiter.

Every owned command declares its local/live data-source annotation and enforces incompatible --data-source requests before IO. The hand-written ferry client must honor the command deadline, preserve typed rate-limit errors and never turn source errors into empty inventory.

## Verification

```bash
go test -count=1 ./internal/ferry ./internal/cli
go build -o sunflower-ferry-pp-cli ./cmd/sunflower-ferry-pp-cli
```

Parser fixtures prove deterministic domain logic; they never replace live provider checks. Test actual six-direction/date, car/child, bike and E daytime behavior read-only, then rebuild both CLI/MCP and the MCPB after source changes. Keep current live evidence and independent-review fixes in the run proofs. Do not alter the release ledger or publish without the user's publication instruction; personal GitHub writes must authenticate as zjsng.

Local learning/profile helpers store only local agent guidance. Dynamic prices/availability require live fetches. Preserve generated reserved packages; use hand-authored extension files and record template issues in the retro evidence.
