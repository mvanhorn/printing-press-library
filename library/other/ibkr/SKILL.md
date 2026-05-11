---
name: pp-ibkr
description: "Printing Press CLI for Ibkr. The IB REST API reference documentation"
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ibkr-pp-cli
---

# Ibkr — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ibkr-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ibkr --cli-only
   ```
2. Verify: `ibkr-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The IB REST API reference documentation

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`ibkr-pp-cli iserver get-brokerage-accounts`** — Wraps all 166 Client Portal API endpoints vs narrow trading-focused subsets in alternatives
- **`ibkr-pp-cli iserver get-account-summary --agent`** — --json, --compact, --no-input, --agent flags for use in AI agent pipelines
- **`ibkr-pp-cli sync`** — sync command downloads API data to SQLite for offline querying
- **`ibkr-pp-cli agent-context`** — ibkr-pp-mcp binary exposes all API operations as MCP tools for Claude and other LLM clients
- **`ibkr-pp-cli tail`** — Stream live changes by polling the API at regular intervals

## Command Reference

**acesws** — Manage acesws


**contract** — Manage contract

- `ibkr-pp-cli contract` — Returns the trading schedule for the 6 total days surrounding the current trading day. Non-Trading days, such as...

**fa** — Manage fa

- `ibkr-pp-cli fa get-accounts-in-model` — Request all accounts held within a model.
- `ibkr-pp-cli fa get-all-model-positions` — Request all positions held within the model.
- `ibkr-pp-cli fa get-allmodels` — Retrieve summaries for all models under the advisor account.
- `ibkr-pp-cli fa get-invested-accounts-in-model` — Request the list of all accounts already invested in the provided model and a summary of their investment.
- `ibkr-pp-cli fa get-model-presets` — Get the preset behavior for model rebalancing.
- `ibkr-pp-cli fa get-model-summary-single` — Request a summary for a single model.
- `ibkr-pp-cli fa set-accountinvestment-in-model` — Assign an account and the amount of cash to allocate into a model.
- `ibkr-pp-cli fa set-model-presets` — Set the preset behavior for models.
- `ibkr-pp-cli fa set-model-target-positions` — Create or Modify a model's target positions.
- `ibkr-pp-cli fa submit-model-orders` — Submit all pending orders to the models. This is similar to the Model page's Submit All Orders selection.

**forecast** — Manage forecast

- `ibkr-pp-cli forecast get-categories` — Returns the category names, parent ids, and markets for Event Contracts.
- `ibkr-pp-cli forecast get-contract` — Provides instrument details for the specific forecast contract.
- `ibkr-pp-cli forecast get-markets` — Returns all high level contract details affiliated with the underlying market conid provided.
- `ibkr-pp-cli forecast get-rules` — Provides trading rules for specific event contracts.
- `ibkr-pp-cli forecast get-schedule` — Provides forecast trading schedules.

**fyi** — Manage fyi

- `ibkr-pp-cli fyi delete-device` — Delete a specific device from our saved list of notification devices.
- `ibkr-pp-cli fyi get-all` — Get a list of available notifications.
- `ibkr-pp-cli fyi get-delivery` — Options for sending fyis to email and other devices.
- `ibkr-pp-cli fyi get-disclaimerss` — Receive additional disclaimers based on the specified typecode.
- `ibkr-pp-cli fyi get-settings` — Return the current choices of subscriptions for notifications.
- `ibkr-pp-cli fyi get-unread` — Returns the total number of unread notifications
- `ibkr-pp-cli fyi modify-delivery` — Choose whether a particular device is enabled or disabled.
- `ibkr-pp-cli fyi modify-emails` — Enable or disable your account's primary email to receive notifications.
- `ibkr-pp-cli fyi modify-notification` — Enable or disable group of notifications by the specific typecode.
- `ibkr-pp-cli fyi read-disclaimer` — Mark a specific disclaimer message as read.
- `ibkr-pp-cli fyi read-notification` — Mark a particular notification message as read or unread.

**gw** — Manage gw

