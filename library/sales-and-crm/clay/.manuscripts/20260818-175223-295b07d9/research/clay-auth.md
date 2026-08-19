> ## Documentation Index
> Fetch the complete documentation index at: https://developers.clay.com/llms.txt
> Use this file to discover all available pages before exploring further.

# Authentication

> Authenticate requests with a Clay API key.

Every request to the Clay Public API must include a Clay API key in the `clay-api-key` header. Store your key in an environment variable and pass it in the header:

```bash theme={"theme":{"light":"github-light","dark":"github-dark"}}
export CLAY_PUBLIC_API_KEY="your_api_key"

curl https://api.clay.com/public/v0/me \
  -H "clay-api-key: $CLAY_PUBLIC_API_KEY"
```

API keys are tied to a Clay user and workspace access. Create one under [Settings → Account → API keys (beta)](https://app.clay.com/workspaces/~/settings/account?accountTab=api-keys-beta). Keep keys server-side and do not expose them in browser code, mobile apps, public repositories, logs, or analytics tools.

## Header

| Header         | Required | Description                               |
| -------------- | -------- | ----------------------------------------- |
| `clay-api-key` | Yes      | Personal API key for the Clay Public API. |

## Failed authentication

Requests without a valid API key return a non-2xx response with the standard error shape:

```json theme={"theme":{"light":"github-light","dark":"github-dark"}}
{
  "message": "Authentication failed"
}
```
