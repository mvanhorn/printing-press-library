// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

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
