# JR East shipcheck

Verdict: ship, pending independent review and final full live matrix.

All canonical shipcheck legs PASS: verify, narrative full-example validation, dogfood, live workflow verification, apify audit, skill verification and scorecard. Verify: 100% (28/28), zero critical failures. Steinberger score: 80/A. All five approved novel command paths are present and work in real-source acceptance. Separate behavioral live acceptance: 18 cases/33 requests, five regions; max latency 1709 ms and max discovery output 39806 bytes. Fixtures and Go vet pass.

Iteration fixes: novel constructors carry explicit source strategy in named files; workflow manifest corrected to the verifier's top-level expect_fields and indexed extract syntax. Earlier workflow failures were manifest-contract mistakes, not suppressed runtime defects. The final six-step live workflow passes, including extraction of a real native line ID into later status/itinerary commands.

Non-blocking structural warnings: the static novel depth scan confuses root status with framework workflow status; exact root Usage and real status output prove correct runtime routing. Dormant generated maxAge/helper warnings and generic guide-only sync do not describe operational features; domain source age is explicit and operational reads never serve cached status. These are retained as generator/shape notes, not fake fixes.

Scope limits: current published certificate links only; complex planned schedules and historical certificates use official handoffs. Unknown year, source timestamps, source reporting limits, AI translation and individual delay remain explicit. No browser runtime, external writes, accounts or paid service.
