---
name: pp-robinhood
description: "Printing Press CLI + MCP for the whole of Robinhood — brokerage accounts (individual + retirement), positions, options + chains, portfolio performance, transfers, dividends, orders, watchlists, plus the official Crypto Trading API as the documented subset. Live reads; writes are dry-run gated."
author: "zaydiscold"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - robinhood-pp-cli
---

# Robinhood — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `robinhood-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install robinhood --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `robinhood-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/robinhood/cmd/robinhood-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Agent Rules

Use read commands and route-map inspection freely. Robinhood Crypto and brokerage/account write routes are PP-gated: they default to `--dry-run`, live execution requires `--live-write`, and the environment must include `ROBINHOOD_PP_ALLOW_WRITES=1`. Never set that gate unless the user explicitly approves the exact live mutation.

The package bundles authenticated browser route maps:

```bash
robinhood-pp-cli brokerage summary --json
robinhood-pp-cli brokerage routes --host api.robinhood.com --json
robinhood-pp-cli brokerage browser-routes --host bonfire.robinhood.com --json
robinhood-pp-cli brokerage plan https://api.robinhood.com/goku/lcm --json
```

`brokerage execute` can use caller-owned `ROBINHOOD_BROKERAGE_TOKEN` or `ROBINHOOD_COOKIE`. Write routes still require `ROBINHOOD_PP_ALLOW_WRITES=1`.

### Typed brokerage commands (stocks, options, accounts, transfers)

Beyond the route map, the `brokerage` group exposes **typed read commands** for the captured brokerage API surface. These are the everyday stock/options/account commands. They authenticate with the same OAuth bearer credential as `brokerage execute` — set `ROBINHOOD_BROKERAGE_TOKEN` (or `ROBINHOOD_COOKIE` + `ROBINHOOD_CSRF_TOKEN`). This is a **separate credential** from the crypto API's `ROBINHOOD_API_KEY` / `ROBINHOOD_PRIVATE_KEY_B64`: the `crypto` group speaks the official signed Crypto API, the `brokerage` group speaks the OAuth-bearer brokerage hosts (`api.robinhood.com`, `bonfire.robinhood.com`, `minerva.robinhood.com`).

```bash
# Accounts — Zayd has multiple individual + a retirement account
robinhood-pp-cli brokerage accounts --json                 # every account
robinhood-pp-cli brokerage portfolios --json               # per-account dollar balances (the "main" balance)
robinhood-pp-cli brokerage account --account-id 1AB23456 --json   # unified balance for one account

# Positions / orders / quotes
robinhood-pp-cli brokerage positions --nonzero --json
robinhood-pp-cli brokerage orders --json
robinhood-pp-cli brokerage quote --symbols AAPL,TSLA --json
robinhood-pp-cli brokerage instrument --symbol AAPL --json

# Options — positions, orders, chain, contracts, greeks
robinhood-pp-cli brokerage options positions --json
robinhood-pp-cli brokerage options orders --json
robinhood-pp-cli brokerage options chain --chain-id <uuid> --json
robinhood-pp-cli brokerage options instruments --chain-id <uuid> --expiration 2026-06-19 --type call --json
robinhood-pp-cli brokerage options marketdata --instruments <uuid> --json

# Performance windows — YTD, week, month, year, 5year, all
robinhood-pp-cli brokerage performance --account-id 1AB23456 --span year --json
robinhood-pp-cli brokerage performance --account-id 1AB23456 --span week --interval day --json

# Transfers / deposits / withdrawals, dividends, account history
robinhood-pp-cli brokerage transfers --json
robinhood-pp-cli brokerage transfers relationships --json
robinhood-pp-cli brokerage dividends --json
robinhood-pp-cli brokerage history --json
```

**Write commands default to dry-run and never auto-execute a trade.** Order placement and cancellation print the exact request and require both `--live-write` and `ROBINHOOD_PP_ALLOW_WRITES=1` for a live mutation — Zayd executes real trades himself. Watchlist add/remove are reversible writes but follow the same gate.

