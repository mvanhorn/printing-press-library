// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Small helpers supporting the tail + waterfall commands: a wrapper around
// linkedin.IsLoggedIn (so tail.go doesn't have to import the linkedin package
// just for one bool), and a LinkedIn inbox snapshot helper.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/linkedin"
)

func linkedinIsLoggedIn() (bool, error) {
	return linkedin.IsLoggedIn()
}

// fetchLinkedInInboxSnapshot spawns the MCP subprocess, calls get_inbox with
// a modest limit, and returns the result as a []json.RawMessage for the
// caller to diff against the previous snapshot. The subprocess is always
// torn down on return.
func fetchLinkedInInboxSnapshot(parent context.Context, flags *rootFlags) ([]json.RawMessage, error) {
	ctx, cancel := signalCtx(parent)
	defer cancel()
	client, err := spawnLIClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	if _, err := client.Initialize(ctx, linkedin.Implementation{Name: "contact-goat-pp-cli", Version: version}); err != nil {
		return nil, fmt.Errorf("initialize linkedin-mcp: %w", err)
	}
	callCtx, callCancel := context.WithTimeout(ctx, flags.timeout)
	defer callCancel()
	result, err := client.CallTool(callCtx, linkedin.ToolNames.Inbox, map[string]any{"limit": 25})
	if err != nil {
		return nil, err
	}
	payload := linkedin.TextPayload(result)
	if payload == "" {
		return nil, nil
	}
	// The inbox MCP emits either an array or an envelope with a "threads" key.
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &arr); err == nil {
		return arr, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &obj); err != nil {
		return nil, fmt.Errorf("unexpected inbox payload: %w", err)
	}
	for _, k := range []string{"threads", "conversations", "data", "results"} {
		if raw, ok := obj[k]; ok {
			if err := json.Unmarshal(raw, &arr); err == nil {
				return arr, nil
			}
		}
	}
	return nil, nil
}
