# Ibkr CLI

The IB REST API reference documentation

Learn more at [Ibkr](https://ibkrcampus.com/ibkr-api-page/ibkr-api-home/).

## Install

The recommended path installs both the `ibkr-pp-cli` binary and the `pp-ibkr` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install ibkr
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install ibkr --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ibkr-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ibkr --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ibkr --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-ibkr skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-ibkr. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
ibkr-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
ibkr-pp-cli gw list
```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`ibkr-pp-cli iserver get-brokerage-accounts`** — Wraps all 166 Client Portal API endpoints vs narrow trading-focused subsets in alternatives
- **`ibkr-pp-cli iserver get-account-summary --agent`** — --json, --compact, --no-input, --agent flags for use in AI agent pipelines
- **`ibkr-pp-cli sync`** — sync command downloads API data to SQLite for offline querying
- **`ibkr-pp-cli agent-context`** — ibkr-pp-mcp binary exposes all API operations as MCP tools for Claude and other LLM clients
- **`ibkr-pp-cli tail`** — Stream live changes by polling the API at regular intervals

## Usage

Run `ibkr-pp-cli --help` for the full command reference and flag list.

## Commands

### acesws

Manage acesws


### contract

Manage contract

- **`ibkr-pp-cli contract get-trading-schedule`** - Returns the trading schedule for the 6 total days surrounding the current trading day. Non-Trading days, such as holidays, will not be returned.

### fa

Manage fa

- **`ibkr-pp-cli fa get-accounts-in-model`** - Request all accounts held within a model.
- **`ibkr-pp-cli fa get-all-model-positions`** - Request all positions held within the model.
- **`ibkr-pp-cli fa get-allmodels`** - Retrieve summaries for all models under the advisor account.
- **`ibkr-pp-cli fa get-invested-accounts-in-model`** - Request the list of all accounts already invested in the provided model and a summary of their investment.
- **`ibkr-pp-cli fa get-model-presets`** - Get the preset behavior for model rebalancing.
- **`ibkr-pp-cli fa get-model-summary-single`** - Request a summary for a single model.
- **`ibkr-pp-cli fa set-accountinvestment-in-model`** - Assign an account and the amount of cash to allocate into a model.
- **`ibkr-pp-cli fa set-model-presets`** - Set the preset behavior for models.
- **`ibkr-pp-cli fa set-model-target-positions`** - Create or Modify a model's target positions.
- **`ibkr-pp-cli fa submit-model-orders`** - Submit all pending orders to the models. This is similar to the Model page's Submit All Orders selection.

### forecast

Manage forecast

- **`ibkr-pp-cli forecast get-categories`** - Returns the category names, parent ids, and markets for Event Contracts.
- **`ibkr-pp-cli forecast get-contract`** - Provides instrument details for the specific forecast contract.
- **`ibkr-pp-cli forecast get-markets`** - Returns all high level contract details affiliated with the underlying market conid provided.
- **`ibkr-pp-cli forecast get-rules`** - Provides trading rules for specific event contracts.
- **`ibkr-pp-cli forecast get-schedule`** - Provides forecast trading schedules.

### fyi

Manage fyi

- **`ibkr-pp-cli fyi delete-device`** - Delete a specific device from our saved list of notification devices.
- **`ibkr-pp-cli fyi get-all`** - Get a list of available notifications.
- **`ibkr-pp-cli fyi get-delivery`** - Options for sending fyis to email and other devices.
- **`ibkr-pp-cli fyi get-disclaimerss`** - Receive additional disclaimers based on the specified typecode.
- **`ibkr-pp-cli fyi get-settings`** - Return the current choices of subscriptions for notifications.
- **`ibkr-pp-cli fyi get-unread`** - Returns the total number of unread notifications
- **`ibkr-pp-cli fyi modify-delivery`** - Choose whether a particular device is enabled or disabled.
- **`ibkr-pp-cli fyi modify-emails`** - Enable or disable your account's primary email to receive notifications.
- **`ibkr-pp-cli fyi modify-notification`** - Enable or disable group of notifications by the specific typecode.
- **`ibkr-pp-cli fyi read-disclaimer`** - Mark a specific disclaimer message as read.
- **`ibkr-pp-cli fyi read-notification`** - Mark a particular notification message as read or unread.

### gw

Manage gw

- **`ibkr-pp-cli gw apply-csv`** - Applies previously verified CSV changes. Requires two tokens:

**Authorization header** — RS256-signed JWT containing an `accountId` claim, used to identify the master account. A missing or invalid token does not return HTTP 401; the request proceeds and fails at validation with `success: false`.

**Request body** — a separate RS256-signed JWT (validity: 1 minute) whose payload contains the request claims (`userName`, `requestId`, `payload`, and optionally `isEmpTrack`). The `requestId` must match a prior successful `/csv/v2/verify` call.

Failures that occur before the body JWT payload is parsed (inactive token, missing payload) return `success: false` without a `requestId` field.<br><br>**Scope**: `restrictions.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create`** - Submit account application. This will create brokerage account for the end user.<br><br>**Scope**: `accounts.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api`** - Create or delete bank instructions by accountId. Only ACH and EDDA are supported for 'Create'.<br><br>**Scope**: `bank-instructions.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-10`** - <br>**Scope**: `statements.write` OR `reports.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-11`** - <br>**Scope**: `statements.write` OR `reports.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-12`** - Provides mechanism to submit Agreements and Disclosures to IBKR once a day instead of with each application. We store these documents on the servers and will use them for new application requests submitted that day.<ul><li>Documents will need to be submitted once a day (before the Applications are submitted). PDFs will be displayed and submitted as is- no changes/edits will be made to the actual PDF files.</li><li>This end-point will not process any Tax Form Documents. Tax Form document should be submitted with every application</li><li>If submitted in the morning, you only need to include the Tax Form attachment for each applicant. Otherwise, you will need to include PDFs with each application (Create Account).</li></ul><br><br>**Scope**: `accounts.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-13`** - View available cash for withdrawal and account equity value by accountId<br><br>**Scope**: `balances.read`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-14`** - View active bank instructions for an accountId.<br><br>**Scope**: `bank-instructions.read`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-15`** - Echo A Request With Signed JWT Security Policy Back After Validation.
- **`ibkr-pp-cli gw create-api-16`** - View available cash for withdrawal with and without margin loan by accountId<br><br>**Scope**: `transfers.read`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-17`** - Cancel request by instructionId.<br><br>**Scope**: `instructions.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-18`** - <br>**Scope**: `instructions.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-19`** - Query list of recent transactions (up to 30 days) based on accountId.<br><br>**Scope**: `instructions.read`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-2`** - Creates Multiple Banking Instructions(ach, Delete, Micro-amount, Predefined-destination-instruction)
- **`ibkr-pp-cli gw create-api-20`** - Transfer positions internally between two accounts with Interactive Brokers<br><br>**Scope**: `transfers.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-21`** - Creates Multiple Internal Asset Transfers Between The Provided Account Id Pairs
- **`ibkr-pp-cli gw create-api-22`** - Transfer cash internally between two accounts with Interactive Brokers.<br><br>**Scope**: `transfers.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-23`** - Creates Multiple Internal Cash Transfers Between The Provided Account Id Pairs
- **`ibkr-pp-cli gw create-api-3`** - Initiate request to submit external position transfer. Methods- ACATS, ATON, Basic FOP, FOP, DWAC. More information on transfer methods can be found here - https://www.interactivebrokers.com/campus/trading-lessons/cash-and-position-transfers/<br><br>**Scope**: `transfers.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-4`** - Creates Multiple External Asset Transfers (Fop, DWAC And Complex Asset Transfer)
- **`ibkr-pp-cli gw create-api-5`** - Initiate request to deposit or withdrawal between IBKR account and bank account. More information on transfer methods can be found here - https://www.interactivebrokers.com/campus/trading-lessons/cash-and-position-transfers<br><br>**Scope**: `transfers.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-6`** - <br>**Scope**: `transfers.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-7`** - <br>**Scope**: `sso-browser-sessions.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-8`** - <br>**Scope**: `sso-sessions.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-api-9`** - <br>**Scope**: `statements.read` OR `statements.write` OR `reports.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw create-tax-voucher-requests`** - Creates tax voucher requests from a signed JWT payload. The request body must be a signed OAuth2 JWT (text/plain) whose decoded payload contains the list of tax voucher requests.<br><br>**Scope**: `tax-vouchers.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw download-file`** - Downloads the tax voucher PDF file for the given tax voucher request ID.<br><br>**Scope**: `tax-vouchers.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw fetch-dividends-1`** - Retrieves dividend details for the given customer account, year, and country.<br><br>**Scope**: `tax-vouchers.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get`** - Retrieve status of request by clientInstructionId.<br><br>**Scope**: `instructions.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-account-restrictions`** - Returns all restriction IDs for the given account. The account must be the caller's master account or a direct sub-account. Returns 403 for both unauthorized and non-existent accounts to prevent ID enumeration.<br><br>**Scope**: `restrictions.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-active-country-list`** - Returns the list of active countries eligible for tax voucher requests, including applicable fees and currencies<br><br>**Scope**: `tax-vouchers.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api`** - Used to query list of enumerations for attributes included within extPositionsTransfers, occupation, employerBusiness, financialInformation, affiliationDetails, tradingPermissions, etc.<br><br>**Scope**: `accounts.read` OR `enumerations.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api-10`** - Verify whether user is valid and available<br><br>**Scope**: `accounts.read` OR `validations.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api-2`** - Retrieve status of all requests associated with instructionSetId.<br><br>**Scope**: `instructions.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api-3`** - Retrieve status of request by instructionId<br><br>**Scope**: `instructions.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api-4`** - <br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api-5`** - Generate URL address to complete real-time KYC verification using Au10Tix<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api-6`** - Query login messages assigned by accountId<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api-7`** - Query status of account by accountId<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api-8`** - Query registration tasks assigned to account and pending tasks that are required for account approval<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-api-9`** - Returns status for account management request<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-current-state-1`** - Returns the current state of a tax voucher request by tax voucher request ID.<br><br>**Scope**: `tax-vouchers.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-list-details`** - Returns list metadata and instrument entries. The list must be owned by the caller. Returns 403 for both unauthorized and non-existent lists to prevent ID enumeration.<br><br>**Scope**: `restrictions.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-restriction-details`** - Returns full details of a restriction including rules and approver users. The restriction must be owned by the caller. Returns 403 for both unauthorized and non-existent restrictions to prevent ID enumeration.<br><br>**Scope**: `restrictions.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-user-restrictions`** - Returns all restriction IDs for the given username. The username must be the master user, a sub-user, or a user with trading access under the master account. Returns 403 for both unauthorized and non-existent users to prevent ID enumeration.<br><br>**Scope**: `restrictions.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw get-years`** - Returns the list of tax years available for tax voucher requests<br><br>**Scope**: `tax-vouchers.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list`** - Retrieve the application request and IBKR response data based on IBKR accountId or externalId. Only available for accounts that originate via API<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list-api`** - Get forms<br><br>**Scope**: `accounts.read` OR `forms.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list-api-10`** - Fetch List Of Available Tax Reports/forms/documents For A Specified Account And Tax Year
- **`ibkr-pp-cli gw list-api-11`** - <br>**Scope**: `statements.read` OR `reports.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list-api-2`** - Get list of banks which support banking connection with Interactive Brokers.<br><br>**Scope**: `enumerations.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list-api-3`** - Fetch Requests' Details By Timeframe<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list-api-4`** - Query all accounts associated with ‘Client ID’ that have incomplete login message<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list-api-5`** - Query status of all accounts associated with ‘Client ID'<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list-api-6`** - Echo A Request With HTTPS Security Policy Back After Validation.
- **`ibkr-pp-cli gw list-api-7`** - Get list of brokers supported for given asset transfer type<br><br>**Scope**: `enumerations.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list-api-8`** - Get required forms<br><br>**Scope**: `accounts.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw list-api-9`** - <br>**Scope**: `statements.read` OR `reports.read`<br>**Security Policy**: `HTTPS`
- **`ibkr-pp-cli gw update`** - Update information for an existing accountId<br><br>**Scope**: `accounts.write`<br>**Security Policy**: `Signed JWT`
- **`ibkr-pp-cli gw verify-csv`** - Validates CSV content without applying changes. Must be called before `/csv/v2/apply` with the same `requestId`.

The master account identity is resolved from the `accountId` claim in the Authorization header JWT. Do not include `masterAcctId` in the request body.

A missing or invalid Authorization header does not return HTTP 401. The request proceeds and fails at validation with `success: false, message: "Master account not specified and not emp track"` unless `isEmpTrack: T` is set in the body.<br><br>**Scope**: `restrictions.write`<br>**Security Policy**: `Signed JWT`

### iserver

Manage iserver

- **`ibkr-pp-cli iserver ack-server-prompt`** - Respond to a server prompt received via ntf websocket message.
- **`ibkr-pp-cli iserver activate-alert`** - Activate or Deactivate existing alerts created for this account. This does not delete alerts, but disables notifications until reactivated.
- **`ibkr-pp-cli iserver cancel-open-order`** - Cancel an existing, unfilled order.
- **`ibkr-pp-cli iserver close-all-md-streams`** - Instruct IServer to close all of its open backend data streams for all instruments.
- **`ibkr-pp-cli iserver close-md-stream`** - Instruct IServer to close its backend stream for the instrument when real-time snapshots are no longer needed.
- **`ibkr-pp-cli iserver confirm-order-reply`** - Confirm an order reply message and continue with submission of order ticket.
- **`ibkr-pp-cli iserver create-alert`** - Endpoint used to create a new alert, or modify an existing alert.
- **`ibkr-pp-cli iserver create-allocation-group`** - Add a new allocation group. This group can be used to trade in place of the {accountId} for the /iserver/account/{accountId}/orders endpoint.
- **`ibkr-pp-cli iserver delete-alert`** - Permanently delete an existing alert. Deleting an MTA alert will reset it to the default state.
- **`ibkr-pp-cli iserver delete-allocation-group`** - Deletes a previously created allocation group. This endpoint is only supported for Financial Advisors and IBroker Accounts.
- **`ibkr-pp-cli iserver delete-watchlist`** - Delete a specified watchlist from the username's settings.
- **`ibkr-pp-cli iserver get-account-market-summary`** - Returns a summary of an account's market value, by currency and asset class.
- **`ibkr-pp-cli iserver get-account-summary`** - Provides a general overview of the account details such as balance values.
- **`ibkr-pp-cli iserver get-alert-details`** - Request details of a specific alert by providing the assigned alertId Id.
- **`ibkr-pp-cli iserver get-algos-by-instrument`** - Returns supported IB Algos for an instrument. A pre-flight request must be submitted before retrieving information.
- **`ibkr-pp-cli iserver get-all-alerts`** - Retrieve a list of all alerts attached to the provided account.
- **`ibkr-pp-cli iserver get-all-watchlists`** - Returns all saved watchlists stored on IB backend for the username in use in the current Web API session.
- **`ibkr-pp-cli iserver get-allocatable-subaccounts`** - Retrieves a list of all sub-accounts and returns their net liquidity and available equity for advisors to make decisions on what accounts should be allocated and how. This endpoint is only supported for Financial Advisors and IBroker Accounts.
- **`ibkr-pp-cli iserver get-allocation-groups`** - Retrieves a list of all of the advisor's allocation groups. This describes the name of the allocation group, number of subaccounts within the group, and the method in use for the group. This endpoint is only supported for Financial Advisors and IBroker Accounts.
- **`ibkr-pp-cli iserver get-allocation-presets`** - Retrieve the preset behavior for allocation groups for specific events. This endpoint is only supported for Financial Advisors and IBroker Accounts.
- **`ibkr-pp-cli iserver get-balance-summary`** - Returns a summary of an account's equity and cash balances, in total and by account segment.
- **`ibkr-pp-cli iserver get-bond-filters`** - Request a list of filters relating to a given Bond issuerID. The issuerId is retrieved from /iserver/secdef/search and can be used in /iserver/secdef/info?issuerId={issuerId} for retrieving conIds.
- **`ibkr-pp-cli iserver get-brokerage-accounts`** - Returns a list of accounts the user has trading access to, their respective aliases and the currently selected account. Note this endpoint must be called before modifying an order or querying open orders.
- **`ibkr-pp-cli iserver get-brokerage-status`** - Current Authentication status to the Brokerage system. Market Data and Trading is not possible if not authenticated.
- **`ibkr-pp-cli iserver get-contract-info`** - Returns the attributes of the instrument.
- **`ibkr-pp-cli iserver get-contract-rules`** - Returns trading related rules for a specific contract and side.
- **`ibkr-pp-cli iserver get-contract-strikes`** - Returns lists of valid strikes for options contracts on a given underlier, for all currently trading expirations. The /iserver/secdef/search endpoint must be called prior for the underlying. Otherwise empty arrays will return for "puts" and "calls".
- **`ibkr-pp-cli iserver get-contract-symbols`** - Returns a list of contracts based on the search symbol provided as a query param.
- **`ibkr-pp-cli iserver get-contract-symbols-from-body`** - Returns a list of contracts based on the search symbol provided as a query param.
- **`ibkr-pp-cli iserver get-currency-pairs`** - Obtains available currency pairs corresponding to the given target currency.
- **`ibkr-pp-cli iserver get-dynamic-accounts`** - Returns a list of accounts matching a query pattern set in the request. Broker accounts configured with the DYNACCT property will not receive account information at login. Instead, they must dynamically query then set their account number. Customers without the DYNACCT property will receive a 503 error.
- **`ibkr-pp-cli iserver get-exchange-rates`** - Obtains the exchange rates of the currency pair.
- **`ibkr-pp-cli iserver get-fund-summary`** - Provides a summary specific for avilable funds giving more depth than the standard /summary endpoint.
- **`ibkr-pp-cli iserver get-info-and-rules`** - Requests full contract details and trading rules for the given conid. A follow-up request will provide additional trading rules.
- **`ibkr-pp-cli iserver get-instrument-info`** - Requests full contract details for the given conid.
- **`ibkr-pp-cli iserver get-margin-summary`** - Returns a summary of an account's margin, in total and by account segment.
- **`ibkr-pp-cli iserver get-md-history`** - Request historical data for an instrument in the form of OHLC bars.
- **`ibkr-pp-cli iserver get-md-snapshot`** - Get Market Data for the given conid(s). A pre-flight request must be made prior to ever receiving data. For some fields, it may take more than a few moments to receive information. See response fields for a list of available fields that can be request via fields argument. The endpoint /iserver/accounts must be called prior to /iserver/marketdata/snapshot. For derivative contracts the endpoint /iserver/secdef/search must be called first.
- **`ibkr-pp-cli iserver get-mta-details`** - Retrieve information about your MTA alert. Each login user only has one mobile trading assistant (MTA) alert with it's own unique tool id that cannot be changed. MTA alerts can not be created or deleted, only modified. When modified a new order Id is generated.
- **`ibkr-pp-cli iserver get-open-orders`** - Returns open orders and filled or cancelled orders submitted during the current brokerage session.
- **`ibkr-pp-cli iserver get-order-status`** - Retrieve the status of a single order. Only displays orders from the current brokerage session. If orders executed on a previous day or session, queries will 503 error.
- **`ibkr-pp-cli iserver get-pnl`** - Returns updated profit and loss values for the selected account. Initial request will return an empty array in the upnl object.
- **`ibkr-pp-cli iserver get-scanner-parameters`** - Returns an xml file containing all available parameters to be sent for the Iserver scanner request.
- **`ibkr-pp-cli iserver get-scanner-results`** - Searches for contracts according to the filters specified in /iserver/scanner/params endpoint.
- **`ibkr-pp-cli iserver get-single-allocation-group`** - Retrieves the configuration of a single account group.  This describes the name of the allocation group, the specific accounts contained in the group, and the allocation method in use along with any relevant quantities. This endpoint is only supported for Financial Advisors and IBroker Accounts.
- **`ibkr-pp-cli iserver get-specific-watchlist`** - Retrieve details of a single watchlist stored in the username's settings.
- **`ibkr-pp-cli iserver get-trade-history`** - Retrieve a list of trades, up to a maximum of 7 days prior.
- **`ibkr-pp-cli iserver initialize-session`** - After retrieving the access token and subsequent Live Session Token, customers can initialize their brokerage session with the ssodh/init endpoint.
- **`ibkr-pp-cli iserver modify-allocation-group`** - Modify an existing allocation group.
- **`ibkr-pp-cli iserver modify-open-order`** - Modify an existing, unfilled order.
- **`ibkr-pp-cli iserver post-new-watchlist`** - Create a named watchlist by submitting a set of conids.
- **`ibkr-pp-cli iserver preview-margin-impact`** - Preview the projected effects of an order ticket or bracket of orders, including cost and changes to margin and account equity.
- **`ibkr-pp-cli iserver reset-order-suppression`** - Removes suppression of all order reply messages that were previously suppressed in the current brokerage session.
- **`ibkr-pp-cli iserver set-active-account`** - Switch the active account for how you request data. Only available for financial advisors and multi-account structures.
- **`ibkr-pp-cli iserver set-allocation-preset`** - Set the preset behavior for new allocation groups for specific events.
- **`ibkr-pp-cli iserver set-dynamic-account`** - Set the active dynamic account.
- **`ibkr-pp-cli iserver submit-new-order`** - Submit a new order(s) ticket, bracket, or OCA group.
- **`ibkr-pp-cli iserver suppress-order-replies`** - Suppress the specified order reply messages for the duration of the brokerage session.

### logout

Manage logout

- **`ibkr-pp-cli logout logout`** - Logs the user out of the gateway session. Any further activity requires re-authentication. Discard client-side cookies upon logout.

### oauth

Manage oauth

- **`ibkr-pp-cli oauth req-access-token`** - Request an access token for the IB username that has granted authorization to the consumer.
- **`ibkr-pp-cli oauth req-live-session-token`** - Generate a Live Session Token shared secret and gain access to Web API.
- **`ibkr-pp-cli oauth req-temp-token`** - Request a temporary token as a third party to begin the OAuth 1.0a authorization workflow.

### oauth2

Manage oauth2

- **`ibkr-pp-cli oauth2 generate-token`** - Generate OAuth 2.0 access tokens based on request parameters.

### pa

Manage pa

- **`ibkr-pp-cli pa create`** - Returns consolidated portfolio allocation by Financial Instrument, Asset Class, Sector, Region, or Country for a given set of accounts. Result is aggregated for the included accounts. Current day data is supported only if all included accounts have the same base currency as specified in the currency parameter.
- **`ibkr-pp-cli pa get-performance-all-periods`** - Returns the performance (MTM) for the given accounts, if more than one account is passed, the result is consolidated.
- **`ibkr-pp-cli pa get-single-performance-period`** - Returns the performance (MTM) for the given accounts, if more than one account is passed, the result is consolidated.
- **`ibkr-pp-cli pa get-transactions`** - Transaction history for a given number of conids and accounts. Types of transactions include dividend payments, buy and sell transactions, transfers.

### portfolio

Manage portfolio

- **`ibkr-pp-cli portfolio get-all-accounts`** - List All Accounts
- **`ibkr-pp-cli portfolio get-all-accounts-for-conid`** - Get positions in accounts for a given instrument (no secDef await control)
- **`ibkr-pp-cli portfolio get-all-subaccounts`** - Retrieve attributes of the subaccounts in the account structure.
- **`ibkr-pp-cli portfolio get-many-subaccounts`** - Used in tiered account structures (such as Financial Advisor and IBroker Accounts) to return a list of sub-accounts, paginated up to 20 accounts per page, for which the user can view position and account-related information.  This endpoint must be called prior to calling other /portfolio endpoints for those sub-accounts. If you have less than 100 sub-accounts use /portfolio/subaccounts.  To query a list of accounts the user can trade, see /iserver/accounts.

### portfolio2

Manage portfolio2


### sso

Manage sso

- **`ibkr-pp-cli sso get-session-validation`** - Validates the current session for the SSO user.

### tickle

Manage tickle

- **`ibkr-pp-cli tickle get-session-token`** - If the gateway has not received any requests for several minutes an open session will automatically timeout. The tickle endpoint pings the server to prevent the session from ending. It is expected to call this endpoint approximately every 60 seconds to maintain the connection to the brokerage session.

### trsrv

Manage trsrv

- **`ibkr-pp-cli trsrv get-conids-by-exchange`** - Send out a request to retrieve all contracts made available on a requested exchange. This returns all contracts that are tradable on the exchange, even those that are not using the exchange as their primary listing.
- **`ibkr-pp-cli trsrv get-future-by-symbol`** - Returns a list of non-expired future contracts for given symbol(s)
- **`ibkr-pp-cli trsrv get-instrument-definition`** - Returns a list of security definitions for the given conids.
- **`ibkr-pp-cli trsrv get-stock-by-symbol`** - Returns an object contains all stock contracts for given symbol(s)
- **`ibkr-pp-cli trsrv get-trading-schedule`** - Returns the trading schedule up to a month for the requested contract.

### ws

Manage ws

- **`ibkr-pp-cli ws open-websocket`** - Open websocket.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ibkr-pp-cli gw list

# JSON for scripting and agents
ibkr-pp-cli gw list --json

# Filter to specific fields
ibkr-pp-cli gw list --json --select id,name,status

# Dry run — show the request without sending
ibkr-pp-cli gw list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ibkr-pp-cli gw list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-ibkr -g
```

Then invoke `/pp-ibkr <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add ibkr ibkr-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ibkr-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ibkr": {
      "command": "ibkr-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
ibkr-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/ib-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**icli**](https://github.com/mattsta/icli) — Python (380 stars)
- [**ibkr-cli**](https://github.com/fatwang2/ibkr-cli) — Python (120 stars)
- [**ibkr-cgt**](https://github.com/jonassvedas/ibkr-cgt) — Python (45 stars)
- [**tws-cli**](https://github.com/mark-hennessy/tws-cli) — C# (28 stars)
- [**pry-ib**](https://github.com/davek42/pry-ib) — Ruby (15 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
