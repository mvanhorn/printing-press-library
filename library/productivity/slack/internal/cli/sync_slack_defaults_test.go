// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
)

type recordingSyncClient struct {
	params map[string]string
}

func (c *recordingSyncClient) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	c.params = params
	return json.RawMessage(`{"ok":true,"channels":[],"response_metadata":{"next_cursor":""}}`), nil
}

func (*recordingSyncClient) RateLimit() float64 { return 0 }

func TestSlackSyncConversationsRequestTypes(t *testing.T) {
	tests := []struct {
		name          string
		flatFlags     []string
		resourceFlags []string
		want          string
	}{
		{"default channels only", nil, nil, "public_channel,private_channel"},
		{"explicit --param override", []string{"types=public_channel,im"}, nil, "public_channel,im"},
		{"explicit resource override", nil, []string{"conversations:types=mpim"}, "mpim"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params, err := parseSyncUserParams(tc.flatFlags, tc.resourceFlags, nil)
			if err != nil {
				t.Fatal(err)
			}
			db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			client := &recordingSyncClient{}
			result := syncResource(context.Background(), client, db, "conversations", "", true, 1, false, false, params, io.Discard)
			if result.Err != nil {
				t.Fatalf("sync conversations: %v", result.Err)
			}
			if got := client.params["types"]; got != tc.want {
				t.Fatalf("request types = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplySlackSyncDefaultsScopesConversations(t *testing.T) {
	params := map[string]string{}
	applySlackSyncDefaults("conversations", params)

	if got := params["types"]; got != "public_channel,private_channel" {
		t.Fatalf("types = %q, want public and private channels only", got)
	}
}

func TestApplySlackSyncDefaultsLeavesOtherResourcesUntouched(t *testing.T) {
	params := map[string]string{}
	applySlackSyncDefaults("users", params)

	if len(params) != 0 {
		t.Fatalf("params = %#v, want no Slack-specific defaults", params)
	}
}
