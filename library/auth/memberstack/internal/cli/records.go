// Hand-written novel command: shorthand for the Prisma-style records query.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newRecordsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "records",
		Short: "Shorthand wrappers around custom data record operations",
		Long: `'find' compiles --where / --order-by / --limit flags into the Prisma findMany
envelope that POST /v2/data-tables/:tableKey/records/query expects. Use this
when an agent wants ad-hoc filtering without constructing the JSON envelope by
hand. For complex queries, use 'data-tables records query' with a JSON body.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRecordsFindCmd(flags))
	return cmd
}

func newRecordsFindCmd(flags *rootFlags) *cobra.Command {
	var where string
	var orderBy string
	var take int
	var skip int

	cmd := &cobra.Command{
		Use:   "find [table-key]",
		Short: "Query records using shorthand flags; compiles to the Prisma findMany payload.",
		Long: `Builds and sends a Prisma-style findMany request against a custom data table.
--where takes comma-separated field=value or field>=value / field<=value /
field LIKE 'pattern' clauses. --order-by takes 'field:asc' or 'field:desc'.

Example payload built from --where 'inStock=true,price>=20' --order-by price:asc --limit 10:
  {"query":{"findMany":{"where":{"inStock":true,"price":{"gte":20}},"orderBy":{"price":"asc"},"take":10}}}`,
		Example: `  memberstack-pp-cli records find products --where 'inStock=true' --order-by price:asc --limit 10
  memberstack-pp-cli records find products --where 'category=widgets' --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && isTerminal(cmd.OutOrStdout()) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if len(args) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "would POST /v2/data-tables/%s/records/query (findMany)\n", args[0])
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "would query records (findMany)")
				}
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("table key is required"))
			}
			table := args[0]

			whereMap, err := parseWhereClause(where)
			if err != nil {
				return usageErr(fmt.Errorf("parsing --where: %w", err))
			}
			orderObj, err := parseOrderBy(orderBy)
			if err != nil {
				return usageErr(fmt.Errorf("parsing --order-by: %w", err))
			}

			findMany := map[string]any{}
			if len(whereMap) > 0 {
				findMany["where"] = whereMap
			}
			if orderObj != nil {
				findMany["orderBy"] = orderObj
			}
			if take > 0 {
				findMany["take"] = take
			}
			if skip > 0 {
				findMany["skip"] = skip
			}
			payload := map[string]any{"query": map[string]any{"findMany": findMany}}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/v2/data-tables/" + table + "/records/query"
			data, _, err := c.Post(cmd.Context(), path, payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&where, "where", "", "Comma-separated predicates (field=value, field>=N, field LIKE 'pat'); compiles to a Prisma where object")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "Sort spec: 'field:asc' or 'field:desc'")
	cmd.Flags().IntVar(&take, "limit", 0, "Max records to return (Prisma take)")
	cmd.Flags().IntVar(&skip, "skip", 0, "Offset into the result set (Prisma skip)")
	return cmd
}

// parseWhereClause converts "name=foo,price>=20,inStock=true" into
// {"name":"foo","price":{"gte":20},"inStock":true}.
func parseWhereClause(s string) (map[string]any, error) {
	out := map[string]any{}
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, raw := range splitTopLevelComma(s) {
		clause := strings.TrimSpace(raw)
		if clause == "" {
			continue
		}
		op, key, value, err := splitPredicate(clause)
		if err != nil {
			return nil, err
		}
		v := parseValue(value)
		switch op {
		case "=":
			out[key] = v
		case "!=":
			out[key] = map[string]any{"not": v}
		case ">=":
			out[key] = map[string]any{"gte": v}
		case "<=":
			out[key] = map[string]any{"lte": v}
		case ">":
			out[key] = map[string]any{"gt": v}
		case "<":
			out[key] = map[string]any{"lt": v}
		case "LIKE":
			out[key] = map[string]any{"contains": strings.Trim(value, "'\"%")}
		}
	}
	return out, nil
}

func splitTopLevelComma(s string) []string {
	out := []string{}
	depth := 0
	last := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[last:i])
				last = i + 1
			}
		}
	}
	out = append(out, s[last:])
	return out
}

func splitPredicate(c string) (op, key, val string, err error) {
	// Order matters: longest tokens first.
	for _, t := range []string{"!=", ">=", "<=", " LIKE ", "=", ">", "<"} {
		if i := strings.Index(c, t); i > 0 {
			op = strings.TrimSpace(t)
			key = strings.TrimSpace(c[:i])
			val = strings.TrimSpace(c[i+len(t):])
			return
		}
	}
	return "", "", "", fmt.Errorf("could not parse predicate %q (expected key=val / >= / <= / > / < / LIKE)", c)
}

func parseValue(v string) any {
	v = strings.TrimSpace(v)
	if v == "true" {
		return true
	}
	if v == "false" {
		return false
	}
	if v == "null" {
		return nil
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return strings.Trim(v, "'\"")
}

func parseOrderBy(s string) (map[string]any, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected 'field:asc' or 'field:desc', got %q", s)
	}
	dir := strings.ToLower(parts[1])
	if dir != "asc" && dir != "desc" {
		return nil, fmt.Errorf("direction must be asc or desc, got %q", dir)
	}
	return map[string]any{parts[0]: dir}, nil
}

// Anchor for json import (kept across edits).
var _ = json.RawMessage(nil)
