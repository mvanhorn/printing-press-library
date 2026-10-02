package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/client"
)

// The shared legacy mirror has no account ownership column. Until that store
// is isolated, suggestions must derive both identity and history from the same
// authenticated client, without reading or writing shared response caches.
func fetchRedemptionSuggestionInputs(ctx context.Context, c *client.Client) (json.RawMessage, []redemptionHistoryRow, error) {
	previousNoCache := c.NoCache
	c.NoCache = true
	defer func() { c.NoCache = previousNoCache }()

	account, err := c.Get(ctx, "/users/me", nil)
	if err != nil {
		return nil, nil, err
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(account, &envelope); err != nil {
		return nil, nil, fmt.Errorf("decode authenticated account: %w", err)
	}
	userData := account
	if len(envelope.Result) > 0 {
		userData = envelope.Result
	}
	var user struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(userData, &user); err != nil || strings.TrimSpace(user.ID) == "" {
		return nil, nil, fmt.Errorf("authenticated account response did not contain a user id")
	}
	path := replacePathParam("/users/{id}/redemptions", "id", user.ID)
	// The endpoint may return a bare array or an object with a result array.
	// The generic paginator recognizes both without a forced response path.
	data, err := paginatedGet(ctx, c, path, map[string]string{"limit": "100"}, nil,
		true, "cursor", "cursor", "limit", 100, "cursor", "meta.has_more")
	if err != nil {
		return nil, nil, err
	}
	var history []redemptionHistoryRow
	if err := json.Unmarshal(collectionItemsForOutput(data, path), &history); err != nil {
		return nil, nil, fmt.Errorf("decode authenticated redemption history: %w", err)
	}
	return account, history, nil
}
