# Live Gumroad Dogfood

Date: 2026-05-11

Auth: A throwaway Gumroad OAuth application token was created through Gumroad settings for this dogfood pass. The token is not stored in the repository and command output payloads are intentionally omitted from this proof.

## Commands

- `gumroad-pp-cli doctor --agent` PASS
- `gumroad-pp-cli user --agent` PASS
- `gumroad-pp-cli products list --agent` PASS
- `gumroad-pp-cli sales list --agent` PASS
- `gumroad-pp-cli payouts list --agent` PASS
- `gumroad-pp-cli sync --resources products,sales,payouts --latest-only --json` PASS
- `gumroad-pp-cli search "printing-press-dogfood" --data-source local --json` PASS
- `gumroad-pp-cli analytics --type sales --group-by product_id --limit 10 --json` PASS

## Notes

Only read-oriented API calls were run. The sync, search, and analytics checks exercised the local SQLite workflow without mutating Gumroad account data.
