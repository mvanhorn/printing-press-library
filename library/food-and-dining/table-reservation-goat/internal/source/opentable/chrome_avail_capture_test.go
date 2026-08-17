// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package opentable

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/network"
)

func boundCapture(t *testing.T, ids ...network.RequestID) *availabilityCaptureCoordinator {
	t.Helper()
	c := newAvailabilityCaptureCoordinator()
	c.Arm()
	for _, id := range ids {
		c.RequestWillBeSent(id)
		if action, result := c.BindRequest(id, true); action != nil || result != nil {
			t.Fatalf("BindRequest(%s) emitted action=%v result=%v before response/loading completion", id, action, result)
		}
	}
	return c
}

func TestAvailabilityCaptureCoordinator_EventOrdering(t *testing.T) {
	t.Run("response alone does not fetch", func(t *testing.T) {
		c := boundCapture(t, "one")
		if action := c.ResponseReceived("one", 200); action != nil {
			t.Fatalf("ResponseReceived emitted early fetch: %+v", action)
		}
	})

	t.Run("response then loading finished", func(t *testing.T) {
		c := boundCapture(t, "one")
		if action := c.ResponseReceived("one", 200); action != nil {
			t.Fatalf("ResponseReceived emitted early fetch: %+v", action)
		}
		action := c.LoadingFinished("one")
		if action == nil || action.requestID != "one" {
			t.Fatalf("LoadingFinished action = %+v; want fetch for one", action)
		}
		result := c.FetchCompleted("one", []byte(`{"ok":true}`), nil)
		if result == nil || result.err != nil || result.status != 200 {
			t.Fatalf("FetchCompleted result = %+v; want HTTP 200 success", result)
		}
	})

	t.Run("loading finished before response", func(t *testing.T) {
		c := boundCapture(t, "one")
		if action := c.LoadingFinished("one"); action != nil {
			t.Fatalf("LoadingFinished emitted early fetch: %+v", action)
		}
		action := c.ResponseReceived("one", 204)
		if action == nil || action.requestID != "one" {
			t.Fatalf("ResponseReceived action = %+v; want fetch for one", action)
		}
	})

	t.Run("loading failed before response", func(t *testing.T) {
		c := boundCapture(t, "one")
		result := c.LoadingFailed("one", "net::ERR_FAILED")
		if result == nil || result.err == nil || !strings.Contains(result.err.Error(), "net::ERR_FAILED") {
			t.Fatalf("LoadingFailed result = %+v; want terminal reason", result)
		}
		if action := c.ResponseReceived("one", 200); action != nil {
			t.Fatalf("late response emitted fetch: %+v", action)
		}
	})

	t.Run("response then loading failed", func(t *testing.T) {
		c := boundCapture(t, "one")
		c.ResponseReceived("one", 502)
		result := c.LoadingFailed("one", "connection reset")
		if result == nil || result.err == nil || !strings.Contains(result.err.Error(), "connection reset") {
			t.Fatalf("LoadingFailed result = %+v; want terminal reason", result)
		}
	})

	t.Run("cache response with empty body is an error", func(t *testing.T) {
		c := boundCapture(t, "cached")
		c.ResponseReceived("cached", 200)
		action := c.LoadingFinished("cached")
		if action == nil {
			t.Fatal("cache-served request did not emit fetch after loading completion")
		}
		result := c.FetchCompleted("cached", nil, nil)
		if result == nil || result.err == nil || !strings.Contains(result.err.Error(), "empty") {
			t.Fatalf("empty body result = %+v; want error", result)
		}
	})

	t.Run("duplicate and late events are ignored", func(t *testing.T) {
		c := boundCapture(t, "one")
		c.ResponseReceived("one", 200)
		if action := c.ResponseReceived("one", 500); action != nil {
			t.Fatalf("duplicate response emitted action: %+v", action)
		}
		action := c.LoadingFinished("one")
		if action == nil {
			t.Fatal("first completion did not emit fetch")
		}
		if duplicate := c.LoadingFinished("one"); duplicate != nil {
			t.Fatalf("duplicate completion emitted action: %+v", duplicate)
		}
		result := c.FetchCompleted("one", []byte("body"), nil)
		if result == nil || result.status != 200 {
			t.Fatalf("result = %+v; duplicate response overwrote status", result)
		}
		if late := c.LoadingFailed("one", "late"); late != nil {
			t.Fatalf("late failure emitted result: %+v", late)
		}
	})
}

