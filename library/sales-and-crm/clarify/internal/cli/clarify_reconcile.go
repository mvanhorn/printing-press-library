// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored reconcile wiring for the Clarify mirror. The generated
// profiler leaves flatReconcileModes empty because Clarify's payloads carry no
// tenant column — but the `resources` sync target is naturally partitioned by
// the JSON:API `type` field, and one sync run enumerates exactly one object
// type ({object} path placeholder). Registering that partition lets a
// complete full sync prune upstream-deleted records from the mirror, so
// stale/brief/velocity stop reporting rows that no longer exist in Clarify.

package cli

import (
	"os"
	"strings"
)

func init() {
	// The `resources` rows are partitioned by json_extract(data, '$.type');
	// the active partition value is the object type this sync run enumerates.
	flatReconcileModes["resources"] = "flat"
	flatReconcileDefs["resources"] = flatReconcileDefT{BodyField: "type"}
	resolveTenantID = clarifyResolveObjectScope
}

// clarifyResolveObjectScope returns the object type the current sync run is
// scoped to, or "" when unknown (reconcile then SKIPS — it never deletes on an
// unresolved scope). Mirrors the sync merge order: an explicit
// --path-context object=<type> wins over the CLARIFY_OBJECT env default.
func clarifyResolveObjectScope() string {
	args := os.Args
	for i, a := range args {
		if a == "--path-context" && i+1 < len(args) {
			if v, ok := strings.CutPrefix(args[i+1], "object="); ok {
				return strings.TrimSpace(v)
			}
		}
		if v, ok := strings.CutPrefix(a, "--path-context=object="); ok {
			return strings.TrimSpace(v)
		}
	}
	return strings.TrimSpace(os.Getenv("CLARIFY_OBJECT"))
}
