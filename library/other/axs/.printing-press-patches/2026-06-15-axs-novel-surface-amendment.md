# AXS novel surface amendment

## Why

Direct quarqnet summary evidence showed the initial dogfood conclusion was too
narrow: synced `/componentsummaries/` records include battery status, firmware
version, and shift-count fields. The novel surface should keep the high-value
firmware, battery, shift, and usage views, with accurate local source
strategies.

## Amendment

- Keep the shipped novel surface to `firmware-check`, `battery`, `wear`,
  `shifts`, `garage`, and `since`.
- Implement `firmware-check`, `battery`, `wear`, and `shifts` as local
  generated-tree amendments over synced component summaries.
- Mark `firmware-check` with `// pp:data-source local`.
- Mark `battery` with `// pp:data-source local`.
- Mark `wear` with `// pp:data-source local`.
- Mark `shifts` with `// pp:data-source local`.
- Mark `garage` with `// pp:data-source live`.
- Mark `since` with `// pp:data-source local`.
- Keep README/SKILL language clear that the CLI reads web-app-visible status but
  does not flash firmware, pair components, or configure shifting.
- Under `PRINTING_PRESS_DOGFOOD=1`, let `bikes get` replace the static example
  UUID with a real ID derived in memory from `bikes list`; do not persist the ID
  or change normal user behavior.
- Validate `tail` resources before polling so invalid resource arguments fail
  fast instead of returning a warning with exit zero.
- In dogfood/agent contexts, compact `summaries components --json` to
  operational component fields so live proof capture does not include giant
  per-second arrays or account owner identifiers.

## Reprint guard

Future prints should preserve the AXS web-service/Bluetooth boundary:

- Cloud-supported: bikes, components, registrations, models, products,
  notifications, linked IDs, activities, summaries, component summaries, stats,
  sync/search.
- Novel supported: `firmware-check`, `battery`, `wear`, `shifts`, `garage`,
  `since`.
- Out of scope: BLE pairing, firmware flashing, and shift configuration.
