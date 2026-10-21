// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

// A bare sync must stay bounded to the user's own boards and Pins (dependents
// follow only those parents), never the whole parameterized API surface.
func TestDefaultSyncResourcesStayBounded(t *testing.T) {
	if got, want := defaultSyncResources(), []string{"boards", "pins"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("defaultSyncResources() = %v, want %v", got, want)
	}
}
