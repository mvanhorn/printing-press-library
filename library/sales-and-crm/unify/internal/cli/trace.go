package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/store"

	"github.com/spf13/cobra"
)

// newTraceCmd walks reference attributes from a starting record through the
// local mirror, collecting attached records without making N+1 API calls.
func newTraceCmd(flags *rootFlags) *cobra.Command {
	var dbPath, object, id string
	var maxDepth int

	cmd := &cobra.Command{
		Use:   "trace <object> <record-id>",
		Short: "Walk reference attributes from a record through the local mirror",
		Long: `Loads the named record, then follows every reference attribute
(opportunities, people, record_owner, etc.) to list the attached records
inline. The trace is shallow by default (depth 1) — pass --depth 2 to
follow references-of-references.

References that point at records not yet in the local mirror are reported
as 'unresolved' so you know to add them to the watchlist and re-sync.`,
		Example: strings.Trim(`
  unify-pp-cli trace company 73fcb798-9ccd-4138-8f6a-9a801123783c --agent
  unify-pp-cli trace company 73fcb... --depth 2
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) >= 1 && object == "" {
				object = args[0]
			}
			if len(args) >= 2 && id == "" {
				id = args[1]
			}
			if object == "" || id == "" {
				return usageErr(fmt.Errorf("usage: trace <object> <record-id>"))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			if maxDepth < 1 {
				maxDepth = 1
			}
			visited := map[string]bool{}
			root, err := traceWalk(ctx, s, object, id, 0, maxDepth, visited)
			if err != nil {
				return apiErr(err)
			}
			blob, _ := json.MarshalIndent(root, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().IntVar(&maxDepth, "depth", 1, "How many reference hops to walk")
	return cmd
}

func traceWalk(ctx context.Context, s *store.Store, objectName, id string, depth, maxDepth int, visited map[string]bool) (map[string]any, error) {
	visitKey := objectName + "/" + id
	if visited[visitKey] {
		return map[string]any{"object": objectName, "id": id, "note": "cycle (already visited)"}, nil
	}
	visited[visitKey] = true

	attrs := fetchRecordAttrs(ctx, s, objectName, id)
	if attrs == nil {
		return map[string]any{
			"object":     objectName,
			"id":         id,
			"unresolved": true,
			"hint":       "not in local mirror — add to watchlist and run sync",
		}, nil
	}
	node := map[string]any{
		"object": objectName,
		"id":     id,
		"attrs":  attrs,
	}
	if depth >= maxDepth {
		return node, nil
	}

	refs := map[string]any{}
	for k, v := range attrs {
		if isReferenceLike(v) {
			refs[k] = walkRefValue(ctx, s, v, depth+1, maxDepth, visited)
		}
	}
	if len(refs) > 0 {
		node["references"] = refs
	}
	return node, nil
}

func isReferenceLike(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		_, hasID := t["id"]
		_, hasObject := t["object"]
		return hasID && hasObject
	case []any:
		if len(t) == 0 {
			return false
		}
		first, ok := t[0].(map[string]any)
		if !ok {
			return false
		}
		_, hasID := first["id"]
		_, hasObject := first["object"]
		return hasID && hasObject
	}
	return false
}

func walkRefValue(ctx context.Context, s *store.Store, v any, depth, maxDepth int, visited map[string]bool) any {
	switch t := v.(type) {
	case map[string]any:
		obj, _ := t["object"].(string)
		id, _ := t["id"].(string)
		if obj == "" || id == "" {
			return t
		}
		node, _ := traceWalk(ctx, s, obj, id, depth, maxDepth, visited)
		return node
	case []any:
		out := make([]any, 0, len(t))
		for _, it := range t {
			out = append(out, walkRefValue(ctx, s, it, depth, maxDepth, visited))
		}
		return out
	}
	return v
}
