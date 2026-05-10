// Hand-authored bridge: spec-mapped commands generated for absorbed `/graphql`
// endpoints originally POSTed an empty body, which Monarch rejects with HTTP 400.
// gqlPost adapts those generator code paths onto the typed GraphQL client by
// returning (data, statusCode, err) — the shape the generated rendering code
// already consumes — so each call site needs only the one-line client swap.

package cli

import (
	"encoding/json"
	"errors"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func gqlPost(c *client.Client, query string, vars map[string]any) (json.RawMessage, int, error) {
	if vars == nil {
		vars = map[string]any{}
	}
	data, err := c.Query(query, vars)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return nil, apiErr.StatusCode, err
		}
		return nil, 0, err
	}
	return data, 200, nil
}

// fetchOwnerUserID returns the authenticated user's ID via the `me` query.
// Required by createTransaction's CreateTransactionMutationInput, which has an
// ownerUserId field that the web app populates from the same source.
func fetchOwnerUserID(c *client.Client) (string, error) {
	data, err := c.Query(client.MeGetQuery, nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Me struct {
			ID string `json:"id"`
		} `json:"me"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if resp.Me.ID == "" {
		return "", errors.New("me.id missing from response")
	}
	return resp.Me.ID, nil
}

// fetchUncategorizedID returns the household's Uncategorized category UUID by
// matching name in the categories list. Each Monarch household has its own
// Uncategorized id, so this resolution must happen at runtime per user — the
// `categoryId` field is required by createTransaction.
func fetchUncategorizedID(c *client.Client) (string, error) {
	data, err := c.Query(client.CategoriesListQuery, nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Categories []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	for _, cat := range resp.Categories {
		if cat.Name == "Uncategorized" {
			return cat.ID, nil
		}
	}
	return "", errors.New("Uncategorized category not found in categories list")
}
