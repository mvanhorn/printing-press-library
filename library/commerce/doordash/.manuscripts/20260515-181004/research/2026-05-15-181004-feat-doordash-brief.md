# DoorDash research brief

This promoted print packages a curated DoorDash GraphQL surface discovered from browser-session traffic and normalized into safe CLI workflows.

Key publishable workflows:

- read-only store and menu discovery (`search`, `menu`, `item-options`, `convenience-search`)
- redacted account reads (`recent-orders`, `addresses`, `payment-methods`)
- checkout/fee preview that does not place orders
- explicitly gated cart/order mutation commands

Sensitive live session material is not included in the public artifact.
