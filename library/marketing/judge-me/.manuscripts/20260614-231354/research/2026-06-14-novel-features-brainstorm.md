# Judge.me novel feature brainstorm

## Customer model
- Ecommerce operator using Judge.me to manage product/social-proof quality.
- Support/reputation lead triaging bad reviews and unpublished/hidden review states.
- Growth/merchandising owner checking whether product pages have enough trust evidence before promotion.

## Candidates (pre-cut)
1. reputation summary — store-level trust dashboard from widget/shop/settings endpoints plus local review stats. Score 8/10.
2. reputation products — local SQLite product risk ranking by low-star rate and verified rate. Score 8/10.
3. reputation moderation-queue — review triage queue from synced review metadata. Score 7/10.
4. reputation settings-audit — trust-impact settings audit. Score 6/10.
5. reputation product — product-level widget/count evidence packet. Score 7/10.
6. auto-hide bad reviews — killed, mutates review curation and risks authenticity.
7. auto-reply generator — killed, customer-facing communication risk.
8. webhook installer — killed, external event-delivery mutation.

## Survivors and kills
### Survivors
- reputation summary: cross-endpoint + local mirror dashboard; weekly pre-campaign check; closest killed sibling was raw widget-count wrapper.
- reputation products: local SQLite aggregation; weekly product-quality review; closest killed sibling was endpoint-only reviews list.
- reputation moderation-queue: local metadata triage queue; daily/weekly support workflow; closest killed sibling was mutating review curation.
- reputation settings-audit: settings-to-trust findings; setup/theme QA workflow; closest killed sibling was raw settings dump.
- reputation product: combines count histogram and widget evidence; merchandising/ad QA workflow; closest killed sibling was raw preview-badge wrapper.

### Killed candidates
- auto-hide bad reviews: rejected because it mutates public review state.
- auto-reply generator: rejected because it can send customer-facing messages.
- webhook installer: rejected because it changes outbound event routing.
