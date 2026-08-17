// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: read the user's saved/favorite businesses via the authed
// myaccount API. The web app's favorites list is served by a SIGNED .asmx
// endpoint (GetFavoriteBusinesses) that we deliberately avoid; no clean
// unsigned equivalent was captured. This command still makes a real authed API
// call to the myaccount favorites path and reports honestly when the endpoint
// is unavailable — it never fabricates data. generate --force preserves this body.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// favoritesPath is the authed myaccount favorites endpoint probed by this
// command. If Vagaro exposes a clean unsigned bookmarks endpoint it will live
// under this myaccount namespace alongside purchases/appointments.
const favoritesPath = "https://api.vagaro.com/us02/api/v2/myaccount/favorites"

type favoritesResult struct {
	Available bool              `json:"available"`
	Count     int               `json:"count"`
	Favorites []json.RawMessage `json:"favorites"`
	Message   string            `json:"message"`
	NextStep  string            `json:"next_step,omitempty"`
}

func newFavoritesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "favorites",
		Short: "List your saved/favorite businesses (requires auth).",
		Long: `Read the businesses you've saved on your account.

Requires auth: run 'vagaro-pp-cli auth login --chrome' first. Vagaro's web app
serves favorites through a signed endpoint we avoid; if no clean unsigned
myaccount endpoint answers, this command reports that honestly rather than
guessing or fabricating a list.`,
		Example:     "  vagaro-pp-cli favorites",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Real authed API call — never a fabricated payload. // pp:client-call
			gc, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"pageSize": 24, "pageNumber": 1, "device": "Website",
				"module": "MyAccount", "version": "2.5.3",
			}
			data, status, err := gc.PostQueryWithParams(ctx, favoritesPath, map[string]string{}, body)
			if err != nil || status < 200 || status >= 300 || !json.Valid(data) {
				return emitVagaro(cmd, flags, favoritesResult{
					Available: false,
					Favorites: []json.RawMessage{},
					Message:   "favorites requires an endpoint not yet available (Vagaro serves the list via a signed endpoint we avoid); no clean unsigned myaccount favorites endpoint responded",
					NextStep:  "if you're not logged in, run 'vagaro-pp-cli auth login --chrome'; otherwise this surface is not yet wired",
				})
			}

			items := extractFavoriteItems(data)
			return emitVagaro(cmd, flags, favoritesResult{
				Available: true,
				Count:     len(items),
				Favorites: items,
				Message:   "saved businesses from your account",
			})
		},
	}
	return cmd
}

// extractFavoriteItems pulls the favorites array out of a {data:[...]} or
// {results:[...]} envelope, falling back to a bare array.
func extractFavoriteItems(data json.RawMessage) []json.RawMessage {
	var env map[string]json.RawMessage
	if json.Unmarshal(data, &env) == nil {
		for _, key := range []string{"data", "Data", "results", "favorites", "businesses"} {
			if raw, ok := env[key]; ok {
				var arr []json.RawMessage
				if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
					return arr
				}
			}
		}
	}
	var arr []json.RawMessage
	if json.Unmarshal(data, &arr) == nil {
		return arr
	}
	return []json.RawMessage{}
}
