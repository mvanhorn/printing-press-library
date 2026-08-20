# Namecheap API research

Source: https://www.namecheap.com/support/api/intro/

Namecheap exposes its public API through `/xml.response`, selected by a `Command` query parameter. The CLI uses resource-shaped pseudo-paths in `spec.yaml` so the public command surface is `domains`, `dns`, `users`, and `ssl` rather than `xml-response`. Runtime request preparation maps those commands back to `/xml.response` and injects `ApiUser`, `ApiKey`, `UserName`, `ClientIp`, and `Command`.

Credential env vars: `NAMECHEAP_USERNAME`, `NAMECHEAP_API_KEY`, optional `NAMECHEAP_CLIENT_IP`, optional `NAMECHEAP_SANDBOX`.

Namecheap returns XML and can signal failures with HTTP 200 plus `ApiResponse Status="ERROR"`, so the client converts XML envelopes to JSON and treats Status=ERROR as command failure.
