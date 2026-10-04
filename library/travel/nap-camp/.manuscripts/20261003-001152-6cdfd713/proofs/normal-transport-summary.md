# Current Normal Transport Contract

Source fingerprint: `5bd92fbf1f697323eef7d7bdae621976581062abb2acaacbce5bb74da7c1e57e`. CLI SHA256: `15a4c64fb6ea0b89bbbf7d30deb9fdafab1962d8ce94320971daf7a33ac5cddc`.

Actual ordinary CLI planner fixtures used a 100ms individual-request timeout against isolated local throttled and slow HTTP responses. throttle: exit5, 0.1116s, stdout0bytes; slow: exit5, 0.1178s, stdout0bytes. Both failed explicitly with Nap Camp and timeout/deadline context, empty stdout and no inferred success or availability evidence.

The timeout bounds individual HTTP requests. Normal retry/backoff can extend total process runtime; this is not a total-process deadline. The fixtures performed no provider transaction. Optional local learning was disabled only for these transport probes, not the full live gate.
