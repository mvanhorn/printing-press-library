# Facebook Marketplace Absorb Manifest

## Sources

| Source | Role | Features Absorbed |
| --- | --- | --- |
| Prepared Facebook Marketplace HAR | Primary captured source | Search, listing detail, inbox, seller threads, message seller, listing composer, listing create, listing availability, listing delete, location, radius |
| Project use-case brief | Product scope | Buy-side watch, matches, draft, stale, sell-side replies, write gating |

## Captured Endpoint Surface

| Command Surface | Best Source | Status |
| --- | --- | --- |
| `marketplace browse-feed` | HAR `MarketplaceCometBrowseFeedLightContainerQuery` | generated |
| `marketplace set-buy-location` | HAR `CometMarketplaceSetBuyLocationMutation` | generated, write-gated in polish |
| `marketplace set-browse-radius` | HAR `CometMarketplaceSetBrowseRadiusMutation` | generated, write-gated in polish |
| `marketplace-search content` | HAR `CometMarketplaceSearchContentPaginationQuery` | generated |
| `listing get` | HAR `MarketplacePDPContainerQuery` | generated |
| `listing media` | HAR `MarketplacePDPC2CMediaViewerWithImagesQuery` | generated |
| `listing create` | HAR `useCometMarketplaceListingCreateMutation` | generated, write-gated in polish |
| `listing change-availability` | HAR `CometProductItemChangeAvailabilityMutation` | generated, write-gated in polish |
| `listing delete` | HAR `useCometMarketplaceForSaleItemDeleteMutation` | generated, write-gated in polish |
| `composer root` | HAR `CometMarketplaceComposerRootComponentQuery` | generated |
| `composer price-prediction` | HAR `useMarketplaceComposerPricePredictionQuery` | generated |
| `composer shipping-options` | HAR `useMarketplaceComposerCalculatedShippingOptionsQuery` | generated |
| `inbox list` | HAR `CometMarketplaceInboxContentContainerQuery` | generated |
| `inbox seller-threads` | HAR `CometMarketplaceInboxSellerTabThreadViewContainerQuery` | generated |
| `inbox seller-threads-page` | HAR `CometMarketplaceInboxSellerTabThreadViewPaginationQuery` | generated |
| `inbox message-seller` | HAR `CometMarketplaceMessageSellerMutation` | generated, write-gated in polish |

## Transcendence Features

| Feature | Command | Score | Buildability | Status |
| --- | --- | ---: | --- | --- |
| AI listing drafter | `draft` | 9 | hand-code | approved |
| Buy-side watcher | `watch add/list/toggle/run` | 9 | hand-code | approved |
| Watch matches | `matches` | 8 | hand-code | approved |
| Stale listing query | `stale` | 8 | hand-code | approved |
| Sell-side inbox shortcut | `inbox` | 7 | generated + polish | approved |
| Write-gated seller reply | `reply --write` | 8 | hand-code | approved |
| Cross-platform marketplace search | none | 3 | rejected | out of v1 scope |

## Phase Gate 1.5 Decision

Approved using the user's explicit pre-authorization:

- Build `draft`.
- Build `watch add`, `watch list`, `watch toggle`, `watch run`, and `matches`.
- Build `stale`.
- Keep `inbox` and build `reply --write`.
- Reject cross-platform OfferUp/Craigslist/eBay ideas for v1.

Features approved here are shipping scope and must not be silently downgraded to stubs. If a feature proves infeasible, return to this manifest with a revised scope.
