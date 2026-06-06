# QuickBooks Online (QBO) CLI Brief

## API Identity
- **Domain**: Cloud accounting and small business financial management SaaS (Intuit).
- **Users**: Small business owners, accountants, bookkeepers, and integration agents.
- **Data profile**: Ledger-heavy (Accounts, JournalEntries, Invoices, Payments, Bills, Purchases, Customers, Vendors).

## Reachability Risk
- **Medium.** Requires OAuth 2.0 Authorization Code Flow. Access tokens expire in 1 hour; refresh tokens expire in 101 days with rotation. No simple API keys are supported. Environment can be Sandbox or Production.

## Top Workflows
1. **Financial Syncing**: Syncing ledger transactions locally into SQLite to bypass custom QBO SQL query limits (lack of Joins, Group By).
2. **Bank Reconciliation**: Matching bank transactions against open invoices/bills using fuzzy matching on amount/date.
3. **Double-Billing Audit**: Locating duplicate expenses or bills within a time window.
4. **Net Worth Calculation**: Grouping asset and liability account balances to calculate real-time net worth.
