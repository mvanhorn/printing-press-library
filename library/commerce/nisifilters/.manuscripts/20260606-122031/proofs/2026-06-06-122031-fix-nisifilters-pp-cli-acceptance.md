# nisifilters-pp-cli Acceptance Report (Phase 5)

Level: Full Dogfood (live, no auth — read-only public API)
Tests: 112/112 passed (0 failed)
Auth context: type=none

Gate: PASS

Fix applied during dogfood (fix-before-ship): removed the dead `product_attributes`
resource — NiSi's global /wc/store/v1/products/attributes returns [] (3 happy-path
"missing runnable example" failures). The `filters` command provides attribute
discovery from the products table instead. Spec edited + regenerated; re-dogfood clean.

Printing Press issues for retro: 1 — syncResourcePath base_url override gap (carried over
from francescogola; worked around via absolute wc/store URLs in spec).

PII: report contains no PII (read-only public catalog/content).
