# Robinhood CLI

# Introduction

This is a CLI + MCP for **the whole of Robinhood**, not just crypto. It covers the brokerage surface — multiple accounts (individual + retirement), positions, equity and options orders, option chains, portfolio performance over YTD/1-week/1-month/1-year/5-year/all-time windows, ACH transfers (deposits and withdrawals), dividends, watchlists, and recurring investments (resume/pause/edit) — across `api.robinhood.com` and the `bonfire` gateway, captured live and risk-labeled. True full-account coverage: identify, navigate, and modify across every account. Brokerage auth uses an OAuth bearer (`ROBINHOOD_BROKERAGE_TOKEN`); reads run live while all write commands are **read-only by default**, staying in dry-run "test mode" behind the `ROBINHOOD_PP_ALLOW_WRITES=1` gate plus `--live-write`.

The **official Robinhood Crypto Trading API** below is the easy documented subset (it has its own public spec and key/signature auth), included alongside the brokerage map.

Welcome to Robinhood Crypto Trading API documentation for traders and developers! The Crypto API lets you view crypto market data, access your account information, and place crypto orders programmatically.

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

Created by [@zaydiscold](https://github.com/zaydiscold) (zaydiscold).

## Install

The recommended path installs both the `robinhood-pp-cli` binary and the `pp-robinhood` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install robinhood
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install robinhood --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install robinhood --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install robinhood --agent claude-code
npx -y @mvanhorn/printing-press-library install robinhood --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/robinhood/cmd/robinhood-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/robinhood-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install robinhood --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-robinhood --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-robinhood --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install robinhood --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/robinhood-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ROBINHOOD_API_KEY` and `ROBINHOOD_PRIVATE_KEY_B64` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "robinhood": {
      "command": "robinhood-pp-mcp",
      "env": {
        "ROBINHOOD_API_KEY": "<your-key>",
        "ROBINHOOD_PRIVATE_KEY_B64": "<your-private-key-base64>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Create Robinhood Crypto API credentials from Robinhood's crypto account settings. The CLI needs both the API key and the base64 Ed25519 private key used to sign requests.

```bash
export ROBINHOOD_API_KEY="<paste-your-key>"
export ROBINHOOD_PRIVATE_KEY_B64="<paste-your-private-key-base64>"
```

You can also persist this in your config file at `~/.config/robinhood-pp-cli/config.toml`.

### 3. Verify Setup

```bash
robinhood-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
robinhood-pp-cli crypto marketdata-best-bid-ask
```

### Brokerage/account route map

The package also bundles the 2026-05-27 authenticated Robinhood browser map from ticker, options, crypto, account, settings, transfer, document, and market pages:

```bash
robinhood-pp-cli brokerage summary --json
robinhood-pp-cli brokerage browser-routes --host bonfire.robinhood.com --json
robinhood-pp-cli brokerage execute https://api.robinhood.com/goku/lcm --json
```

Read inspection has no write gate. Any route that can write defaults to `--dry-run`; live execution requires `--live-write` plus `ROBINHOOD_PP_ALLOW_WRITES=1`. Brokerage/account execution uses caller-owned `ROBINHOOD_BROKERAGE_TOKEN` or `ROBINHOOD_COOKIE`.

### Typed brokerage commands

Alongside the route map, the `brokerage` group ships typed read commands for the everyday brokerage surface — accounts, positions, orders, options, performance, transfers, dividends, and history. They authenticate with the OAuth bearer credential (`ROBINHOOD_BROKERAGE_TOKEN`, or `ROBINHOOD_COOKIE` + `ROBINHOOD_CSRF_TOKEN`) shared with `brokerage execute`. This is a **separate credential** from the crypto API's `ROBINHOOD_API_KEY` / `ROBINHOOD_PRIVATE_KEY_B64`.

```bash
robinhood-pp-cli brokerage accounts --json                       # all accounts (individual + retirement)
robinhood-pp-cli brokerage portfolios --json                     # per-account dollar balances
robinhood-pp-cli brokerage positions --nonzero --json            # open equity positions
robinhood-pp-cli brokerage options chain --chain-id <uuid> --json
robinhood-pp-cli brokerage performance --account-id 1AB23456 --span year --json
robinhood-pp-cli brokerage transfers --json                      # ACH deposits + withdrawals
robinhood-pp-cli brokerage dividends --json
```

Order placement/cancellation (`brokerage orders place`/`cancel`, `brokerage options place`/`cancel`) and watchlist add/remove default to `--dry-run` and never auto-execute; a live mutation requires `--live-write` plus `ROBINHOOD_PP_ALLOW_WRITES=1`.

## Usage

Run `robinhood-pp-cli --help` for the full command reference and flag list.

## Commands

### crypto

Access Robinhood Crypto trading, holdings, orders, pairs, and market data

- **`robinhood-pp-cli crypto marketdata-best-bid-ask`** - Fetch a single bid and ask price per symbol, representing the best available price across our partner market makers. This price does not take into account the order size, and may not be the final execution price.

The bid and ask prices include a spread. The buy spread is the percent difference between the ask and the mid price. The sell spread is the percent difference between the bid and the mid price.

Note that in the v2 endpoint, **Place crypto orders with fee tiers**, partner exchanges provide prices instead of market makers, with orders routed accordingly. For more information on routing, visit our [<u>Help Center</u>](https://robinhood.com/us/en/support/articles/crypto-order-routing/).
- **`robinhood-pp-cli crypto marketdata-best-bid-ask-marketdata`** - Fetch a single bid and ask price per symbol, representing the best available price across our partner exchanges. This price does not take into account the order size or fee, and may not be the final execution price.
- **`robinhood-pp-cli crypto marketdata-estimated-price`** - This endpoint returns the estimated total cost or credit for a particular symbol, book side, and asset quantity. You can include a list of quantities in a single request to retrieve the price for various hypothetical order sizes. 

The estimated price represents the expected execution price if you were to subsequently place an order. To estimate the cost for a Buy order, request an Ask quote. If you are preparing to place a Sell order, request a Bid quote. The execution price may vary due to market volatility and once executed the transaction may not be undone.

The bid and ask prices are the best prices our market makers provide for an order, inclusive of a spread. The buy spread is the percent difference between the ask and the mid price. The sell spread is the percent difference between the bid and the mid price.
- **`robinhood-pp-cli crypto marketdata-estimated-price-trading`** - This endpoint returns the estimated total cost or credit for a particular symbol, book side, asset quantity, and fee. You can include a list of quantities in a single request to retrieve the price for various hypothetical order sizes.

The estimated price represents the expected execution price if you were to subsequently place an order. The execution price may vary due to market volatility and once executed the transaction may not be undone.

The bid and ask prices are the best prices our partner exchanges provide for an order, excluding the fee.
- **`robinhood-pp-cli crypto post-trading-cancel-order`** - Cancels an open crypto trading order.
- **`robinhood-pp-cli crypto post-trading-order`** - Places a new crypto trading order with an order type. 

 **Note**: Depending on the type used in the request body, you must include the respective order configuration in the request body.

For order configurations that support both `asset_quantity` or `quote_amount`, only one can be present in the request body.
- **`robinhood-pp-cli crypto trading-account-details`** - Fetches the Robinhood Crypto account details for the current user.
- **`robinhood-pp-cli crypto trading-accounts`** - Retrieve a paginated list of crypto trading accounts for the authenticated user. Returns account details including buying power, status, and fee tier information.
- **`robinhood-pp-cli crypto trading-cancel-order`** - Cancels an open crypto trading order.
- **`robinhood-pp-cli crypto trading-holdings`** - Fetch a list of holdings for the current user.
- **`robinhood-pp-cli crypto trading-holdings-trading`** - Retrieve a paginated list of crypto holdings for a specific account. Returns the total quantity and available quantity for each asset held in the account. The `account_number` query parameter is required.
- **`robinhood-pp-cli crypto trading-orders`** - Fetch a list of orders for the current user.
- **`robinhood-pp-cli crypto trading-orders-get`** - Fetch a list of orders for the current user in a specific account.
- **`robinhood-pp-cli crypto trading-orders-post`** - Place a new crypto trading order with an order type. This endpoint only supports placing orders on USD symbols that have `is_api_tradable=true` as defined in our Get Crypto Trading Pairs endpoint.

**Note:** Depending on the type used in the request body, you must include the respective order configuration in the request body.

For order configurations that support both `asset_quantity` or `quote_amount`, only one can be present in the request body.
- **`robinhood-pp-cli crypto trading-trading-pairs`** - Fetch a list of trading pairs.
- **`robinhood-pp-cli crypto trading-trading-pairs-trading`** - Fetch a paginated list of available trading pairs for crypto trading. Returns trading pair details including price increments, order size limits, and tradability status. Trading pairs can be filtered by symbol.

### brokerage

Inspect Robinhood brokerage/account route maps and the typed brokerage surface. Read commands authenticate with `ROBINHOOD_BROKERAGE_TOKEN` (OAuth bearer); write commands default to `--dry-run`.

- **`robinhood-pp-cli brokerage summary`** - Summarize bundled brokerage/account route maps.
- **`robinhood-pp-cli brokerage plan`** - Build a dry-run request plan for a mapped route.
- **`robinhood-pp-cli brokerage execute`** - Execute a mapped brokerage/account request with PP write gates.
- **`robinhood-pp-cli brokerage accounts`** - List all brokerage accounts (individual + retirement). Maps `GET /accounts/`.
- **`robinhood-pp-cli brokerage ceres-accounts`** - List accounts via the ceres gateway. Maps `GET /ceres/v1/accounts`.
- **`robinhood-pp-cli brokerage account`** - Unified balance view for one account. Maps `GET /accounts/{id}/unified/` (bonfire).
- **`robinhood-pp-cli brokerage account-switcher`** - Accounts in the app account-switcher shape. Maps `GET /home/account_switcher/v2` (bonfire).
- **`robinhood-pp-cli brokerage positions`** - List equity positions. Maps `GET /positions/`.
- **`robinhood-pp-cli brokerage portfolios`** - Per-account equity/market value/withdrawable. Maps `GET /portfolios/`.
- **`robinhood-pp-cli brokerage instrument`** - Look up an instrument by symbol. Maps `GET /instruments/`.
- **`robinhood-pp-cli brokerage quote`** - Real-time quotes for symbols. Maps `GET /marketdata/quotes/`.
- **`robinhood-pp-cli brokerage orders`** - List equity orders. Maps `GET /orders/`.
- **`robinhood-pp-cli brokerage orders place`** - Place an equity order (dry-run by default). Maps `POST /orders/`.
- **`robinhood-pp-cli brokerage orders cancel`** - Cancel an equity order (dry-run by default). Maps `POST /orders/{id}/cancel/`.
- **`robinhood-pp-cli brokerage options positions`** - Aggregate options positions. Maps `GET /options/aggregate_positions/`.
- **`robinhood-pp-cli brokerage options orders`** - Options orders. Maps `GET /options/orders/`.
- **`robinhood-pp-cli brokerage options chain`** - Option chains, or one by id. Maps `GET /options/chains/` and `/options/chains/{id}/`.
- **`robinhood-pp-cli brokerage options instruments`** - Option contracts for a chain. Maps `GET /options/instruments/`.
- **`robinhood-pp-cli brokerage options marketdata`** - Options greeks/IV/bid-ask. Maps `GET /marketdata/options/`.
- **`robinhood-pp-cli brokerage options place`** - Place an options order (dry-run by default). Maps `POST /options/orders/`.
- **`robinhood-pp-cli brokerage options cancel`** - Cancel an options order (dry-run by default). Maps `POST /options/orders/{id}/cancel/`.
- **`robinhood-pp-cli brokerage performance`** - Portfolio value over a window (YTD, week, month, year, 5year, all). Maps `GET /portfolios/historicals/{id}/`.
- **`robinhood-pp-cli brokerage transfers`** - ACH transfers (deposits + withdrawals). Maps `GET /ach/transfers/`.
- **`robinhood-pp-cli brokerage transfers relationships`** - Linked bank relationships. Maps `GET /ach/relationships/`.
- **`robinhood-pp-cli brokerage transfers unified`** - Unified transfers across rails. Maps `GET /paymenthub/unified_transfers/` (bonfire).
- **`robinhood-pp-cli brokerage dividends`** - Dividends (paid + pending). Maps `GET /dividends/`.
- **`robinhood-pp-cli brokerage history`** - Account transaction history. Maps `GET /history/transactions/` (minerva).
- **`robinhood-pp-cli brokerage watchlist`** - Default watchlist; `items`/`add`/`remove` subcommands. Maps `GET /discovery/lists/default/`.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
robinhood-pp-cli crypto marketdata-best-bid-ask

# JSON for scripting and agents
robinhood-pp-cli crypto marketdata-best-bid-ask --json

# Filter to specific fields
robinhood-pp-cli crypto marketdata-best-bid-ask --json --select id,name,status

# Dry run — show the request without sending
robinhood-pp-cli crypto marketdata-best-bid-ask --dry-run

# Agent mode — JSON + compact + no prompts in one flag
robinhood-pp-cli crypto marketdata-best-bid-ask --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
robinhood-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/robinhood-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ROBINHOOD_API_KEY` | per_call | Yes | Robinhood Crypto API key. |
| `ROBINHOOD_PRIVATE_KEY_B64` | per_call | Yes | Base64 Ed25519 private signing key from Robinhood Crypto credentials. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `robinhood-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ROBINHOOD_API_KEY`
- Verify the signing key environment variable is set: `test -n "$ROBINHOOD_PRIVATE_KEY_B64" && echo set`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
