# xero CLI

Fixture-first, read-only Xero CLI for agents.

## CLI commands

- `status` - Show fixture-only/read-only safety status.
- `auth status` - Show no-live-OAuth credential status.
- `org info` - Read Xero Organisation fixture.
- `accounts list` - List Xero Accounts from a fixture.
- `contacts list` - List Xero Contacts from a fixture.
- `suppliers list` - List supplier contacts from a fixture.
- `vendors list` - Alias/convenience view for supplier contacts.
- `items list` - List Xero Items from a fixture.
- `invoices list` - List Xero sales invoices from a fixture.
- `bills list` - List Xero purchase invoices / bills from a fixture.
- `payments list` - List Xero Payments from a fixture.
- `reports profit-and-loss` - Read Xero ProfitAndLoss report fixture.
- `reports balance-sheet` - Read Xero BalanceSheet report fixture.
- `reports trial-balance` - Read Xero TrialBalance report fixture.
- `journals list` - List Xero Journals from a fixture.
- `raw get` - Read a fixture as a raw Xero endpoint envelope.

## Safety requirements

- First Printing Press candidate is fixture-only and read-only.
- No live Xero OAuth, no token storage, no .env, no live API calls.
- No write/mutation commands.
- Future live OAuth should use `offline_access accounting.settings.read accounting.contacts.read accounting.transactions.read accounting.reports.read accounting.journals.read`; optional `accounting.attachments.read`, `files.read`, `openid profile email`.
- Future auth must follow `docs/printing-press-submission-safety.md` from the prototype.
