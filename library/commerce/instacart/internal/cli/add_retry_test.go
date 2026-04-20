package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/instacart"
)

// scripted builds a mutationInvoker that replays a fixed sequence of
// (errorType, goErr) pairs. Each call consumes one entry. ErrorType "" means
// the response was clean (successful add). A non-nil goErr is returned as a
// transport error regardless of errorType.
type scriptedResp struct {
	errorType string
	goErr     error
}

func scripted(t *testing.T, resps ...scriptedResp) (mutationInvoker, *[]instacart.UpdateCartItemsVars) {
	t.Helper()
	calls := make([]instacart.UpdateCartItemsVars, 0, len(resps))
	idx := 0
	fn := func(_ context.Context, vars instacart.UpdateCartItemsVars) (instacart.UpdateCartItemsResponse, error) {
		calls = append(calls, vars)
		if idx >= len(resps) {
			t.Fatalf("invoker called %d times, only %d scripted responses", idx+1, len(resps))
		}
		r := resps[idx]
		idx++
		if r.goErr != nil {
			return instacart.UpdateCartItemsResponse{}, r.goErr
		}
		var resp instacart.UpdateCartItemsResponse
		resp.UpdateCartItems.ErrorType = r.errorType
		if r.errorType == "" {
			resp.UpdateCartItems.Cart = &instacart.UpdateCartResultCart{ID: "cart-1", ItemCount: 1}
		}
		return resp, nil
	}
	return fn, &calls
}

func candidatesFixture(ids ...string) []SearchResult {
	out := make([]SearchResult, 0, len(ids))
	for _, id := range ids {
		out = append(out, SearchResult{ItemID: id, Name: "item-" + id, ProductID: id, Retailer: "costco"})
	}
	return out
}

