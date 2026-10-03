# Current Normal Transport Contract

Source fingerprint: `0589d1376c7ee41ee961cda777327743e4aa8fbce0183f7ec92a8c20b52ed0bb`. CLI SHA256: `08573b3d0522f7f7143a534b3b1bdc7517f17bd1a574c5f2fbdf9291cb66d8e5`.

Actual ordinary CLI planner fixtures used a 100ms individual-request timeout against isolated local throttled and slow HTTP responses. throttle: exit5, 0.1115s, stdout0bytes; slow: exit5, 0.1114s, stdout0bytes. Both failed explicitly with Nap Camp and timeout/deadline context, empty stdout and no inferred success or availability evidence.

The timeout bounds individual HTTP requests. Normal retry/backoff can extend total process runtime; this is not a total-process deadline. The fixtures performed no provider transaction. Optional local learning was disabled only for these transport probes, not the full live gate.
