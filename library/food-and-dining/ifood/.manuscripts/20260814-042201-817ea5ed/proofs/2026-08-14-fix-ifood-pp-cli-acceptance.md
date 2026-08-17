# iFood CLI full-dogfood acceptance

- Level: full
- Initial result: 68/86 evaluated checks passed; 18 failed; gate failed
- Root causes: 16 missing `Examples` sections and two `bm` invocations missing mandatory request-body flags
- Intermediate result: 107/112 passed; the remaining five checks exposed local feedback and credential-file fixture behavior
- Final result: 113/113 evaluated checks passed; 0 failed; 100% pass rate; no hollow feature coverage
- Fixes applied: all defects were corrected and validated without remote writes; flagship workflows now have embedded credential-free examples
- Browser proof: the existing signed-in iFood session displayed an authenticated profile, a selected delivery address, a real market rated 4.9, and live product-search results; no cart action was taken and no credential material was copied
- Current marker: `status=pass`, `level=full`, source fingerprint `c0b39c98522bab049103543a878793d6274a7e9ebeed3b6c84e943b66c29f4e5`

## Gate

`PASS`. The binary-owned full matrix produced a fresh `phase5-acceptance.json` matching the accepted source tree.

`cart build` is strictly read-only; `cart add --execute --yes` owns the confirmation-gated write. No checkout, payment, order submission, account change, address change, or cart mutation occurred during acceptance.
