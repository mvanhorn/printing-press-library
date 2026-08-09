// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestNotificationsReadFromUserEnvelope(t *testing.T) {
	if got, err := resourceReadPath("notifications"); err != nil || got != HabiticaNotificationsPath {
		t.Fatalf("resourceReadPath(notifications) = %q, %v; want %q", got, err, HabiticaNotificationsPath)
	}
	if got, err := syncResourcePath("notifications"); err != nil || got != HabiticaNotificationsPath {
		t.Fatalf("syncResourcePath(notifications) = %q, %v; want %q", got, err, HabiticaNotificationsPath)
	}
	paths := responsePathForResource("notifications", HabiticaNotificationsPath)
	if len(paths) != 1 || paths[0] != HabiticaNotificationsResponsePath {
		t.Fatalf("notification response paths = %v, want [%q]", paths, HabiticaNotificationsResponsePath)
	}

	body := json.RawMessage(`{"success":true,"data":{"notifications":[{"id":"notice-1"}]}}`)
	payload, ok := ResponsePayloadAtPath(body, HabiticaNotificationsResponsePath)
	if !ok {
		t.Fatal("notification payload was not found in the /user response")
	}
	var notifications []map[string]any
	if err := json.Unmarshal(payload, &notifications); err != nil {
		t.Fatalf("decode notifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0]["id"] != "notice-1" {
		t.Fatalf("notifications = %#v", notifications)
	}
}
