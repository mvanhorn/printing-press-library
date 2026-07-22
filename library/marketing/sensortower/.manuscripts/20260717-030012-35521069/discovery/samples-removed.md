# Raw response samples removed

The raw endpoint response samples (`samples/*.json`) captured during discovery were
removed before publishing. They contained public app-store listing metadata (e.g. a
publisher support email and corporate mailing address) that the publish PII gate flags
regardless of whether it is customer data. The endpoint shapes, field lists, and the
429/auth findings they informed are all documented in `browser-sniff-report.md`.