func TestTryAddCandidates_FirstSucceeds(t *testing.T) {
	invoke, calls := scripted(t, scriptedResp{errorType: ""})
	got, err := tryAddCandidates(context.Background(), invoke, "costco", "cart-1",
		candidatesFixture("items_1576-A", "items_1576-B"), 1, 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Picked.ItemID != "items_1576-A" {
		t.Fatalf("picked=%q, want items_1576-A", got.Picked.ItemID)
	}
	if len(got.Attempts) != 0 {
		t.Fatalf("attempts=%v, want none", got.Attempts)
	}
	if len(*calls) != 1 {
		t.Fatalf("invoker called %d times, want 1", len(*calls))
	}
}

func TestTryAddCandidates_RetryThenSucceed(t *testing.T) {
	invoke, calls := scripted(t,
		scriptedResp{errorType: notFoundBasketProduct},
		scriptedResp{errorType: ""},
	)
	got, err := tryAddCandidates(context.Background(), invoke, "costco", "cart-1",
		candidatesFixture("items_1576-A", "items_1576-B", "items_1576-C"), 1, 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Picked.ItemID != "items_1576-B" {
		t.Fatalf("picked=%q, want items_1576-B", got.Picked.ItemID)
	}
	if len(got.Attempts) != 1 {
		t.Fatalf("attempts=%d, want 1", len(got.Attempts))
	}
	if got.Attempts[0].ItemID != "items_1576-A" {
		t.Fatalf("attempt[0].ItemID=%q, want items_1576-A", got.Attempts[0].ItemID)
	}
	if got.Attempts[0].ErrorType != notFoundBasketProduct {
		t.Fatalf("attempt[0].ErrorType=%q, want %q", got.Attempts[0].ErrorType, notFoundBasketProduct)
	}
	if len(*calls) != 2 {
		t.Fatalf("invoker called %d times, want 2", len(*calls))
	}
}

func TestTryAddCandidates_RetryCapStopsAtMax(t *testing.T) {
	invoke, calls := scripted(t,
		scriptedResp{errorType: notFoundBasketProduct},
		scriptedResp{errorType: notFoundBasketProduct},
		scriptedResp{errorType: notFoundBasketProduct},
	)
	_, err := tryAddCandidates(context.Background(), invoke, "costco", "cart-1",
		candidatesFixture("A", "B", "C", "D", "E"), 1, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ce *addCandidateError
	if !errors.As(err, &ce) {
		t.Fatalf("err type=%T, want *addCandidateError", err)
	}
	if len(ce.Attempts) != retryMaxAttempts {
		t.Fatalf("attempts=%d, want %d (retryMaxAttempts)", len(ce.Attempts), retryMaxAttempts)
	}
	if len(*calls) != retryMaxAttempts {
		t.Fatalf("invoker called %d times, want %d", len(*calls), retryMaxAttempts)
	}
	if ce.LastErrorType != notFoundBasketProduct {
		t.Fatalf("LastErrorType=%q, want %q", ce.LastErrorType, notFoundBasketProduct)
	}
}

func TestTryAddCandidates_ExhaustShortSlice(t *testing.T) {
	invoke, calls := scripted(t,
		scriptedResp{errorType: notFoundBasketProduct},
		scriptedResp{errorType: notFoundBasketProduct},
	)
	_, err := tryAddCandidates(context.Background(), invoke, "costco", "cart-1",
		candidatesFixture("A", "B"), 1, 0)
	var ce *addCandidateError
	if !errors.As(err, &ce) {
		t.Fatalf("err type=%T, want *addCandidateError", err)
	}
	if len(ce.Attempts) != 2 {
		t.Fatalf("attempts=%d, want 2", len(ce.Attempts))
	}
	if len(*calls) != 2 {
		t.Fatalf("invoker called %d times, want 2", len(*calls))
	}
}

func TestTryAddCandidates_NonRetryableErrorStopsImmediately(t *testing.T) {
	invoke, calls := scripted(t,
		scriptedResp{errorType: "soldOut"},
	)
	_, err := tryAddCandidates(context.Background(), invoke, "costco", "cart-1",
		candidatesFixture("A", "B", "C"), 1, 0)
	var ce *addCandidateError
	if !errors.As(err, &ce) {
		t.Fatalf("err type=%T, want *addCandidateError", err)
	}
	if ce.LastErrorType != "soldOut" {
		t.Fatalf("LastErrorType=%q, want soldOut", ce.LastErrorType)
	}
	if len(ce.Attempts) != 1 {
		t.Fatalf("attempts=%d, want 1 (first attempt failed with non-retryable)", len(ce.Attempts))
	}
	if len(*calls) != 1 {
		t.Fatalf("invoker called %d times, want 1 (should not continue past non-retryable)", len(*calls))
	}
}

func TestTryAddCandidates_TransportErrorPropagates(t *testing.T) {
	boom := errors.New("network down")
	invoke, calls := scripted(t, scriptedResp{goErr: boom})
	_, err := tryAddCandidates(context.Background(), invoke, "costco", "cart-1",
		candidatesFixture("A", "B"), 1, 0)
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v, want wraps %v", err, boom)
	}
	if len(*calls) != 1 {
		t.Fatalf("invoker called %d times, want 1 (transport error stops retry)", len(*calls))
	}
}

func TestTryAddCandidates_EmptyCandidates(t *testing.T) {
	invoke, calls := scripted(t)
	_, err := tryAddCandidates(context.Background(), invoke, "costco", "cart-1", nil, 1, 0)
	var ce *addCandidateError
	if !errors.As(err, &ce) {
		t.Fatalf("err type=%T, want *addCandidateError", err)
	}
	if len(ce.Attempts) != 0 {
		t.Fatalf("attempts=%d, want 0", len(ce.Attempts))
	}
	if len(*calls) != 0 {
		t.Fatalf("invoker called %d times, want 0", len(*calls))
	}
}

func TestTryAddCandidates_MaxAttemptsOverride(t *testing.T) {
	invoke, calls := scripted(t,
		scriptedResp{errorType: notFoundBasketProduct},
		scriptedResp{errorType: ""},
	)
	got, err := tryAddCandidates(context.Background(), invoke, "costco", "cart-1",
		candidatesFixture("A", "B", "C"), 1, 2)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Picked.ItemID != "B" {
		t.Fatalf("picked=%q, want B", got.Picked.ItemID)
	}
	if len(*calls) != 2 {
		t.Fatalf("invoker called %d times, want 2 (maxAttempts=2)", len(*calls))
	}
}

func TestTryAddCandidates_SingleCandidateItemIDPath(t *testing.T) {
	invoke, calls := scripted(t, scriptedResp{errorType: notFoundBasketProduct})
	_, err := tryAddCandidates(context.Background(), invoke, "costco", "cart-1",
		candidatesFixture("items_1576-X"), 1, 1)
	var ce *addCandidateError
	if !errors.As(err, &ce) {
		t.Fatalf("err type=%T, want *addCandidateError", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("invoker called %d times, want 1 (single-candidate path)", len(*calls))
	}
	if ce.LastErrorType != notFoundBasketProduct {
		t.Fatalf("LastErrorType=%q", ce.LastErrorType)
	}
}