```bash
robinhood-pp-cli brokerage orders place --body-json '{"symbol":"AAPL","side":"buy","type":"market","quantity":"1","time_in_force":"gfd"}' --dry-run
robinhood-pp-cli brokerage orders cancel --order-id <uuid> --dry-run
robinhood-pp-cli brokerage options place --body-json '{"direction":"debit","type":"limit","legs":[]}' --dry-run
robinhood-pp-cli brokerage options cancel --order-id <uuid> --dry-run
robinhood-pp-cli brokerage watchlist add --list-id <uuid> --body-json '{"object_id":"<instrument-uuid>","object_type":"instrument","operation":"create"}' --dry-run
robinhood-pp-cli brokerage watchlist remove --list-id <uuid> --body-json '{"object_id":"<instrument-uuid>","object_type":"instrument","operation":"delete"}' --dry-run
```

# Introduction


Welcome to Robinhood Crypto Trading API documentation for traders and developers! The API lets you view crypto market data, access your account information, and place crypto orders programmatically.

To get started, head to your [crypto account settings](https://robinhood.com/account/crypto) on web classic to create credentials. There are two versions of the Crypto Trading API, giving you the option to place crypto orders with fee tiers (v2) or without fee tiers (v1). All read-only API actions are available on both versions. For more information about API version differences, fee tiers, and answers to common questions, visit our [Help Center](https://robinhood.com/us/en/support/articles/crypto-api). You can also view the [Fee Schedule](https://cdn.robinhood.com/assets/robinhood/legal/rhc-fee-schedule.pdf) for the latest rates.

The Robinhood Crypto Trading API is available to customers in the United States only. Use of the Robinhood Crypto Trading API is subject to the [Robinhood Crypto Customer Agreement](https://cdn.robinhood.com/assets/robinhood/legal/Robinhood%20Crypto%20Customer%20Agreement.pdf) as well as all other terms and disclosures made available on [Robinhood Crypto's about page](https://robinhood.com/us/en/about/crypto).


# Authentication

To get started with the Robinhood Crypto Trading API, you need to create a key, signature, and headers. Authenticated requests must include all three `x-api-key`, `x-signature`, and `x-timestamp` HTTP headers.


## Creating an API key

Head to your [crypto account settings](https://robinhood.com/account/crypto) on web classic to create credentials. After creating credentials, you will receive the API key associated with the credential. You can modify, disable, and delete credentials you created at any time.

Your API key will be used as the `x-api-key` header you will need to pass during authentication when calling our API endpoints. Additionally, you will need the public key generated in the [Creating a key pair](#section/Authentication/Creating-a-key-pair) section to create your API credentials.


## Creating a key pair


#### Below are example scripts on how to generate the public and private key pair. You'll need the public key for creating an API credential and the private key for authenticating requests. Remember to never share your private key with anyone. Robinhood will never ask you to share it with us.

We highly recommend saving your private and public keys in an encrypted format to ensure the highest level of security. Encrypting your keys will protect them from unauthorized access or theft. Avoid saving them in plain text or any easily accessible location. Instead, consider using strong encryption algorithms or tools specifically designed for key storage. Remember to choose a strong passphrase for encryption and store it separately from your keys. By taking these precautions, you can significantly reduce the risk of compromising your keys and safeguard your sensitive information.

 ### Python

Note that you'll need to have `pynacl` installed to run the Python script. You can install them with the following pip command in your terminal.
```console
pip install pynacl
```

```python
import nacl.signing
import base64

# Generate an Ed25519 keypair
private_key = nacl.signing.SigningKey.generate()
public_key = private_key.verify_key

# Convert keys to base64 strings
private_key_base64 = base64.b64encode(private_key.encode()).decode()
public_key_base64 = base64.b64encode(public_key.encode()).decode()

# Print the keys in base64 format
print("Private Key (Base64):")
print(private_key_base64)

print("Public Key (Base64):")
print(public_key_base64)
```


## Headers and Signature

<SecurityDefinitions />


# Making your first API call

Developing your own application to place trades with your Robinhood account is quick and simple. Once you've finished [authentication](https://docs.robinhood.com/crypto/trading/#section/Authentication), start here with the code you'll need to access the API and make supported API calls. These are essentially the building blocks of code for each API call, which you can easily build on based on your preferred strategies.

1. Create a script file

```console
mkdir robinhood-api-trading && cd robinhood-api-trading
touch robinhood_api_trading.py
touch robinhood_api_trading_v2.py
```

2. Install PyNaCl library

```console
pip install pynacl
```

3. To use v1 endpoints, copy the script below into the newly created `robinhood_api_trading.py` file. Make sure to add your API key and secret key into the `API_KEY` and `BASE64_PRIVATE_KEY` variables.

```python
import base64
import datetime
import json
from typing import Any, Dict, Optional
import uuid
import requests
from nacl.signing import SigningKey

API_KEY = "ADD YOUR API KEY HERE"
BASE64_PRIVATE_KEY = "ADD YOUR PRIVATE KEY HERE"

class CryptoAPITrading:
    def __init__(self):
        self.api_key = API_KEY
        private_key_seed = base64.b64decode(BASE64_PRIVATE_KEY)
        self.private_key = SigningKey(private_key_seed)
        self.base_url = "https://trading.robinhood.com"

    @staticmethod
    def _get_current_timestamp() -> int:
        return int(datetime.datetime.now(tz=datetime.timezone.utc).timestamp())

    @staticmethod
    def get_query_params(key: str, *args: Optional[str]) -> str:
        if not args:
            return ""

        params = []
        for arg in args:
            params.append(f"{key}={arg}")

        return "?" + "&".join(params)

    def make_api_request(self, method: str, path: str, body: str = "") -> Any:
        timestamp = self._get_current_timestamp()
        headers = self.get_authorization_header(method, path, body, timestamp)
        url = self.base_url + path

        try:
            response = {}
            if method == "GET":
                response = requests.get(url, headers=headers, timeout=10)
            elif method == "POST":
                response = requests.post(url, headers=headers, json=json.loads(body), timeout=10)
            return response.json()
        except requests.RequestException as e:
            print(f"Error making API request: {e}")
            return None

    def get_authorization_header(
            self, method: str, path: str, body: str, timestamp: int
    ) -> Dict[str, str]:
        message_to_sign = f"{self.api_key}{timestamp}{path}{method}{body}"
        signed = self.private_key.sign(message_to_sign.encode("utf-8"))

        return {
            "x-api-key": self.api_key,
            "x-signature": base64.b64encode(signed.signature).decode("utf-8"),
            "x-timestamp": str(timestamp),
        }

    def get_account(self) -> Any:
        path = "/api/v1/crypto/trading/accounts/"
        return self.make_api_request("GET", path)

    # The symbols argument must be formatted in trading pairs, e.g "BTC-USD", "ETH-USD". If no symbols are provided,
    # all supported symbols will be returned
    def get_trading_pairs(self, *symbols: Optional[str]) -> Any:
        query_params = self.get_query_params("symbol", *symbols)
        path = f"/api/v1/crypto/trading/trading_pairs/{query_params}"
        return self.make_api_request("GET", path)

    # The asset_codes argument must be formatted as the short form name for a crypto, e.g "BTC", "ETH". If no asset
    # codes are provided, all crypto holdings will be returned
    def get_holdings(self, *asset_codes: Optional[str]) -> Any:
        query_params = self.get_query_params("asset_code", *asset_codes)
        path = f"/api/v1/crypto/trading/holdings/{query_params}"
        return self.make_api_request("GET", path)

    # The symbols argument must be formatted in trading pairs, e.g "BTC-USD", "ETH-USD". If no symbols are provided,
    # the best bid and ask for all supported symbols will be returned
    def get_best_bid_ask(self, *symbols: Optional[str]) -> Any:
        query_params = self.get_query_params("symbol", *symbols)
        path = f"/api/v1/crypto/marketdata/best_bid_ask/{query_params}"
        return self.make_api_request("GET", path)

    # The symbol argument must be formatted in a trading pair, e.g "BTC-USD", "ETH-USD"
    # The side argument must be "bid", "ask", or "both".
    # Multiple quantities can be specified in the quantity argument, e.g. "0.1,1,1.999".
    def get_estimated_price(self, symbol: str, side: str, quantity: str) -> Any:
        path = f"/api/v1/crypto/marketdata/estimated_price/?symbol={symbol}&side={side}&quantity={quantity}"
        return self.make_api_request("GET", path)

    def place_order(
            self,
            client_order_id: str,
            side: str,
            order_type: str,
            symbol: str,
            order_config: Dict[str, str],
    ) -> Any:
        body = {
            "client_order_id": client_order_id,
            "side": side,
            "type": order_type,
            "symbol": symbol,
            f"{order_type}_order_config": order_config,
        }
        path = "/api/v1/crypto/trading/orders/"
        return self.make_api_request("POST", path, json.dumps(body))

    def cancel_order(self, order_id: str) -> Any:
        path = f"/api/v1/crypto/trading/orders/{order_id}/cancel/"
        return self.make_api_request("POST", path)

    def get_order(self, order_id: str) -> Any:
        path = f"/api/v1/crypto/trading/orders/{order_id}/"
        return self.make_api_request("GET", path)

    def get_orders(self) -> Any:
        path = "/api/v1/crypto/trading/orders/"
        return self.make_api_request("GET", path)


def main():
    api_trading_client = CryptoAPITrading()
    print(api_trading_client.get_account())

    """
    BUILD YOUR TRADING STRATEGY HERE

    order = api_trading_client.place_order(
          str(uuid.uuid4()),
          "buy",
          "market",
          "BTC-USD",
          {"asset_quantity": "0.0001"}
    )
    """


if __name__ == "__main__":
    main()
```

4. To use v2 endpoints, copy the script below into the newly created `robinhood_api_trading_v2.py` file. Make sure to add your API key and secret key into the `API_KEY` and `BASE64_PRIVATE_KEY` variables.

```python
import base64
import datetime
import json
from typing import Any, Dict, Optional
import uuid
import requests
from nacl.signing import SigningKey

API_KEY = "ADD YOUR API KEY HERE"
BASE64_PRIVATE_KEY = "ADD YOUR PRIVATE KEY HERE"


class CryptoAPITradingV2:
    def __init__(self):
        self.api_key = API_KEY
        private_key_seed = base64.b64decode(BASE64_PRIVATE_KEY)
        self.private_key = SigningKey(private_key_seed)
        self.base_url = "https://trading.robinhood.com"

    @staticmethod
    def _get_current_timestamp() -> int:
        return int(datetime.datetime.now(tz=datetime.timezone.utc).timestamp())

    @staticmethod
    def get_query_params(params_dict: Dict[str, Any]) -> str:
        """
        Build query parameter string from a dictionary.
        - Single values: {"key1": "value1", "key2": "value2"}
        - Multiple values for same key: {"symbol": ["BTC-USD", "ETH-USD"]}
        """
        if not params_dict:
            return ""
        
        params = []
        for key, value in params_dict.items():
            if value is None:
                continue
            if isinstance(value, list):
                # Handle multiple values for the same key
                for v in value:
                    params.append(f"{key}={v}")
            else:
                params.append(f"{key}={value}")
        
        return "?" + "&".join(params) if params else ""

    def make_api_request(self, method: str, path: str, body: str = "") -> Any:
        timestamp = self._get_current_timestamp()
        headers = self.get_authorization_header(method, path, body, timestamp)
        url = self.base_url + path

        try:
            response = {}
            if method == "GET":
                response = requests.get(url, headers=headers, timeout=10)
            elif method == "POST":
                response = requests.post(url, headers=headers, json=json.loads(body) if body else None, timeout=10)

            # Check for non-success status codes
            if response.status_code >= 400:
                print(f"HTTP Error {response.status_code}: {response.reason}")
                try:
                    return response.json()
                except json.JSONDecodeError:
                    return {"error": response.text or response.reason, "status_code": response.status_code}

            return response.json()
        except requests.RequestException as e:
            print(f"Error making API request: {e}")
            return None

    def get_authorization_header(
            self, method: str, path: str, body: str, timestamp: int
    ) -> Dict[str, str]:
        message_to_sign = f"{self.api_key}{timestamp}{path}{method}{body}"
        signed = self.private_key.sign(message_to_sign.encode("utf-8"))

        return {
            "x-api-key": self.api_key,
            "x-signature": base64.b64encode(signed.signature).decode("utf-8"),
            "x-timestamp": str(timestamp),
        }

    def get_accounts(self) -> Any:
        path = "/api/v2/crypto/trading/accounts/"
        return self.make_api_request("GET", path)

    # The symbols argument must be formatted in trading pairs, e.g "BTC-USD", "ETH-USD". If no symbols are provided,
    # all supported symbols will be returned
    def get_trading_pairs(self, *symbols: Optional[str]) -> list:
        params = {"symbol": list(symbols)} if symbols else {}
        query_params = self.get_query_params(params)
        path = f"/api/v2/crypto/trading/trading_pairs/{query_params}"
        
        all_results = []
        response = self.make_api_request("GET", path)
        
        while response:
            results = response.get("results", [])
            all_results.extend(results)
            
            next_url = response.get("next")
            if not next_url:
                break
            
            next_path = next_url.replace(self.base_url, "")
            response = self.make_api_request("GET", next_path)
        
        return all_results

    # The asset_codes argument must be formatted as the short form name for a crypto, e.g "BTC", "ETH". If no asset
    # codes are provided, all crypto holdings will be returned
    def get_holdings(self, account_number: str, *asset_codes: Optional[str]) -> Any:
        params = {"account_number": account_number}
        if asset_codes:
            params["asset_code"] = list(asset_codes)
        query_params = self.get_query_params(params)
        path = f"/api/v2/crypto/trading/holdings/{query_params}"
        return self.make_api_request("GET", path)

    # The symbols argument must be formatted in trading pairs, e.g "BTC-USD", "ETH-USD".
    def get_best_bid_ask(self, *symbols: str) -> Any:
        params = {"symbol": list(symbols)} if symbols else {}
        query_params = self.get_query_params(params)
        path = f"/api/v2/crypto/marketdata/best_bid_ask/{query_params}"
        return self.make_api_request("GET", path)

    # The symbol argument must be formatted in a trading pair, e.g "BTC-USD", "ETH-USD"
    # The side argument must be "bid", "ask", or "both".
    # Multiple quantities can be specified in the quantity argument, e.g. "0.1,1,1.999".
    def get_estimated_price(self, symbol: str, side: str, quantity: str) -> Any:
        params = {"symbol": symbol, "side": side, "quantity": quantity}
        query_params = self.get_query_params(params)
        path = f"/api/v2/crypto/trading/estimated_price/{query_params}"
        return self.make_api_request("GET", path)

    def place_order(
            self,
            account_number: str,
            client_order_id: str,
            side: str,
            order_type: str,
            symbol: str,
            order_config: Dict[str, str],
    ) -> Any:
        body = {
            "client_order_id": client_order_id,
            "side": side,
            "type": order_type,
            "symbol": symbol,
            f"{order_type}_order_config": order_config,
        }
        params = {"account_number": account_number}
        query_params = self.get_query_params(params)
        path = f"/api/v2/crypto/trading/orders/{query_params}"
        return self.make_api_request("POST", path, json.dumps(body))

    def cancel_order(self, order_id: str) -> Any:
        path = f"/api/v2/crypto/trading/orders/{order_id}/cancel/"
        return self.make_api_request("POST", path)

    def get_order(self, account_number: str, order_id: str) -> Any:
        params = {"account_number": account_number}
        query_params = self.get_query_params(params)
        path = f"/api/v2/crypto/trading/orders/{order_id}/{query_params}"
        return self.make_api_request("GET", path)

    def get_orders(self, account_number: str) -> Any:
        params = {
            "account_number": account_number,
            "created_at_start": "2023-01-01T20:57:50Z"
        }
        query_params = self.get_query_params(params)
        path = f"/api/v2/crypto/trading/orders/{query_params}"
        return self.make_api_request("GET", path)


def main():
    api_trading_client = CryptoAPITradingV2()
    print(api_trading_client.get_trading_pairs())
    # print(api_trading_client.get_trading_pairs("BTC-USD", "ETH-USD"))
    # print(api_trading_client.get_best_bid_ask("BTC-USD")) #"BTC-USD", "ETH-USD"
    # print(api_trading_client.get_estimated_price(symbol="BTC-USD", side="both", quantity="0.01"))

    # accounts = api_trading_client.get_accounts()
    # print(accounts)
    # account_number = accounts["results"][0]["account_number"]
    # print(f"account_number: {account_number}")
    # print(api_trading_client.get_orders(account_number))
    # print(api_trading_client.get_holdings(account_number))
    # print(api_trading_client.get_holdings(account_number, "BTC", "ETH"))

    """
    BUILD YOUR TRADING STRATEGY HERE

    order = api_trading_client.place_order(
          account_number,
          str(uuid.uuid4()),
          "buy",
          "market",
          "BTC-USD",
          {"asset_quantity": "0.000001"}
    )
    """


if __name__ == "__main__":
    main()
```

5. Run your script from the command line

```console
# to run the Python script using the v1 API endpoints:
python robinhood_api_trading.py
# to run the Python script using the v2 API endpoints:
python robinhood_api_trading_v2.py
```


# Pagination


 Endpoints that return a list of results allow you to paginate to have more control over how many and which results to display.


### Pagination Parameters

| Parameter | Description |
|-|-|
| cursor | The cursor is a unique identifier for each page in a list of results. The cursor is used to paginate through the pages of results. |
| limit | The limit is the number of items to return in a single page. Some of our endpoints support this query parameter. You can view support by checking the query parameters in the documentation of each endpoint.  |


### Pagination Response


| Field | Description |
|-|-|
| next | The API request endpoint that includes the next cursor query parameter to use for pagination. |
| previous | The API request endpoint that includes the previous cursor query parameter to use for pagination. |
| results | The list of response items for the current cursor.


# Rate Limiting


### Rate Limits
* Requests per minute per user account: 100
* Requests per minute per user account in bursts: 300


Rate limiting is applied using a token bucket implementation. The burst size or `capacity` is the number of tokens you can use to call an endpoint. This capacity is initialized at the maximum capacity and will be refilled using a `refill amount` at a timed interval called `refill interval` until the max capacity is once again reached.


### Rate Limiting Terms


| Term | Description |
|-|-|
| Max capacity | The maximum amount of tokens allowed. Will no longer continue refilling if this amount is reached. |
| Remaining amount | The number of tokens remaining that can be consumed to call an endpoint. |
| Refill amount | The number of tokens that are refilled at each refill interval. |
| Refill interval | The timed interval at which the tokens are refilled.

 The actual values of the configuration will fluctuate depending on the availability of our service and our current expected volume at the time of service. Rate limits are applied per endpoint and may differ among each endpoint depending on their expected use case.


 #### Example rate limiting configuration:


| Term | Value |
|-|-|
| Max capacity | 5 |
| Remaining amount | 2 |
| Refill amount | 1 |
| Refill interval | 1 second |


| Action | Time (in seconds) | Remaining amount | Description |
|-|-|-|-|
| Initialize | 0 | 5 | Initial state of the token bucket. |
| Endpoint call 1 | 0.5 | 4 | One token consumed for calling endpoint 1. |
| Endpoint call 2 | 0.7 | 3 | One token consumed for calling endpoint 2. |
| Refill | 1 | 4 | One token refilled at refill interval. |
| Refill | 2 | 5 | One token refilled at refill interval. |
| No refill | 3 | 5 | No refill since max capacity has been reached and no endpoints were called. |
| Endpoint call 3 | 3.5 | 4 | One token consumed for calling endpoint 3.


# Error Responses


### Error response format

#### Type
 The `type` field in the error response will be mapped to the following:
 | Error type | Status codes |
|-|-|
| validation_error | 400 |
| client_error | 4XX, excluding 400 |
| server_error | 5XX |


#### Errors
The `errors` field will contain a list of error details, each item will contain a nested `attr` and `detail` field.
| Field | Description |
|-|-|
| attr | Error types of `validation_error` will specify the field name or `non_field_errors` if the error cannot be attributed to a field. Will be `null` for error types of either `client_error` and `server_error`. |
| detail | Will contain a human readable string describing the error. |



### Example Error Response
Here's a sample error response where the `client_order_id` field in the payload was a value that was not expected when calling the ***Add Crypto Order*** endpoint. The `detail` field for each error in the `errors` list will help understand why the `validation_error` was thrown. The `attr` field will indicate which field name in the request body or query parameter the error was thrown for if applicable.
```javascript
{
  "type": "validation_error"
  "errors": [
    {
      "detail": "Must be a valid UUID.",
      "attr": "client_order_id"
    }
 ]
}
```


### Common Error Status Codes
 | Status Code | Error |
|-|-|
| 400 | Bad request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not found |
| 405 | Method not allowed |
| 406 | Not acceptable |
| 415 | Unsupported media type |
| 429 | Too many requests |
| 500 | Internal server error |
| 503 | Service unavailable |

## Command Reference

**crypto** — Manage crypto

- `robinhood-pp-cli crypto marketdata-best-bid-ask` — Fetch a single bid and ask price per symbol, representing the best available price across our partner market makers.
- `robinhood-pp-cli crypto marketdata-best-bid-ask-marketdata` — Fetch a single bid and ask price per symbol, representing the best available price across our partner exchanges.
- `robinhood-pp-cli crypto marketdata-estimated-price` — This endpoint returns the estimated total cost or credit for a particular symbol, book side, and asset quantity.
- `robinhood-pp-cli crypto marketdata-estimated-price-trading` — This endpoint returns the estimated total cost or credit for a particular symbol, book side, asset quantity, and fee.
- `robinhood-pp-cli crypto post-trading-cancel-order` — Cancels an open crypto trading order.
- `robinhood-pp-cli crypto post-trading-order` — Places a new crypto trading order with an order type.
- `robinhood-pp-cli crypto trading-account-details` — Fetches the Robinhood Crypto account details for the current user.
- `robinhood-pp-cli crypto trading-accounts` — Retrieve a paginated list of crypto trading accounts for the authenticated user.
- `robinhood-pp-cli crypto trading-cancel-order` — Cancels an open crypto trading order.
- `robinhood-pp-cli crypto trading-holdings` — Fetch a list of holdings for the current user.
- `robinhood-pp-cli crypto trading-holdings-trading` — Retrieve a paginated list of crypto holdings for a specific account.
- `robinhood-pp-cli crypto trading-orders` — Fetch a list of orders for the current user.
- `robinhood-pp-cli crypto trading-orders-get` — Fetch a list of orders for the current user in a specific account.
- `robinhood-pp-cli crypto trading-orders-post` — Place a new crypto trading order with an order type.
- `robinhood-pp-cli crypto trading-trading-pairs` — Fetch a list of trading pairs.
- `robinhood-pp-cli crypto trading-trading-pairs-trading` — Fetch a paginated list of available trading pairs for crypto trading.

**brokerage** — Inspect Robinhood brokerage/account route maps and the typed brokerage surface (OAuth bearer auth)

- `robinhood-pp-cli brokerage summary` — Summarize bundled brokerage/account route maps.
- `robinhood-pp-cli brokerage plan` — Build a dry-run request plan for a mapped route.
- `robinhood-pp-cli brokerage execute` — Execute a mapped brokerage/account request with PP write gates.
- `robinhood-pp-cli brokerage accounts` — List all brokerage accounts (individual + retirement).
- `robinhood-pp-cli brokerage ceres-accounts` — List accounts via the ceres gateway.
- `robinhood-pp-cli brokerage account` — Show the unified balance view for one account.
- `robinhood-pp-cli brokerage account-switcher` — List accounts as shown in the app account switcher.
- `robinhood-pp-cli brokerage positions` — List equity positions.
- `robinhood-pp-cli brokerage portfolios` — List portfolios (equity, market value, withdrawable per account).
- `robinhood-pp-cli brokerage instrument` — Look up an instrument (tradable security) by symbol.
- `robinhood-pp-cli brokerage quote` — Fetch real-time quotes for one or more symbols.
- `robinhood-pp-cli brokerage orders` — List equity orders.
- `robinhood-pp-cli brokerage orders place` — Place an equity order (dry-run by default).
- `robinhood-pp-cli brokerage orders cancel` — Cancel an equity order (dry-run by default).
- `robinhood-pp-cli brokerage options positions` — List aggregate options positions.
- `robinhood-pp-cli brokerage options orders` — List options orders.
- `robinhood-pp-cli brokerage options chain` — List option chains, or fetch one chain by id.
- `robinhood-pp-cli brokerage options instruments` — List option instruments (contracts) for a chain.
- `robinhood-pp-cli brokerage options marketdata` — Fetch options market data (greeks, IV, bid/ask).
- `robinhood-pp-cli brokerage options place` — Place an options order (dry-run by default).
- `robinhood-pp-cli brokerage options cancel` — Cancel an options order (dry-run by default).
- `robinhood-pp-cli brokerage performance` — Portfolio value over a window (YTD, week, month, year, 5year, all).
- `robinhood-pp-cli brokerage transfers` — List ACH transfers (deposits + withdrawals).
- `robinhood-pp-cli brokerage transfers relationships` — List linked bank (ACH) relationships.
- `robinhood-pp-cli brokerage transfers unified` — List unified transfers across rails.
- `robinhood-pp-cli brokerage dividends` — List dividends (paid + pending).
- `robinhood-pp-cli brokerage history` — List account transaction history.
- `robinhood-pp-cli brokerage watchlist` — List the default watchlist.
- `robinhood-pp-cli brokerage watchlist items` — List watchlist items.
- `robinhood-pp-cli brokerage watchlist add` — Add an item to a watchlist (safe write, dry-run by default).
- `robinhood-pp-cli brokerage watchlist remove` — Remove an item from a watchlist (safe write, dry-run by default).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
robinhood-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `robinhood-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export ROBINHOOD_API_KEY="<your-key>"
export ROBINHOOD_PRIVATE_KEY_B64="<your-private-key-base64>"
```

Or persist it in `~/.config/robinhood-pp-cli/config.toml`.

Run `robinhood-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  robinhood-pp-cli crypto marketdata-best-bid-ask --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
robinhood-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
robinhood-pp-cli feedback --stdin < notes.txt
robinhood-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.robinhood-pp-cli/feedback.jsonl`. They are never POSTed unless `ROBINHOOD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ROBINHOOD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
robinhood-pp-cli profile save briefing --json
robinhood-pp-cli --profile briefing crypto marketdata-best-bid-ask
robinhood-pp-cli profile list --json
robinhood-pp-cli profile show briefing
robinhood-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `robinhood-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add robinhood-pp-mcp -- robinhood-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which robinhood-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   robinhood-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `robinhood-pp-cli <command> --help`.
