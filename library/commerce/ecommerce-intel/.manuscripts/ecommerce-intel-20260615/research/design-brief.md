# Ecommerce Intel research brief

## Goal

Create a private Printing Press CLI that helps Cathryn run ecommerce marketing and Shopify page work from one local-first command surface.

## Source model

Ecommerce Intel is a composite CLI. It does not need to own raw API integrations in v0.1; it records source plans and normalizes local/fixture/imported data while delegating live collection to focused child CLIs:

- Shopify: products, orders, inventory, collections, abandoned checkouts
- Klaviyo: campaign/flow revenue, decay, attribution, list quality
- GA4: landing page/product/category sessions, ecommerce conversion behavior
- Google Search Console: query/page clicks, impressions, CTR, average position
- Ahrefs: top pages, keywords, referring domains, authority signals

## Product direction

The CLI should answer: "What should Cathryn work on next?" not merely print dashboards. The core ranked queues are:

- revenue and conversion opportunities
- money products/pages/categories
- inventory risk
- Klaviyo/email actions
- product and collection page actions
- GEO / answer-engine readiness for ChatGPT, Perplexity, Google AI Overviews, Google AI Mode, and Claude

## Privacy

This module is private-only. Its manifest sets `visibility: private`; registry rendering supports private entries without public release links.
