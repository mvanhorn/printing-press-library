# Azure Cost Admin Live Validation Summary

Validation used a local Azure CLI session with read-only commands. The public proof intentionally omits subscription IDs, tenant IDs, account emails, resource IDs, resource group names, and exact spend amounts.

## Checks

- `doctor --json` passed Azure account detection, Azure Cost Management, Azure Resource Graph, and Azure Retail Prices reachability.
- `spend summary --timeframe MonthToDate --agent --select timeframe,currency` returned a timeframe and currency only.
- `spend by-service --timeframe MonthToDate --agent --select timeframe,currency` returned a timeframe and currency only.
- `tags untagged --tag owner --limit 1 --agent --select type` returned a resource type only.
- `price search --service "Virtual Machines" --region eastus --currency USD --limit 1 --agent --select serviceName,region,currencyCode` returned public retail price metadata.
- `go test ./...` passed.
- CLI dry-runs produced read-only Cost Management, Resource Graph, and Retail Prices request shapes.
- Secret and identifier scans found no Azure tenant IDs, subscription IDs, user emails, resource IDs, customer names, or exact spend outputs in the source tree.

## Notes

`price search` is treated as an estimate helper. Actual spend validation uses Azure Cost Management, not the public Retail Prices API.
