# Facebook Marketplace CLI Brief

## Source Priority

Primary source: user-provided, auth-stripped HAR capture from Facebook Marketplace. Facebook has no public Marketplace OpenAPI surface for these workflows, so this print uses the captured browser GraphQL traffic as the authoritative spec source.

The first `browser-sniff --har` pass detected 1,544 entries, 180 GraphQL POSTs, GraphQL protocol confidence 0.92, and browser-required reachability. The stock spec emitter produced only one HTML endpoint, so this run preserves that raw output and uses an augmented internal spec extracted from the HAR's captured Relay operation names and document ids.

## Auth And Reachability

Authentication is browser-session based. The cookie/session is the secret, not an API key. The generated CLI must store session material in the generated browser-auth/config flow under the user's home directory or platform keychain-compatible local application paths, never in the Dropbox project workspace.

Reachability is high risk. The traffic analysis reported browser-required mode and challenge evidence. `fbm doctor` is mandatory and should detect session drift before writes. Write commands require an explicit `--write` flag and a recent passing `fbm doctor` result.

## Shipping Scope

Approved v1 scope:

- Buy-side search and listing detail reads.
- Local watcher workflows: `watch add`, `watch list`, `watch toggle`, `watch run`, and `matches`.
- AI listing drafter: `draft`.
- Local stale listing query: `stale`.
- Sell-side inbox reads and explicit write-gated replies.
- Sell-side listing create/update/delete only behind `--write` and doctor gating.

Rejected for v1:

- Cross-platform OfferUp, Craigslist, eBay, or marketplace federation.
- Multi-account orchestration.
- Buy-side auto-outreach. `match contact` must remain human-invoked.
- Always-on daemon behavior.

## HAR-Backed Operations

The augmented spec includes the v1-relevant GraphQL operations observed in the HAR:

- `CometMarketplaceSearchContentPaginationQuery`
- `MarketplacePDPContainerQuery`
- `MarketplacePDPC2CMediaViewerWithImagesQuery`
- `MarketplaceCometBrowseFeedLightContainerQuery`
- `CometMarketplaceInboxContentContainerQuery`
- `CometMarketplaceInboxSellerTabThreadViewContainerQuery`
- `CometMarketplaceInboxSellerTabThreadViewPaginationQuery`
- `CometMarketplaceMessageSellerMutation`
- `CometMarketplaceComposerRootComponentQuery`
- `useMarketplaceComposerPricePredictionQuery`
- `useMarketplaceComposerCalculatedShippingOptionsQuery`
- `useCometMarketplaceListingCreateMutation`
- `CometProductItemChangeAvailabilityMutation`
- `useCometMarketplaceForSaleItemDeleteMutation`
- `CometMarketplaceSetBuyLocationMutation`
- `CometMarketplaceSetBrowseRadiusMutation`

## Constraints To Carry Into Implementation

- Binary name: `facebook-market-pp-cli`; suggested shell alias: `fbm`.
- Buy-side watcher filtering is deterministic first: `must_have_keywords`, `reject_keywords`, price band, distance. LLM relevance scoring is not performed; matches remain deterministic.
- Eval suites start at 15 listing items and 25 `(watch, listing)` pairs, then grow from real failures.
- Idempotency keys must be created before every write network call. Write states are `queued`, `submitted`, `confirmed`, `unknown_outcome`, `failed`, and `halted_by_checkpoint`.
- Dropbox receipts redact message bodies after the first 80 chars plus classification, seller PII beyond public listing data, and photo URLs with FB CDN auth tokens.

## Open Decisions

- HAR recapture cadence: default to recapture when `fbm doctor` reports drift; otherwise about every three months.
- `--write` doctor requirement: proposed yes, at call time.
