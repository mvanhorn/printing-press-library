// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package opentable

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/network"
)

type availabilityCaptureAction struct {
	requestID network.RequestID
}

type availabilityCaptureResult struct {
	body   []byte
	status int
	err    error
}

type availabilityCaptureRequest struct {
	bound         bool
	boundKnown    bool
	responseKnown bool
	status        int
	loadingDone   bool
	fetchIssued   bool
	terminal      bool
	err           error
}

// availabilityCaptureCoordinator is the I/O-free state machine for one
// Chrome availability run. Callers execute emitted actions separately and
// publish their result back through FetchCompleted.
type availabilityCaptureCoordinator struct {
	armed       bool
	terminal    bool
	lastFailure error
	requests    map[network.RequestID]*availabilityCaptureRequest
}

func newAvailabilityCaptureCoordinator() *availabilityCaptureCoordinator {
	return &availabilityCaptureCoordinator{requests: make(map[network.RequestID]*availabilityCaptureRequest)}
}

func (c *availabilityCaptureCoordinator) Arm() {
	if !c.terminal {
		c.armed = true
	}
}

func (c *availabilityCaptureCoordinator) RequestWillBeSent(id network.RequestID) {
	if !c.armed || c.terminal {
		return
	}
	if _, ok := c.requests[id]; !ok {
		c.requests[id] = &availabilityCaptureRequest{}
	}
}

func (c *availabilityCaptureCoordinator) BindRequest(id network.RequestID, matches bool) (*availabilityCaptureAction, *availabilityCaptureResult) {
	r := c.requests[id]
	if r == nil || r.boundKnown || c.terminal {
		return nil, nil
	}
	r.boundKnown = true
	r.bound = matches
	if !matches {
		return nil, c.finishFailureIfSettled()
	}
	if r.terminal {
		c.lastFailure = r.err
		return nil, c.finishFailureIfSettled()
	}
	return c.fetchIfReady(id, r), nil
}

func (c *availabilityCaptureCoordinator) ResponseReceived(id network.RequestID, status int) *availabilityCaptureAction {
	r := c.requests[id]
	if r == nil || r.responseKnown || r.terminal || c.terminal {
		return nil
	}
	r.responseKnown = true
	r.status = status
	return c.fetchIfReady(id, r)
}

func (c *availabilityCaptureCoordinator) LoadingFinished(id network.RequestID) *availabilityCaptureAction {
	r := c.requests[id]
	if r == nil || r.loadingDone || r.terminal || c.terminal {
		return nil
	}
	r.loadingDone = true
	return c.fetchIfReady(id, r)
}

func (c *availabilityCaptureCoordinator) LoadingFailed(id network.RequestID, reason string) *availabilityCaptureResult {
	r := c.requests[id]
	if r == nil || r.loadingDone || r.terminal || c.terminal {
		return nil
	}
	r.loadingDone = true
	r.terminal = true
	r.err = fmt.Errorf("request %s loading failed: %s", id, reason)
	if !r.boundKnown || !r.bound {
		return nil
	}
	c.lastFailure = r.err
	return c.finishFailureIfSettled()
}

func (c *availabilityCaptureCoordinator) FetchCompleted(id network.RequestID, body []byte, err error) *availabilityCaptureResult {
	r := c.requests[id]
	if r == nil || !r.fetchIssued || r.terminal || c.terminal {
		return nil
	}
	r.terminal = true
	if err == nil && len(body) == 0 {
		err = errors.New("response body was empty")
	}
	if err != nil {
		r.err = err
		c.lastFailure = err
		return c.finishFailureIfSettled()
	}
	c.terminal = true
	return &availabilityCaptureResult{body: body, status: r.status}
}

func (c *availabilityCaptureCoordinator) fetchIfReady(id network.RequestID, r *availabilityCaptureRequest) *availabilityCaptureAction {
	if !r.boundKnown || !r.bound || !r.responseKnown || !r.loadingDone || r.fetchIssued || r.terminal || c.terminal {
		return nil
	}
	r.fetchIssued = true
	return &availabilityCaptureAction{requestID: id}
}

func (c *availabilityCaptureCoordinator) finishFailureIfSettled() *availabilityCaptureResult {
	boundCount := 0
	for _, r := range c.requests {
		if !r.boundKnown {
			return nil
		}
		if !r.bound {
			continue
		}
		boundCount++
		if !r.terminal {
			return nil
		}
	}
	if boundCount == 0 || c.lastFailure == nil {
		return nil
	}
	c.terminal = true
	return &availabilityCaptureResult{err: c.lastFailure}
}

type responseBodyFetcher func(context.Context, network.RequestID) ([]byte, error)

func fetchResponseBodyWithRetry(ctx context.Context, reqID network.RequestID, fetch responseBodyFetcher, backoff time.Duration) ([]byte, error) {
	const attempts = 3
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		var body []byte
		body, err = fetch(ctx, reqID)
		if err == nil {
			return body, nil
		}
		if !isResponseBodyNotReady(err) || attempt == attempts {
			return nil, fmt.Errorf("fetch response body for request %s after %d attempt(s): %w", reqID, attempt, err)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil, fmt.Errorf("fetch response body for request %s retry backoff: %w", reqID, ctx.Err())
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("fetch response body for request %s: %w", reqID, err)
}

func isResponseBodyNotReady(err error) bool {
	var cdpErr *cdproto.Error
	return errors.As(err, &cdpErr) && cdpErr.Code == -32000 && cdpErr.Message == "No resource with given identifier found"
}
