# session_handshake AuthHeader() fallback

## Why this patch belongs in the printed tree

`internal/config/config.go`'s generated `AuthHeader()` only ever read
`AuthHeaderVal`, which nothing populates for `auth.type: session_handshake`.
The harvested/imported token lands in `FlowSessionToken` (from
`FLOW_SESSION_TOKEN` or `auth login --cookies-file`) or the generic
`AccessToken` field, so every `session_handshake` CLI generated from this
spec shape sends an empty `Authorization` header until this fallback is
added. This looks like a generator bug in the `session_handshake` auth
template, not something specific to Flow -- worth a Printing Press retro.

## Reprint guard

On reprint, preserve or regenerate `AuthHeader()` in
`internal/config/config.go` with this fallback order:

```go
func (c *Config) AuthHeader() string {
	if c.AuthHeaderVal != "" {
		return c.AuthHeaderVal
	}
	if c.FlowSessionToken != "" {
		return "Bearer " + c.FlowSessionToken
	}
	if c.AccessToken != "" {
		return "Bearer " + c.AccessToken
	}
	return ""
}
```