- `ibkr-pp-cli gw apply-csv` — Applies previously verified CSV changes. Requires two tokens: **Authorization header** — RS256-signed JWT...
- `ibkr-pp-cli gw create` — Submit account application. This will create brokerage account for the end user.<br><br>**Scope**:...
- `ibkr-pp-cli gw create-api` — Create or delete bank instructions by accountId. Only ACH and EDDA are supported for 'Create'.<br><br>**Scope**:...
- `ibkr-pp-cli gw create-api-10` — <br>**Scope**: `statements.write` OR `reports.write`<br>**Security Policy**: `Signed JWT`
- `ibkr-pp-cli gw create-api-11` — <br>**Scope**: `statements.write` OR `reports.write`<br>**Security Policy**: `Signed JWT`
- `ibkr-pp-cli gw create-api-12` — Provides mechanism to submit Agreements and Disclosures to IBKR once a day instead of with each application. We...
- `ibkr-pp-cli gw create-api-13` — View available cash for withdrawal and account equity value by accountId<br><br>**Scope**:...
- `ibkr-pp-cli gw create-api-14` — View active bank instructions for an accountId.<br><br>**Scope**: `bank-instructions.read`<br>**Security Policy**:...
- `ibkr-pp-cli gw create-api-15` — Echo A Request With Signed JWT Security Policy Back After Validation.
- `ibkr-pp-cli gw create-api-16` — View available cash for withdrawal with and without margin loan by accountId<br><br>**Scope**:...
- `ibkr-pp-cli gw create-api-17` — Cancel request by instructionId.<br><br>**Scope**: `instructions.write`<br>**Security Policy**: `Signed JWT`
- `ibkr-pp-cli gw create-api-18` — <br>**Scope**: `instructions.write`<br>**Security Policy**: `Signed JWT`
- `ibkr-pp-cli gw create-api-19` — Query list of recent transactions (up to 30 days) based on accountId.<br><br>**Scope**:...
- `ibkr-pp-cli gw create-api-2` — Creates Multiple Banking Instructions(ach, Delete, Micro-amount, Predefined-destination-instruction)
- `ibkr-pp-cli gw create-api-20` — Transfer positions internally between two accounts with Interactive Brokers<br><br>**Scope**:...
- `ibkr-pp-cli gw create-api-21` — Creates Multiple Internal Asset Transfers Between The Provided Account Id Pairs
- `ibkr-pp-cli gw create-api-22` — Transfer cash internally between two accounts with Interactive Brokers.<br><br>**Scope**:...
- `ibkr-pp-cli gw create-api-23` — Creates Multiple Internal Cash Transfers Between The Provided Account Id Pairs
- `ibkr-pp-cli gw create-api-3` — Initiate request to submit external position transfer. Methods- ACATS, ATON, Basic FOP, FOP, DWAC. More information...
- `ibkr-pp-cli gw create-api-4` — Creates Multiple External Asset Transfers (Fop, DWAC And Complex Asset Transfer)
- `ibkr-pp-cli gw create-api-5` — Initiate request to deposit or withdrawal between IBKR account and bank account. More information on transfer...
- `ibkr-pp-cli gw create-api-6` — <br>**Scope**: `transfers.write`<br>**Security Policy**: `Signed JWT`
- `ibkr-pp-cli gw create-api-7` — <br>**Scope**: `sso-browser-sessions.write`<br>**Security Policy**: `Signed JWT`
- `ibkr-pp-cli gw create-api-8` — <br>**Scope**: `sso-sessions.write`<br>**Security Policy**: `Signed JWT`
- `ibkr-pp-cli gw create-api-9` — <br>**Scope**: `statements.read` OR `statements.write` OR `reports.write`<br>**Security Policy**: `Signed JWT`
- `ibkr-pp-cli gw create-tax-voucher-requests` — Creates tax voucher requests from a signed JWT payload. The request body must be a signed OAuth2 JWT (text/plain)...
- `ibkr-pp-cli gw download-file` — Downloads the tax voucher PDF file for the given tax voucher request ID.<br><br>**Scope**:...
- `ibkr-pp-cli gw fetch-dividends-1` — Retrieves dividend details for the given customer account, year, and country.<br><br>**Scope**:...
- `ibkr-pp-cli gw get` — Retrieve status of request by clientInstructionId.<br><br>**Scope**: `instructions.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw get-account-restrictions` — Returns all restriction IDs for the given account. The account must be the caller's master account or a direct...
- `ibkr-pp-cli gw get-active-country-list` — Returns the list of active countries eligible for tax voucher requests, including applicable fees and...
- `ibkr-pp-cli gw get-api` — Used to query list of enumerations for attributes included within extPositionsTransfers, occupation,...
- `ibkr-pp-cli gw get-api-10` — Verify whether user is valid and available<br><br>**Scope**: `accounts.read` OR `validations.read`<br>**Security...
- `ibkr-pp-cli gw get-api-2` — Retrieve status of all requests associated with instructionSetId.<br><br>**Scope**:...
- `ibkr-pp-cli gw get-api-3` — Retrieve status of request by instructionId<br><br>**Scope**: `instructions.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw get-api-4` — <br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw get-api-5` — Generate URL address to complete real-time KYC verification using Au10Tix<br><br>**Scope**:...
- `ibkr-pp-cli gw get-api-6` — Query login messages assigned by accountId<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw get-api-7` — Query status of account by accountId<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw get-api-8` — Query registration tasks assigned to account and pending tasks that are required for account...
- `ibkr-pp-cli gw get-api-9` — Returns status for account management request<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw get-current-state-1` — Returns the current state of a tax voucher request by tax voucher request ID.<br><br>**Scope**:...
- `ibkr-pp-cli gw get-list-details` — Returns list metadata and instrument entries. The list must be owned by the caller. Returns 403 for both...
- `ibkr-pp-cli gw get-restriction-details` — Returns full details of a restriction including rules and approver users. The restriction must be owned by the...
- `ibkr-pp-cli gw get-user-restrictions` — Returns all restriction IDs for the given username. The username must be the master user, a sub-user, or a user with...
- `ibkr-pp-cli gw get-years` — Returns the list of tax years available for tax voucher requests<br><br>**Scope**: `tax-vouchers.read`<br>**Security...
- `ibkr-pp-cli gw list` — Retrieve the application request and IBKR response data based on IBKR accountId or externalId. Only available for...
- `ibkr-pp-cli gw list-api` — Get forms<br><br>**Scope**: `accounts.read` OR `forms.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw list-api-10` — Fetch List Of Available Tax Reports/forms/documents For A Specified Account And Tax Year
- `ibkr-pp-cli gw list-api-11` — <br>**Scope**: `statements.read` OR `reports.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw list-api-2` — Get list of banks which support banking connection with Interactive Brokers.<br><br>**Scope**:...
- `ibkr-pp-cli gw list-api-3` — Fetch Requests' Details By Timeframe<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw list-api-4` — Query all accounts associated with ‘Client ID’ that have incomplete login message<br><br>**Scope**:...
- `ibkr-pp-cli gw list-api-5` — Query status of all accounts associated with ‘Client ID'<br><br>**Scope**: `accounts.read`<br>**Security Policy**:...
- `ibkr-pp-cli gw list-api-6` — Echo A Request With HTTPS Security Policy Back After Validation.
- `ibkr-pp-cli gw list-api-7` — Get list of brokers supported for given asset transfer type<br><br>**Scope**: `enumerations.read`<br>**Security...
- `ibkr-pp-cli gw list-api-8` — Get required forms<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw list-api-9` — <br>**Scope**: `statements.read` OR `reports.read`<br>**Security Policy**: `HTTPS`
- `ibkr-pp-cli gw update` — Update information for an existing accountId<br><br>**Scope**: `accounts.write`<br>**Security Policy**: `Signed JWT`
- `ibkr-pp-cli gw verify-csv` — Validates CSV content without applying changes. Must be called before `/csv/v2/apply` with the same `requestId`. The...

**iserver** — Manage iserver

- `ibkr-pp-cli iserver ack-server-prompt` — Respond to a server prompt received via ntf websocket message.
- `ibkr-pp-cli iserver activate-alert` — Activate or Deactivate existing alerts created for this account. This does not delete alerts, but disables...
- `ibkr-pp-cli iserver cancel-open-order` — Cancel an existing, unfilled order.
- `ibkr-pp-cli iserver close-all-md-streams` — Instruct IServer to close all of its open backend data streams for all instruments.
- `ibkr-pp-cli iserver close-md-stream` — Instruct IServer to close its backend stream for the instrument when real-time snapshots are no longer needed.
- `ibkr-pp-cli iserver confirm-order-reply` — Confirm an order reply message and continue with submission of order ticket.
- `ibkr-pp-cli iserver create-alert` — Endpoint used to create a new alert, or modify an existing alert.
- `ibkr-pp-cli iserver create-allocation-group` — Add a new allocation group. This group can be used to trade in place of the {accountId} for the...
- `ibkr-pp-cli iserver delete-alert` — Permanently delete an existing alert. Deleting an MTA alert will reset it to the default state.
- `ibkr-pp-cli iserver delete-allocation-group` — Deletes a previously created allocation group. This endpoint is only supported for Financial Advisors and IBroker...
- `ibkr-pp-cli iserver delete-watchlist` — Delete a specified watchlist from the username's settings.
- `ibkr-pp-cli iserver get-account-market-summary` — Returns a summary of an account's market value, by currency and asset class.
- `ibkr-pp-cli iserver get-account-summary` — Provides a general overview of the account details such as balance values.
- `ibkr-pp-cli iserver get-alert-details` — Request details of a specific alert by providing the assigned alertId Id.
- `ibkr-pp-cli iserver get-algos-by-instrument` — Returns supported IB Algos for an instrument. A pre-flight request must be submitted before retrieving information.
- `ibkr-pp-cli iserver get-all-alerts` — Retrieve a list of all alerts attached to the provided account.
- `ibkr-pp-cli iserver get-all-watchlists` — Returns all saved watchlists stored on IB backend for the username in use in the current Web API session.
- `ibkr-pp-cli iserver get-allocatable-subaccounts` — Retrieves a list of all sub-accounts and returns their net liquidity and available equity for advisors to make...
- `ibkr-pp-cli iserver get-allocation-groups` — Retrieves a list of all of the advisor's allocation groups. This describes the name of the allocation group, number...
- `ibkr-pp-cli iserver get-allocation-presets` — Retrieve the preset behavior for allocation groups for specific events. This endpoint is only supported for...
- `ibkr-pp-cli iserver get-balance-summary` — Returns a summary of an account's equity and cash balances, in total and by account segment.
- `ibkr-pp-cli iserver get-bond-filters` — Request a list of filters relating to a given Bond issuerID. The issuerId is retrieved from /iserver/secdef/search...
- `ibkr-pp-cli iserver get-brokerage-accounts` — Returns a list of accounts the user has trading access to, their respective aliases and the currently selected...
- `ibkr-pp-cli iserver get-brokerage-status` — Current Authentication status to the Brokerage system. Market Data and Trading is not possible if not authenticated.
- `ibkr-pp-cli iserver get-contract-info` — Returns the attributes of the instrument.
- `ibkr-pp-cli iserver get-contract-rules` — Returns trading related rules for a specific contract and side.
- `ibkr-pp-cli iserver get-contract-strikes` — Returns lists of valid strikes for options contracts on a given underlier, for all currently trading expirations....
- `ibkr-pp-cli iserver get-contract-symbols` — Returns a list of contracts based on the search symbol provided as a query param.
- `ibkr-pp-cli iserver get-contract-symbols-from-body` — Returns a list of contracts based on the search symbol provided as a query param.
- `ibkr-pp-cli iserver get-currency-pairs` — Obtains available currency pairs corresponding to the given target currency.
- `ibkr-pp-cli iserver get-dynamic-accounts` — Returns a list of accounts matching a query pattern set in the request. Broker accounts configured with the DYNACCT...
- `ibkr-pp-cli iserver get-exchange-rates` — Obtains the exchange rates of the currency pair.
- `ibkr-pp-cli iserver get-fund-summary` — Provides a summary specific for avilable funds giving more depth than the standard /summary endpoint.
- `ibkr-pp-cli iserver get-info-and-rules` — Requests full contract details and trading rules for the given conid. A follow-up request will provide additional...
- `ibkr-pp-cli iserver get-instrument-info` — Requests full contract details for the given conid.
- `ibkr-pp-cli iserver get-margin-summary` — Returns a summary of an account's margin, in total and by account segment.
- `ibkr-pp-cli iserver get-md-history` — Request historical data for an instrument in the form of OHLC bars.
- `ibkr-pp-cli iserver get-md-snapshot` — Get Market Data for the given conid(s). A pre-flight request must be made prior to ever receiving data. For some...
- `ibkr-pp-cli iserver get-mta-details` — Retrieve information about your MTA alert. Each login user only has one mobile trading assistant (MTA) alert with...
- `ibkr-pp-cli iserver get-open-orders` — Returns open orders and filled or cancelled orders submitted during the current brokerage session.
- `ibkr-pp-cli iserver get-order-status` — Retrieve the status of a single order. Only displays orders from the current brokerage session. If orders executed...
- `ibkr-pp-cli iserver get-pnl` — Returns updated profit and loss values for the selected account. Initial request will return an empty array in the...
- `ibkr-pp-cli iserver get-scanner-parameters` — Returns an xml file containing all available parameters to be sent for the Iserver scanner request.
- `ibkr-pp-cli iserver get-scanner-results` — Searches for contracts according to the filters specified in /iserver/scanner/params endpoint.
- `ibkr-pp-cli iserver get-single-allocation-group` — Retrieves the configuration of a single account group. This describes the name of the allocation group, the specific...
- `ibkr-pp-cli iserver get-specific-watchlist` — Retrieve details of a single watchlist stored in the username's settings.
- `ibkr-pp-cli iserver get-trade-history` — Retrieve a list of trades, up to a maximum of 7 days prior.
- `ibkr-pp-cli iserver initialize-session` — After retrieving the access token and subsequent Live Session Token, customers can initialize their brokerage...
- `ibkr-pp-cli iserver modify-allocation-group` — Modify an existing allocation group.
- `ibkr-pp-cli iserver modify-open-order` — Modify an existing, unfilled order.
- `ibkr-pp-cli iserver post-new-watchlist` — Create a named watchlist by submitting a set of conids.
- `ibkr-pp-cli iserver preview-margin-impact` — Preview the projected effects of an order ticket or bracket of orders, including cost and changes to margin and...
- `ibkr-pp-cli iserver reset-order-suppression` — Removes suppression of all order reply messages that were previously suppressed in the current brokerage session.
- `ibkr-pp-cli iserver set-active-account` — Switch the active account for how you request data. Only available for financial advisors and multi-account structures.
- `ibkr-pp-cli iserver set-allocation-preset` — Set the preset behavior for new allocation groups for specific events.
- `ibkr-pp-cli iserver set-dynamic-account` — Set the active dynamic account.
- `ibkr-pp-cli iserver submit-new-order` — Submit a new order(s) ticket, bracket, or OCA group.
- `ibkr-pp-cli iserver suppress-order-replies` — Suppress the specified order reply messages for the duration of the brokerage session.

**logout** — Manage logout

- `ibkr-pp-cli logout` — Logs the user out of the gateway session. Any further activity requires re-authentication. Discard client-side...

**oauth** — Manage oauth

- `ibkr-pp-cli oauth req-access-token` — Request an access token for the IB username that has granted authorization to the consumer.
- `ibkr-pp-cli oauth req-live-session-token` — Generate a Live Session Token shared secret and gain access to Web API.
- `ibkr-pp-cli oauth req-temp-token` — Request a temporary token as a third party to begin the OAuth 1.0a authorization workflow.

**oauth2** — Manage oauth2

- `ibkr-pp-cli oauth2` — Generate OAuth 2.0 access tokens based on request parameters.

**pa** — Manage pa

- `ibkr-pp-cli pa create` — Returns consolidated portfolio allocation by Financial Instrument, Asset Class, Sector, Region, or Country for a...
- `ibkr-pp-cli pa get-performance-all-periods` — Returns the performance (MTM) for the given accounts, if more than one account is passed, the result is consolidated.
- `ibkr-pp-cli pa get-single-performance-period` — Returns the performance (MTM) for the given accounts, if more than one account is passed, the result is consolidated.
- `ibkr-pp-cli pa get-transactions` — Transaction history for a given number of conids and accounts. Types of transactions include dividend payments, buy...

**portfolio** — Manage portfolio

- `ibkr-pp-cli portfolio get-all-accounts` — List All Accounts
- `ibkr-pp-cli portfolio get-all-accounts-for-conid` — Get positions in accounts for a given instrument (no secDef await control)
- `ibkr-pp-cli portfolio get-all-subaccounts` — Retrieve attributes of the subaccounts in the account structure.
- `ibkr-pp-cli portfolio get-many-subaccounts` — Used in tiered account structures (such as Financial Advisor and IBroker Accounts) to return a list of sub-accounts,...

**portfolio2** — Manage portfolio2


**sso** — Manage sso

- `ibkr-pp-cli sso` — Validates the current session for the SSO user.

**tickle** — Manage tickle

- `ibkr-pp-cli tickle` — If the gateway has not received any requests for several minutes an open session will automatically timeout. The...

**trsrv** — Manage trsrv

- `ibkr-pp-cli trsrv get-conids-by-exchange` — Send out a request to retrieve all contracts made available on a requested exchange. This returns all contracts that...
- `ibkr-pp-cli trsrv get-future-by-symbol` — Returns a list of non-expired future contracts for given symbol(s)
- `ibkr-pp-cli trsrv get-instrument-definition` — Returns a list of security definitions for the given conids.
- `ibkr-pp-cli trsrv get-stock-by-symbol` — Returns an object contains all stock contracts for given symbol(s)
- `ibkr-pp-cli trsrv get-trading-schedule` — Returns the trading schedule up to a month for the requested contract.

**ws** — Manage ws

- `ibkr-pp-cli ws` — Open websocket.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ibkr-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `ibkr-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ibkr-pp-cli gw list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ibkr-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ibkr-pp-cli feedback --stdin < notes.txt
ibkr-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ibkr-pp-cli/feedback.jsonl`. They are never POSTed unless `IBKR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IBKR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
ibkr-pp-cli profile save briefing --json
ibkr-pp-cli --profile briefing gw list
ibkr-pp-cli profile list --json
ibkr-pp-cli profile show briefing
ibkr-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `ibkr-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ibkr-pp-mcp -- ibkr-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ibkr-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ibkr-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ibkr-pp-cli <command> --help`.
