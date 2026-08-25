> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Account API

> Authenticate with the Parallel Account API using device-based OAuth 2.0

<div className="sr-only" aria-hidden="false">
  For AI agents: a documentation index is available at [https://docs.parallel.ai/llms.txt](https://docs.parallel.ai/llms.txt). The full text of all docs is at [https://docs.parallel.ai/llms-full.txt](https://docs.parallel.ai/llms-full.txt). You may also fetch any page as Markdown by appending `.md` to its URL or sending `Accept: text/markdown`.
</div>

The **Account API** lets your application call Parallel programmatically on behalf of a user without prompting them to paste an API key. Authentication uses the [OAuth 2.0 Device Authorization Grant](https://datatracker.ietf.org/doc/html/rfc8628) (RFC 8628), which is designed for clients without a browser or with limited input — CLIs, IDE plugins, IoT devices, and similar.

The flow produces an **access token** that you pass as a `Bearer` token on every Account API request. See the full endpoint reference in the **Account API** tab in the navigation.

<Tip>
  The [**Parallel CLI**](/integrations/cli) handles the entire device-OAuth flow for you — register, device code, polling, refresh, and revoke — and exposes every Account API endpoint as a regular CLI command. If you don't need to embed the flow in your own application, the CLI is the fastest way to use the Account API. The rest of this page describes what the CLI does under the hood, for clients that need to implement it themselves.
</Tip>

## Base URL

All endpoints below are rooted at:

```
https://platform.parallel.ai
```

## Overview

The full lifecycle is:

1. **Register your client** once, to obtain a `client_id`.
2. **Request a device code** to start an authorization session.
3. **Prompt the user to verify** in their browser using the verification URL and user code returned in step 2.
4. **Poll the token endpoint** until the user approves and you receive an `access_token` and `refresh_token`.
5. **Call the Account API** with `Authorization: Bearer <access_token>`.
6. **Refresh** the access token when it expires, or **revoke** the refresh token when the user signs out.

<Note>
  The `access_token` returned by step 4 is what authorizes Account API calls. Without a valid bearer token, requests to the Account API will be rejected.
</Note>

## 1. Register your client

Register your application once to receive a `client_id`. Send a description of your client and the platform it runs on:

```http theme={"system"}
POST https://platform.parallel.ai/getServiceKeys/register
Content-Type: application/json
Accept: application/json

{
  "client_name": "your-app-name",
  "platform": {
    "machine": "arm64",
    "os_name": "posix",
    "processor": "arm",
    "release": "24.6.0",
    "system": "Darwin",
    "version": "Darwin Kernel Version 24.6.0"
  }
}
```

The response contains a `client_id` you reuse for all subsequent device-code requests from the same installation.

## 2. Request a device code

Start an authorization session by requesting a device code. The body is `application/x-www-form-urlencoded`:

```http theme={"system"}
POST https://platform.parallel.ai/getServiceKeys/device/code
Content-Type: application/x-www-form-urlencoded
Accept: application/json

client_id={clientId}&scope={scope}
```

The response includes:

* `device_code` — opaque code your client uses when polling the token endpoint.
* `user_code` — short code the user types in the browser.
* `verification_uri` (and typically `verification_uri_complete`) — the URL to send the user to.
* `expires_in` — lifetime of the device code, in seconds.
* `interval` — minimum number of seconds to wait between polls.

Display the `user_code` and `verification_uri` to the user, or open `verification_uri_complete` in their browser directly.

## 3. Poll for the access token

While the user completes verification in the browser, poll the token endpoint at the interval the server specified:

```http theme={"system"}
POST https://platform.parallel.ai/getServiceKeys/token
Content-Type: application/x-www-form-urlencoded
Accept: application/json

grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id={clientId}&device_code={deviceCode}
```

Polling returns one of:

* **`authorization_pending`** — user has not yet approved. Wait `interval` seconds and try again.
* **`slow_down`** — you are polling too fast. Increase your interval.
* **`access_denied`** — the user rejected the request. Stop polling.
* **`expired_token`** — the device code expired. Restart from step 2.
* **Success** — a JSON body containing `access_token`, `refresh_token`, `token_type` (`Bearer`), and `expires_in`.

Once you have the `access_token`, you can call the Account API.

## 4. Call the Account API

Every Account API request must include an `Authorization` header with the access token from step 3:

```http theme={"system"}
GET https://api.parallel.ai/account/service/...
Authorization: Bearer {accessToken}
Accept: application/json
```

Requests without a valid bearer token are rejected. The endpoint catalogue is documented in the **Account API** tab in the navigation.

## 5. Refresh the access token

Access tokens are short-lived. When one expires, exchange your refresh token for a new pair without prompting the user again:

```http theme={"system"}
POST https://platform.parallel.ai/getServiceKeys/token
Content-Type: application/x-www-form-urlencoded
Accept: application/json

grant_type=refresh_token&refresh_token={refreshToken}
```

The response contains a new `access_token` and (typically) a new `refresh_token`. Persist both and use the new pair going forward.

## 6. Revoke a refresh token

When the user signs out, or when you want to invalidate a session, revoke the refresh token:

```http theme={"system"}
POST https://platform.parallel.ai/getServiceKeys/token/revoke
Content-Type: application/x-www-form-urlencoded
Accept: application/json

refresh_token={refreshToken}
```

After revocation, the refresh token can no longer be used to obtain new access tokens. Existing access tokens remain valid until they expire on their own.

## End-to-end example

```bash theme={"system"}
# 1. Register (once per installation)
curl -X POST https://platform.parallel.ai/getServiceKeys/register \
  -H "Content-Type: application/json" \
  -d '{
    "client_name": "my-app",
    "platform": {
      "machine": "arm64",
      "os_name": "posix",
      "processor": "arm",
      "release": "24.6.0",
      "system": "Darwin",
      "version": "Darwin Kernel Version 24.6.0"
    }
  }'

# 2. Request a device code
curl -X POST https://platform.parallel.ai/getServiceKeys/device/code \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=$CLIENT_ID&scope=$SCOPE"

# 3. Poll for the token (after user approves in browser)
curl -X POST https://platform.parallel.ai/getServiceKeys/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=$CLIENT_ID&device_code=$DEVICE_CODE"

# 4. Call the Account API with the bearer token
curl https://api.parallel.ai/account/service/<endpoint> \
  -H "Authorization: Bearer $ACCESS_TOKEN"

# 5. Refresh when expired
curl -X POST https://platform.parallel.ai/getServiceKeys/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=refresh_token&refresh_token=$REFRESH_TOKEN"

# 6. Revoke on sign-out
curl -X POST https://platform.parallel.ai/getServiceKeys/token/revoke \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "refresh_token=$REFRESH_TOKEN"
```
