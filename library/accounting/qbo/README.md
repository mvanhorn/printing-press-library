# qbo-pp-cli

Fixture-first, read-only QuickBooks Online (QBO) CLI generated with CLI Printing Press plan mode.

## Status

This candidate is prepared for review as a **fixture/read-only scaffold**, not as a live OAuth integration.

It intentionally has:

- no live OAuth;
- no token storage;
- no `.env` usage;
- no live API calls;
- no mutation/write commands.

## Examples

```bash
qbo-pp-cli status
qbo-pp-cli accounts list --fixture testdata/fixtures/qbo/accounts.json
qbo-pp-cli reports trial-balance --fixture testdata/fixtures/qbo/trial_balance.json
```

## Printing Press review safety

The Granola PR review lessons are treated as hard gates before any live OAuth work:

- no silent credential fallback refresh;
- refresh requests must follow provider OAuth specs exactly;
- diagnostic errors must name the failing source;
- helper/keychain timeout behavior must not discard valid output;
- mutation commands are not included in the first PR.