func TestAvailabilityCaptureCoordinator_ArmingAndBinding(t *testing.T) {
	c := newAvailabilityCaptureCoordinator()
	c.RequestWillBeSent("warmup")
	c.Arm()
	if action, result := c.BindRequest("warmup", true); action != nil || result != nil {
		t.Fatalf("warm-up request was captured: action=%v result=%v", action, result)
	}
	c.RequestWillBeSent("other-restaurant")
	c.BindRequest("other-restaurant", false)
	c.ResponseReceived("other-restaurant", 200)
	if action := c.LoadingFinished("other-restaurant"); action != nil {
		t.Fatalf("non-target restaurant emitted fetch: %+v", action)
	}
}

func TestRestaurantIDsFromPostData(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want []int
		ok   bool
	}{
		{name: "numeric ids", body: `{"variables":{"restaurantIds":[1080775,42]}}`, want: []int{1080775, 42}, ok: true},
		{name: "string id", body: `{"variables":{"restaurantIds":["1080775"]}}`, want: []int{1080775}, ok: true},
		{name: "missing ids", body: `{"variables":{}}`},
		{name: "hash-only persisted query", body: `{"extensions":{"persistedQuery":{"sha256Hash":"0123456789abcdef"}}}`},
		{name: "malformed", body: `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := restaurantIDsFromPostData(tc.body)
			if ok != tc.ok || fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("restaurantIDsFromPostData() = (%v, %v); want (%v, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestAvailabilityCaptureCoordinator_MultipleCandidates(t *testing.T) {
	for _, tc := range []struct {
		name         string
		successFirst bool
	}{
		{name: "failed candidate completes first"},
		{name: "successful candidate completes first", successFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := boundCapture(t, "failed", "successful")
			c.ResponseReceived("failed", 500)
			failedAction := c.LoadingFinished("failed")
			c.ResponseReceived("successful", 200)
			successAction := c.LoadingFinished("successful")
			if failedAction == nil || successAction == nil {
				t.Fatalf("actions = failed:%+v successful:%+v; want both", failedAction, successAction)
			}
			if tc.successFirst {
				result := c.FetchCompleted("successful", []byte("winner"), nil)
				if result == nil || string(result.body) != "winner" {
					t.Fatalf("successful result = %+v", result)
				}
				if late := c.FetchCompleted("failed", nil, errors.New("failed")); late != nil {
					t.Fatalf("late failure preempted winner: %+v", late)
				}
				return
			}
			if early := c.FetchCompleted("failed", nil, errors.New("failed")); early != nil {
				t.Fatalf("failed candidate preempted pending success: %+v", early)
			}
			result := c.FetchCompleted("successful", []byte("winner"), nil)
			if result == nil || string(result.body) != "winner" {
				t.Fatalf("successful result = %+v", result)
			}
		})
	}
}

func TestAvailabilityCaptureCoordinator_UnresolvedBindingStaysPending(t *testing.T) {
	c := newAvailabilityCaptureCoordinator()
	c.Arm()
	c.RequestWillBeSent("failed")
	c.RequestWillBeSent("pending")
	c.BindRequest("failed", true)
	c.ResponseReceived("failed", 500)
	failedAction := c.LoadingFinished("failed")
	if failedAction == nil {
		t.Fatal("failed candidate did not emit fetch action")
	}
	c.ResponseReceived("pending", 200)
	if action := c.LoadingFinished("pending"); action != nil {
		t.Fatalf("unbound pending request emitted action: %+v", action)
	}
	if result := c.FetchCompleted(failedAction.requestID, nil, errors.New("failed candidate")); result != nil {
		t.Fatalf("failed candidate preempted unresolved binding: %+v", result)
	}
	action, result := c.BindRequest("pending", true)
	if result != nil || action == nil || action.requestID != "pending" {
		t.Fatalf("pending BindRequest = action:%+v result:%+v; want fetch action", action, result)
	}
	result = c.FetchCompleted("pending", []byte("winner"), nil)
	if result == nil || result.err != nil || string(result.body) != "winner" {
		t.Fatalf("pending candidate result = %+v; want success", result)
	}
}

func TestAvailabilityCaptureCoordinator_NonTargetFailureDoesNotReplaceBoundFailure(t *testing.T) {
	c := newAvailabilityCaptureCoordinator()
	c.Arm()
	c.RequestWillBeSent("target")
	c.RequestWillBeSent("non-target")
	c.BindRequest("target", true)
	c.ResponseReceived("target", 500)
	targetAction := c.LoadingFinished("target")
	if targetAction == nil {
		t.Fatal("target candidate did not emit fetch action")
	}
	if result := c.FetchCompleted(targetAction.requestID, nil, errors.New("target fetch failed")); result != nil {
		t.Fatalf("target failure preempted unresolved binding: %+v", result)
	}
	if result := c.LoadingFailed("non-target", "non-target load failed"); result != nil {
		t.Fatalf("unbound non-target failure emitted result: %+v", result)
	}
	action, result := c.BindRequest("non-target", false)
	if action != nil || result == nil || result.err == nil {
		t.Fatalf("non-target BindRequest = action:%+v result:%+v; want settled target failure", action, result)
	}
	if !strings.Contains(result.err.Error(), "target fetch failed") || strings.Contains(result.err.Error(), "non-target load failed") {
		t.Fatalf("settled error = %v; want bound target failure only", result.err)
	}
}

func TestFetchResponseBodyWithRetry(t *testing.T) {
	notReady := func() error {
		return fmt.Errorf("wrapped: %w", &cdproto.Error{Code: -32000, Message: "No resource with given identifier found"})
	}

	t.Run("exact transient then success", func(t *testing.T) {
		calls := 0
		body, err := fetchResponseBodyWithRetry(context.Background(), "one", func(context.Context, network.RequestID) ([]byte, error) {
			calls++
			if calls == 1 {
				return nil, notReady()
			}
			return []byte("body"), nil
		}, time.Millisecond)
		if err != nil || string(body) != "body" || calls != 2 {
			t.Fatalf("body=%q err=%v calls=%d; want success on call 2", body, err, calls)
		}
	})

	t.Run("persistent transient is wrapped", func(t *testing.T) {
		calls := 0
		_, err := fetchResponseBodyWithRetry(context.Background(), "one", func(context.Context, network.RequestID) ([]byte, error) {
			calls++
			return nil, notReady()
		}, time.Millisecond)
		if err == nil || calls != 3 || !strings.Contains(err.Error(), "after 3 attempt") {
			t.Fatalf("err=%v calls=%d; want wrapped persistent failure", err, calls)
		}
		var cdpErr *cdproto.Error
		if !errors.As(err, &cdpErr) {
			t.Fatalf("wrapped error lost *cdproto.Error: %v", err)
		}
	})

	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "same code different message", err: &cdproto.Error{Code: -32000, Message: "Another failure"}},
		{name: "different code same message", err: &cdproto.Error{Code: -32001, Message: "No resource with given identifier found"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			_, err := fetchResponseBodyWithRetry(context.Background(), "one", func(context.Context, network.RequestID) ([]byte, error) {
				calls++
				return nil, tc.err
			}, time.Millisecond)
			if err == nil || calls != 1 {
				t.Fatalf("err=%v calls=%d; want one fetch and no retry", err, calls)
			}
		})
	}

	t.Run("context canceled during retry backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		_, err := fetchResponseBodyWithRetry(ctx, "one", func(context.Context, network.RequestID) ([]byte, error) {
			calls++
			cancel()
			return nil, notReady()
		}, time.Hour)
		if !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("err=%v calls=%d; want cancellation during backoff", err, calls)
		}
	})
}
