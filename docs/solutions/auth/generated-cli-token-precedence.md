---
name: generated-cli-token-precedence
date: 2026-06-22
problem_type: bug
category: auth
component: library/marketing/attentive/internal/config/config.go
root_cause: Generated token rotation saved the new token to access_token while a higher-priority bearer_auth field could remain in configForSave and keep shadowing it.
resolution_type: fix
tags: [generated-cli, auth, config-precedence, greptile]
---

# Generated CLI token rotation can silently preserve a stale higher-priority credential

## Symptoms

Greptile flagged PR #1316 because Attentive's `auth set-token` wrote the new token through:

```go
cfg.SaveTokens("", "", args[0], "", cfg.TokenExpiry)
```

That persisted `access_token`, but `Config.AuthHeader()` resolves credentials in higher-priority order:

```go
if c.AuthHeaderVal != "" {
	return c.AuthHeaderVal
}
if c.AttentiveBearerAuth != "" {
	return "Bearer " + c.AttentiveBearerAuth
}
if c.AccessToken != "" {
	return "Bearer " + c.AccessToken
}
```

If a config file already had `bearer_auth = "old-token"`, `auth set-token new-token` could complete successfully while runtime auth still used the old bearer token.

The less obvious variant is `ATTENTIVE_BEARER_AUTH`: `Load` marks `AttentiveBearerAuth` as an env override, and `configForSave` preserves the file snapshot for that field. Clearing only the runtime struct in the CLI is not enough in that path; the stale file value can survive the save and shadow `access_token` later when the env var is removed.

## What didn't work

- Clearing only `cfg.AuthHeaderVal` in the CLI did not address `bearer_auth`, which has higher precedence than `access_token`.
- Clearing only `cfg.AttentiveBearerAuth` in the CLI handled a normal file config, but not the env-override case because `configForSave` intentionally restores file-backed `AttentiveBearerAuth` when `envOverrides["AttentiveBearerAuth"]` is still true.
- Treating this as a Postscript-only pattern was a red herring. Postscript has a `SaveCredential` path that deletes the relevant env override before saving; Attentive used `SaveTokens` instead.

## Solution

Clear the higher-priority bearer credential in the config saver, delete its env-override marker, and update the file snapshot before writing:

```go
func (c *Config) SaveTokens(clientID, clientSecret, accessToken, refreshToken string, expiry time.Time) error {
	c.ClientID = clientID
	c.ClientSecret = clientSecret
	c.AccessToken = accessToken
	c.RefreshToken = refreshToken
	c.TokenExpiry = expiry
	c.AttentiveBearerAuth = ""
	delete(c.envOverrides, "ClientID")
	delete(c.envOverrides, "ClientSecret")
	delete(c.envOverrides, "AccessToken")
	delete(c.envOverrides, "RefreshToken")
	delete(c.envOverrides, "TokenExpiry")
	delete(c.envOverrides, "AttentiveBearerAuth")
	c.updateFileConfigField("ClientID")
	c.updateFileConfigField("ClientSecret")
	c.updateFileConfigField("AccessToken")
	c.updateFileConfigField("RefreshToken")
	c.updateFileConfigField("TokenExpiry")
	c.updateFileConfigField("AttentiveBearerAuth")
	return c.save()
}
```

Keep the CLI call explicit too:

```go
cfg.AuthHeaderVal = ""
cfg.AttentiveBearerAuth = ""
if err := cfg.SaveTokens("", "", args[0], "", cfg.TokenExpiry); err != nil {
	return configErr(fmt.Errorf("saving token: %w", err))
}
```

Lock the behavior with tests for both the plain file case and the env-override case. After `auth set-token`, reload the config with `ATTENTIVE_BEARER_AUTH` cleared and assert `AuthHeader()` resolves to the new token and that the old file token is absent.

## Why this works

The fix makes the persistence layer own the invariant: token rotation must remove stale higher-priority credential fields from the saved config before the new `access_token` can be trusted. Deleting the env-override marker is the key step for the env-var path because it allows `updateFileConfigField("AttentiveBearerAuth")` to replace the file snapshot instead of preserving stale TOML.

## Prevention

- Any generated `auth set-token` or `SaveTokens` path must clear all auth fields with precedence above the target persisted token.
- When a config saver changes a field that can be env-overridden, delete that field from `envOverrides` before calling `updateFileConfigField`.
- Regression test both file-only and env-overridden configs when auth precedence is involved.
