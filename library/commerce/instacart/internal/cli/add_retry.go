package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/instacart"
)

// retryMaxAttempts caps how many candidates tryAddCandidates will try before
// giving up. Set at 3 because autosuggest reliably floats a real match into
// the top two or three positions; more tries pile latency without improving
// the success rate.
const retryMaxAttempts = 3

// notFoundBasketProduct is the only UpdateCartItems ErrorType that triggers a
// retry. Observed symptom: a live-resolved item id is a valid product catalog
// entry but not addable to the active cart's shop (cross-warehouse drift or
// transient stock-out). Other error types surface immediately so callers see
// the real failure.
const notFoundBasketProduct = "notFoundBasketProduct"

// addAttempt is one recorded mutation attempt, exposed in the JSON envelope
// so agents can see which candidates were rejected before a winner (or
// exhaustion).
type addAttempt struct {
	ItemID    string `json:"item_id"`
	Name      string `json:"name,omitempty"`
	ErrorType string `json:"error_type"`
}

// addCandidateResult is returned by tryAddCandidates on success.
type addCandidateResult struct {
	Picked   SearchResult
	Response instacart.UpdateCartItemsResponse
	Attempts []addAttempt
}

// addCandidateError signals exhaustion or a non-retryable error. Carries the
// attempt log so callers can surface it in JSON output and in the guidance
// message.
type addCandidateError struct {
	LastErrorType string
	Attempts      []addAttempt
}

func (e *addCandidateError) Error() string {
	if len(e.Attempts) == 0 {
		return "no candidates to try"
	}
	return fmt.Sprintf("all %d candidate(s) rejected by Instacart (last: %s)", len(e.Attempts), e.LastErrorType)
}

// mutationInvoker is the seam tryAddCandidates calls per attempt. Production
// callers build one with newGQLMutationInvoker; tests inject a fake.
type mutationInvoker func(ctx context.Context, vars instacart.UpdateCartItemsVars) (instacart.UpdateCartItemsResponse, error)

// newGQLMutationInvoker returns a mutationInvoker backed by the real Instacart
// GraphQL client.
func newGQLMutationInvoker(client *gql.Client) mutationInvoker {
	return func(ctx context.Context, vars instacart.UpdateCartItemsVars) (instacart.UpdateCartItemsResponse, error) {
		resp, err := client.Mutation(ctx, "UpdateCartItemsMutation", vars, "")
		if err != nil {
			return instacart.UpdateCartItemsResponse{}, err
		}
		var parsed struct {
			Data instacart.UpdateCartItemsResponse `json:"data"`
		}
		if err := json.Unmarshal(resp.RawBody, &parsed); err != nil {
			return instacart.UpdateCartItemsResponse{}, fmt.Errorf("parse UpdateCartItems response: %w", err)
		}
		return parsed.Data, nil
	}
}

// tryAddCandidates walks the ranked candidate slice, firing the mutation
// against each until one succeeds or the retry cap is hit. Only
// notFoundBasketProduct advances to the next candidate; any other ErrorType
// stops immediately. Transport errors propagate unchanged.
//
// maxAttempts caps total tries; pass 0 to use retryMaxAttempts. The slice
// itself also acts as a cap: if fewer candidates are supplied than the cap,
// the loop just ends.
func tryAddCandidates(
	ctx context.Context,
	invoke mutationInvoker,
	retailer, cartID string,
	candidates []SearchResult,
	qty float64,
	maxAttempts int,
) (*addCandidateResult, error) {
	if len(candidates) == 0 {
		return nil, &addCandidateError{}
	}
	if maxAttempts <= 0 {
		maxAttempts = retryMaxAttempts
	}

	var attempts []addAttempt
	tried := 0
	for _, cand := range candidates {
		if tried >= maxAttempts {
			break
		}
		tried++

		vars := instacart.UpdateCartItemsVars{
			CartItemUpdates: []instacart.CartItemUpdate{{
				ItemID:         cand.ItemID,
				Quantity:       qty,
				QuantityType:   "each",
				TrackingParams: json.RawMessage(`{}`),
			}},
			CartType:         "grocery",
			CartID:           cartID,
			RequestTimestamp: time.Now().UnixMilli(),
		}
		resp, err := invoke(ctx, vars)
		if err != nil {
			return nil, err
		}
		if resp.UpdateCartItems.ErrorType == "" {
			return &addCandidateResult{
				Picked:   cand,
				Response: resp,
				Attempts: attempts,
			}, nil
		}
		attempts = append(attempts, addAttempt{
			ItemID:    cand.ItemID,
			Name:      cand.Name,
			ErrorType: resp.UpdateCartItems.ErrorType,
		})
		if resp.UpdateCartItems.ErrorType != notFoundBasketProduct {
			return nil, &addCandidateError{
				LastErrorType: resp.UpdateCartItems.ErrorType,
				Attempts:      attempts,
			}
		}
	}

	last := notFoundBasketProduct
	if len(attempts) > 0 {
		last = attempts[len(attempts)-1].ErrorType
	}
	return nil, &addCandidateError{LastErrorType: last, Attempts: attempts}
}
