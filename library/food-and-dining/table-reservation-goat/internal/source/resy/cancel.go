// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

// PATCH: resy-source-port — see .printing-press-patches.json for the change-set rationale.

package resy

import (
	"context"
	"encoding/json"
	"fmt"
)

// CancelResponse is the parsed result of a successful /3/cancel call. Either
// Cancelled=true or a non-empty CancelToken is taken as positive
// confirmation; the wire layer carries the raw envelope for callers that
// want to introspect.
type CancelResponse struct {
	Cancelled   bool   `json:"cancelled"`
	CancelToken string `json:"cancel_token"`
	Message     string `json:"message"`
}

// Cancel cancels a reservation by resy_token. Mirrors the TS surface: any
// of {cancelled: true} or {cancel_token: ...} counts as success; everything
// else surfaces the API's message string in the error.
func (c *Client) Cancel(ctx context.Context, reservationID string) (CancelResponse, error) {
	if c.creds.AuthToken == "" {
		return CancelResponse{}, ErrAuthMissing
	}
	body, err := c.rawCancel(ctx, reservationID)
	if err != nil {
		return CancelResponse{}, err
	}
	var resp struct {
		CancelResponse
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if jerr := json.Unmarshal(body, &resp); jerr != nil {
		return CancelResponse{}, fmt.Errorf("resy: parse cancel: %w", jerr)
	}
	if resp.Cancelled || resp.CancelToken != "" {
		return resp.CancelResponse, nil
	}
	msg := resp.Message
	if resp.Error != nil && resp.Error.Message != "" {
		msg = resp.Error.Message
	}
	if msg == "" {
		msg = "resy /3/cancel returned no confirmation"
	}
	return resp.CancelResponse, fmt.Errorf("resy: cancel failed: %s", msg)
}
