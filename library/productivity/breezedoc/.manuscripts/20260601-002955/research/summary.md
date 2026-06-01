# BreezeDoc Research Summary

Run ID: 20260601-002955

Source:
- Official developer documentation: https://breezedoc.com/developer/docs/
- Credential setup page: https://breezedoc.com/integrations/api

API shape:
- REST API rooted at https://breezedoc.com/api
- Bearer-token authentication
- Core resources: documents, document recipients, templates, teams, invoices, current user

Published CLI additions:
- `workflow invoice-lifecycle`
- `workflow client-workspace-snapshot`
- `workflow document-follow-up`
- `workflow signature-packet-prep`

Validation notes:
- Structural publish validation passed after recording novel workflow features.
- Full live dogfood passed 86 of 86 counted tests.
- Mutating workflow examples use `--dry-run` unless an operator explicitly passes `--send`.
