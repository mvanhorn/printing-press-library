# CPSC Recalls CLI Research Brief

## API Identity
CPSC's Recall Retrieval Web Services publish decades of consumer-product recalls through unauthenticated JSON/XML REST queries at SaferProducts.gov. Records include products, hazards, incidents/injuries, remedies, manufacturers, retailers, countries, images, and recall dates.

## Users
- A parent/caregiver checking household or children's products for safety actions.
- A retailer or marketplace trust-and-safety analyst screening a catalog.
- A school, daycare, or property manager maintaining an equipment inventory.
- A product-safety journalist or researcher tracking brands, hazards, remedies, and incident trends.

## Top Workflows
1. **Inventory check:** match a CSV/list of owned or stocked products against recall records and show evidence for each candidate match.
2. **Brand/category watch:** report newly published or materially updated recalls for saved brands/products.
3. **Hazard pulse:** summarize recent recalls by hazard/remedy/category without treating recall count as incident rate.
4. **Recall packet:** produce an actionable record with affected product description, remedy, consumer contact, images, and source URL.

## Reachability Risk
The public API is unauthenticated but older documentation and flexible string filters create fuzzy-matching risk. Product names are unstandardized; inventory matches must be presented as candidates with matched fields, never definitive identity unless model/UPC identifiers agree.

## Table Stakes
Recall ID/date/title/product/manufacturer/retailer/hazard filters; detail; JSON/XML support; pagination or bounded queries; images/contact/remedy; local sync/search.

## Data Layer
SQLite stores recalls and nested products/hazards/remedies/incidents/images/manufacturers/retailers, normalized match tokens, observation timestamps, and user inventories. Raw source fields remain available for auditing.

## Codebase Intelligence
CPSC's website, email alerts, mobile app, and generic recall feeds are the main alternatives. The differentiator is evidence-preserving inventory matching and local change monitoring.

## User Vision
Build `inventory-check`, `watch`, `hazard-pulse`, and `packet` atop a complete CPSC recall query surface.

## Product Thesis
Turn recall data into an actionable household/catalog safety workflow without overstating fuzzy product matches.

## Build Priorities
1. Inventory matching with confidence evidence.
2. Saved watch and change detection.
3. Recall packet and hazard pulse.
4. Full query mirror.

## Sources
- https://www.cpsc.gov/Recalls/CPSC-Recalls-Application-Program-Interface-API-Information (raw capture HTTP 200)
- https://www.saferproducts.gov/RestWebServices/Recall

