// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import (
	"strings"
	"testing"
)

// TestSQLToolParamDescribesRealSchema pins the sql tool's query-parameter
// description to the store's ACTUAL schema. The store keeps every synced record
// in one `resources` table keyed by `resource_type` with the raw record in a
// JSON `data` column — there are no per-resource tables. The prior description
// ("Tables match resource names.") sent agent hosts down a dead end: they tried
// `FROM groups` / `FROM get_groups` and got "no such table". The description
// must instead point at `resources`, `resource_type`, and `json_extract`.
func TestSQLToolParamDescribesRealSchema(t *testing.T) {
	desc := sqlQueryParamDesc

	for _, must := range []string{"resources", "resource_type", "json_extract"} {
		if !strings.Contains(desc, must) {
			t.Errorf("sql query param description must mention %q so hosts query the real schema; got: %q", must, desc)
		}
	}

	if strings.Contains(desc, "Tables match resource names") {
		t.Errorf("sql query param description still contains the misleading 'Tables match resource names' claim: %q", desc)
	}
}
