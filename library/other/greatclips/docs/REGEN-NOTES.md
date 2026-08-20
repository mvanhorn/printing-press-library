# Regeneration Notes

If you regenerate this CLI via `printing-press generate`, two hand-written
patches will be overwritten. Re-apply them after every regen, or upstream
the changes to the Printing Press generator template.

## Patch 1: PreRequestHook field on Client (internal/client/client.go)

`client.go` carries a "DO NOT EDIT" banner because it's emitted by the
generator. v0.2 adds one field to the Client struct and one invocation
in the request loop:

```go
type Client struct {
    // ... existing fields ...
    PreRequestHook func(*http.Request) error
}
```

```go
// In Client.do, after header overrides are applied:
if c.PreRequestHook != nil {
    if hookErr := c.PreRequestHook(req); hookErr != nil {
        return nil, 0, fmt.Errorf("pre-request hook: %w", hookErr)
    }
}
```

The hook is also invoked from `Client.dryRun` against a synthetic request so
the dry-run output shows the URL with `s=****` masking.

The `maskSigQueryParam` helper sits next to `maskToken` in the same file.

## Patch 2: Hook registration (internal/cli/root.go)

`rootFlags.newClient()` sets the hook after construction:

```go
c.PreRequestHook = newICSSignHook()
```

`newICSSignHook` lives in `internal/cli/icssign_hook.go` (hand-authored,
not generator-owned).

## Why this lives outside the generator (for now)

The signing logic is API-specific — every other Printing Press CLI would
not benefit. Two options for v0.3:

1. **Keep as-is**: accept that regen overwrites client.go's PreRequestHook
   field and root.go's hook assignment. Re-apply by hand. The two patches
   are small (~10 lines each) and obvious.
2. **Upstream**: add a generic `PreRequestHook` field to the generator's
   `client.go.tmpl` template — costs nothing for CLIs that never set it,
   and makes the extension point first-class.

Option 2 is the right long-term answer. File a Printing Press retro
issue when v0.3 is being scoped.

## Test coverage

`internal/icssign/sign_test.go` carries the golden vector
(`Sign("1778530000000[{\"storeNumber\":\"8991\"}]")` →
`Y1p7j0qekK28DOLVNF2CkxhGhLvEW9PVtm30sEiMZas`). Any regen that drops
`internal/icssign/` entirely will lose this; the package is hand-authored
and lives outside `internal/cliutil/` (generator-reserved) for exactly
this reason.

## Debug

`GREATCLIPS_DEBUG_SIGN=1 greatclips-pp-cli wait --store-number 8991 --dry-run`
prints the exact bytes the hook signs:

```
[icssign] host=www.stylewaretouch.net method=POST
[icssign] t=1778536401567
[icssign] body_bytes=24 body="[{\"storeNumber\":\"8991\"}]"
[icssign] signing_input="1778536401567[{\"storeNumber\":\"8991\"}]"
[icssign] s=taWqK1Qq-acloQT2sPFigU5ONfhnHvuLhdSu0c7cQ-Q
```

Use this to confirm the Go-computed signature matches the JS algorithm.
Run the same input through the SPA's `generateICSSignature` in browser
DevTools to verify byte-identity.
