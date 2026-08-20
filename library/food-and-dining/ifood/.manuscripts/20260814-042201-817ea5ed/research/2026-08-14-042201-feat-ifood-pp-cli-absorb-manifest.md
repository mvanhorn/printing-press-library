# iFood Absorb Manifest

## Absorbed

| Capability | Source | Required CLI behavior |
|---|---|---|
| Grocery market discovery | iFood web grocery home feed | Discover markets and enforce a minimum visible rating. |
| Merchant product search | iFood merchant catalog search | Resolve each requested term to an available, named, positively priced product. |
| Complete quotation | Existing iFood CLI workflow | Compare the same requested list across at least three complete markets. |
| Browser-owned authentication | Validated live Browser workflow | Never export cookies, authorization headers, anti-fraud values, or CAPTCHA state. |
| Preview-first cart composition | Existing iFood CLI workflow | Emit exact request details with `executed=false` unless explicitly authorized. |

## Transcendence

- Deterministic validation of Browser observations without browser credential export.
- Lowest-complete-total selection across rating-qualified markets.
- An explicit confirmation boundary before the first cart item is added.
- A cart plan that remains local, auditable, and unable to proceed to checkout or payment.

## Reprint Watch-list

The prior CLI's Browser commands and associated tests are an API-specific customization. Regeneration must preserve their local-only data-source annotation, credential-field rejection, three-market completeness rules, and `remote_write_performed=false` contract.
