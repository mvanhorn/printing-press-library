package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newSQLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a raw SQL query against the local mirror (read-only)",
		Args:  cobra.ExactArgs(1),
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			q := args[0]
			if err := guardReadOnlySQL(q); err != nil {
				return err
			}
			return runReadOnlySQL(cx, q, mode())
		}),
	}
	return cmd
}

// bannedSQLKeyword matches any write-side SQL keyword as a standalone token.
// Word boundaries prevent false positives on column or alias names that
// contain these substrings (e.g. "created_at"). The regex catches keywords
// anywhere in the statement -- including buried inside a CTE such as
// `WITH x AS (SELECT 1) DELETE FROM protocols`, which a prefix/semicolon
// check would let through.
var bannedSQLKeyword = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|create|attach|detach|pragma|replace|truncate|vacuum|reindex)\b`)

func guardReadOnlySQL(q string) error {
	if bannedSQLKeyword.MatchString(q) {
		return fmt.Errorf("only SELECT statements are allowed (use writer commands for syncing)")
	}
	return nil
}

// runReadOnlySQL executes a SELECT and renders results via the typed Recorder
// so JSON/CSV emit raw numbers and table output formats large values as $K/M/B.
func runReadOnlySQL(cx *Ctx, q string, m format.Mode) error {
	rows, err := cx.S.Query(q)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = strings.ToUpper(c)
	}

	// Two-pass: collect raw values, infer per-column types, then render.
	type rawRow []any
	var raws []rawRow
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		raws = append(raws, vals)
	}

	// Per-column type inference: numeric if every non-nil value is numeric.
	colNumeric := make([]bool, len(cols))
	colMonetary := make([]bool, len(cols))
	for i, name := range cols {
		colNumeric[i] = true
		anyVal := false
		anyBig := false
		for _, r := range raws {
			v := r[i]
			if v == nil {
				continue
			}
			anyVal = true
			if !isNumericScan(v) {
				colNumeric[i] = false
				break
			}
			if f, ok := numericAsFloat(v); ok && f >= 1000 {
				anyBig = true
			}
		}
		if !anyVal {
			colNumeric[i] = false
		}
		if colNumeric[i] && anyBig && isMonetaryColumn(name) {
			colMonetary[i] = true
		}
	}

	rec := format.NewRecorder(headers)
	right := []int{}
	for i, n := range colNumeric {
		if n {
			right = append(right, i)
		}
	}
	for _, r := range raws {
		cells := make([]format.Cell, len(cols))
		for i, v := range r {
			if v == nil {
				cells[i] = format.Str("")
				continue
			}
			if colNumeric[i] {
				f, _ := numericAsFloat(v)
				if colMonetary[i] {
					cells[i] = format.USD(f)
				} else {
					cells[i] = format.Raw(f)
				}
			} else {
				cells[i] = format.Str(stringify(v))
			}
		}
		rec.Append(cells)
	}
	return format.RenderRec(os.Stdout, m, rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet(right)})
}

// isMonetaryColumn returns true if the column name suggests a USD amount.
// Used to decide whether $K/M/B formatting is appropriate in SQL table mode.
func isMonetaryColumn(name string) bool {
	n := strings.ToLower(name)
	for _, sub := range []string{"tvl", "mcap", "fees", "rev", "volume", "circulating", "price", "amount", "aum", "oi", "flow", "treasury", "value"} {
		if strings.Contains(n, sub) {
			return true
		}
	}
	return false
}

func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	case string:
		return x
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		return format.USDString(x) // shouldn't be hit; numeric path handles floats
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", x)
	}
}

func isNumericScan(v any) bool {
	switch v.(type) {
	case int64, float64:
		return true
	case []byte:
		// SQLite REAL/INTEGER often comes back as numeric directly with modernc;
		// strings remain []byte. Try parsing.
		b, ok := v.([]byte)
		if !ok {
			return false
		}
		_, err := parseFloatLoose(string(b))
		return err == nil
	case string:
		_, err := parseFloatLoose(v.(string))
		return err == nil
	}
	return false
}

func numericAsFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case []byte:
		f, err := parseFloatLoose(string(x))
		return f, err == nil
	case string:
		f, err := parseFloatLoose(x)
		return f, err == nil
	}
	return 0, false
}

// parseFloatLoose accepts the same shape as strconv.ParseFloat but rejects
// strings that obviously aren't numeric so the type-inference pass in `sql`
// can decide whether to right-align a column. Signs are allowed at position 0
// or immediately after an `e`/`E` (so "-1.5e-9" parses correctly).
func parseFloatLoose(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	prevIsExp := false
	for i, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c == '.':
		case (c == '-' || c == '+') && (i == 0 || prevIsExp):
		case c == 'e' || c == 'E':
		default:
			return 0, fmt.Errorf("not numeric")
		}
		prevIsExp = c == 'e' || c == 'E'
	}
	var f float64
	_, err := fmt.Sscanf(s, "%g", &f)
	return f, err
}
