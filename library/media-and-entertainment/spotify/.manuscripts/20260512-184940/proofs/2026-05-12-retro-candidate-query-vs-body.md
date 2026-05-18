# Retro candidate: PUT/DELETE/POST handlers route `in: query` params to the body

**Severity:** high — affects any spec with non-GET methods that put parameters in the URL query string. Spotify, GitHub, Atlassian APIs are all known to do this.
**Category:** generator template gap.
**Component to fix:** `internal/generator/templates/` (specifically the non-GET handler templates) in [mvanhorn/cli-printing-press](https://github.com/mvanhorn/cli-printing-press).

## What happened

While dogfooding `spotify-pp-cli` (run `20260512-184940`), every attempt to add or remove items from the user's Spotify library failed with HTTP 400 `"Missing required field: uris"`, despite the CLI correctly sending `{"uris": ["spotify:track:..."]}` in the request body.

Inspection of `internal/cli/me_save-library-items.go` and `me_remove-library-items.go` showed both handlers built an empty body and sent zero query params:

```go
path := "/me/library"
data, statusCode, err := c.Put(path, body)
```

But Spotify's OpenAPI spec declares `uris` as `in: query` on both `PUT /me/library` and `DELETE /me/library`:

```yaml
parameters:
- name: uris
  required: true
  in: query
  schema: ...
```

A sibling generated file — `me_check-library-contains.go` (`GET /me/library/contains`) — correctly routes `uris` to the query string. So **the generator's GET path respects `in: query` but PUT/DELETE/POST do not.**

## Cross-API impact

This is generic to any OpenAPI spec where a non-GET endpoint declares query parameters. Examples from APIs in the public library that likely hit this same bug today:

- GitHub: `PUT /repos/{owner}/{repo}/topics` with `?names=...`
- Atlassian: `DELETE /rest/api/3/issue/{issueIdOrKey}/votes` with `?notifyUsers=...`
- Most "save items to ..." / "remove items from ..." idioms across vendor APIs

So the fix raises the floor on every printed CLI that wraps a non-GET-with-query-params endpoint, not just the next Spotify-shaped spec.

## Proposed fix

Touch the generator templates for PUT, DELETE, and POST handlers in `internal/generator/templates/` and:

1. Walk the endpoint's parameters list.
2. For each `in: query` parameter, append it to a `params map[string]string` exactly the way the GET template does.
3. Add a `c.PutWithParams(path, params, body)` (and `DeleteWithParams`, `PostWithParams`) method on the generated `internal/client/client.Client` so the params reach `req.URL.Query()` cleanly without string-concatenating them into `path`.

The shipped printed CLI works around the gap by hand-rolling `path := "/me/library?uris=" + url.QueryEscape(flagUris)`, but that's the printed-CLI fix; the machine fix is in the templates so future CLIs get it right at generation time.

## Test case for the verifier

When this fix lands, the golden harness should grow a fixture covering an OpenAPI snippet like:

```yaml
paths:
  /widgets/{id}/tags:
    put:
      operationId: tag-widget
      parameters:
        - name: id
          in: path
          required: true
          schema: {type: string}
        - name: names
          in: query
          required: true
          schema: {type: string}
```

Expected generated handler: builds `params := map[string]string{"names": flagNames}` and calls `c.PutWithParams("/widgets/"+id+"/tags", params, nil)`. Current behavior: builds an empty `body` and drops `names` on the floor.

## Where the bug was first observed

- `~/printing-press/library/spotify/internal/cli/me_save-library-items.go` (now patched)
- `~/printing-press/library/spotify/internal/cli/me_remove-library-items.go` (now patched)
- Spotify OpenAPI spec at `runs/20260512-184940/research/spotify-openapi.yml` — `/me/library` PUT and DELETE

## Next step

Run `/printing-press-retro spotify` from the printing-press repo to formalize this as a GitHub issue, or file directly via `gh issue create --repo mvanhorn/cli-printing-press --title "feat(generator): route in:query parameters to URL query string on PUT/DELETE/POST handlers" --body "$(cat 2026-05-12-retro-candidate-query-vs-body.md)"`.
