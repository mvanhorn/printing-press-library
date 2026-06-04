# qbo CLI

Fixture-first, read-only QuickBooks Online CLI for agents.

## CLI commands

- `status` - Show fixture-only/read-only safety status.
- `auth status` - Show no-live-OAuth credential status.
- `company info` - Read QBO CompanyInfo from a fixture.
- `accounts list` - List chart-of-accounts records from a fixture.
- `customers list` - List QBO Customer records from a fixture.
- `vendors list` - List QBO Vendor records from a fixture.
- `items list` - List QBO Item records from a fixture.
- `invoices list` - List QBO Invoice records from a fixture.
- `bills list` - List QBO Bill records from a fixture.
- `payments list` - List QBO accounting Payment records from a fixture.
- `reports profit-and-loss` - Read QBO ProfitAndLoss report fixture.
- `reports balance-sheet` - Read QBO BalanceSheet report fixture.
- `reports trial-balance` - Read QBO TrialBalance report fixture.
- `query` - Echo/read a QBO query fixture; future live mode maps to QBO SQL-like query endpoint.
- `raw get` - Read a fixture as a raw QBO resource envelope.

## Safety requirements

- First Printing Press candidate is fixture-only and read-only.
- No live Intuit OAuth, no token storage, no .env, no live API calls.
- No write/mutation commands.
- Future live OAuth must use `com.intuit.quickbooks.accounting`; optional `openid profile email`; do not request `com.intuit.quickbooks.payment` unless Intuit Payments API features are added.
- Future auth must follow `docs/printing-press-submission-safety.md` from the prototype.
