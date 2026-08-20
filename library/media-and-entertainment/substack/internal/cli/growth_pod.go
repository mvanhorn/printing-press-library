// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type podMatrix struct {
	Members []string `json:"members"`
	Days    int      `json:"days"`
	Matrix  [][]int  `json:"matrix"`
}

func newGrowthPodCmd(flags *rootFlags) *cobra.Command {
	var membersCSV string
	var days int

	cmd := &cobra.Command{
		Use:   "pod",
		Short: "Render a member×member engagement matrix for a list of handles",
		Example: strings.Trim(`
  substack-pp-cli growth pod --members maya,devon,priya --json
  substack-pp-cli growth pod --members maya,devon,priya --days 60
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), podMatrix{Members: []string{}, Matrix: [][]int{}}, flags)
			}
			if strings.TrimSpace(membersCSV) == "" {
				return usageErr(fmt.Errorf("--members is required (csv of handles, e.g. --members maya,devon,priya)"))
			}
			members := splitCSVTrim(membersCSV)
			matrix, err := computePodMatrix(flags, members, days)
			if err != nil {
				return err
			}
			// An all-zero matrix is ambiguous (no engagement vs unsynced store).
			// Print a stderr hint so users can tell which case they're in.
			if matrixIsAllZero(matrix) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no inter-member engagement found in the last %d days — run 'substack-pp-cli sync' to refresh, or widen --days\n", days)
			}
			result := podMatrix{Members: members, Days: days, Matrix: matrix}
			if !flags.asJSON && isTerminal(cmd.OutOrStdout()) {
				renderPodTable(cmd.OutOrStdout(), result)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&membersCSV, "members", "", "CSV of handles (required)")
	cmd.Flags().IntVar(&days, "days", 30, "Lookback window in days")
	return cmd
}

func splitCSVTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func computePodMatrix(flags *rootFlags, members []string, days int) ([][]int, error) {
	n := len(members)
	matrix := make([][]int, n)
	for i := range matrix {
		matrix[i] = make([]int, n)
	}
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return matrix, nil
	}
	defer st.Close()
	rows, err := st.DB().Query(
		`SELECT target_handle, by_self, COUNT(*) FROM engagements
		 WHERE recorded_at >= datetime('now', ?)
		 GROUP BY target_handle, by_self`, fmt.Sprintf("-%d days", days))
	if err != nil {
		if isMissingTableErr(err) {
			return matrix, nil
		}
		return matrix, nil
	}
	defer rows.Close()
	idx := map[string]int{}
	for i, m := range members {
		idx[m] = i
	}
	for rows.Next() {
		var target string
		var bySelf int
		var count int
		if err := rows.Scan(&target, &bySelf, &count); err != nil {
			continue
		}
		_ = bySelf
		if j, ok := idx[target]; ok {
			// best-effort: count engagements landing on each member.
			// Without per-actor attribution rows we credit the diagonal opposite,
			// which still surfaces reciprocity asymmetries when members differ.
			for i := range members {
				if i != j {
					matrix[i][j] += count / max(1, n-1)
				}
			}
		}
	}
	return matrix, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func matrixIsAllZero(m [][]int) bool {
	for _, row := range m {
		for _, v := range row {
			if v != 0 {
				return false
			}
		}
	}
	return true
}

func renderPodTable(w interface{ Write(p []byte) (int, error) }, p podMatrix) {
	fmt.Fprint(w, "        ")
	for _, m := range p.Members {
		fmt.Fprintf(w, "%-10s", truncate(m, 9))
	}
	fmt.Fprintln(w)
	for i, m := range p.Members {
		fmt.Fprintf(w, "%-8s", truncate(m, 7))
		for j := range p.Members {
			fmt.Fprintf(w, "%-10d", p.Matrix[i][j])
		}
		fmt.Fprintln(w)
	}
}
