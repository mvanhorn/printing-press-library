// Copyright 2026 joltsconsulting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/adminbyrequest/internal/store"
	"github.com/spf13/cobra"
)

type deniedToken struct {
	Token string `json:"token"`
	Count int    `json:"count"`
}

var deniedReasonsStopwords = map[string]struct{}{
	"the": {}, "and": {}, "or": {}, "but": {}, "to": {}, "of": {}, "for": {}, "in": {}, "on": {},
	"a": {}, "an": {}, "is": {}, "it": {}, "this": {}, "that": {}, "be": {}, "by": {}, "with": {},
	"as": {}, "at": {}, "from": {}, "if": {}, "was": {}, "are": {}, "we": {}, "you": {}, "your": {},
	"i": {}, "my": {}, "me": {}, "no": {}, "not": {}, "do": {}, "did": {}, "so": {}, "too": {},
}

var deniedReasonsTokenRE = regexp.MustCompile(`[a-zA-Z][a-zA-Z'\-]+`)

func newRequestsDeniedReasonsCmd(flags *rootFlags) *cobra.Command {
	var topN int
	var minLen int
	var dbPath string

	cmd := &cobra.Command{
		Use:     "denied-reasons",
		Short:   "Top-N word distribution of free-text denial reasons (offline)",
		Long:    "Tokenize the deniedReason field across all locally-synced denied requests, drop common stopwords, return the most frequent tokens.",
		Example: "  adminbyrequest-pp-cli requests denied-reasons --top 20 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("adminbyrequest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store at %s: %w (run sync first)", dbPath, err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT denied_reason FROM requests
				 WHERE denied_reason IS NOT NULL AND TRIM(denied_reason) != ''`)
			if err != nil {
				return fmt.Errorf("querying denied reasons: %w", err)
			}
			defer rows.Close()

			freq := map[string]int{}
			samples := 0
			for rows.Next() {
				var reason string
				if err := rows.Scan(&reason); err != nil {
					return err
				}
				samples++
				for _, tok := range deniedReasonsTokenRE.FindAllString(reason, -1) {
					t := strings.ToLower(tok)
					if len(t) < minLen {
						continue
					}
					if _, skip := deniedReasonsStopwords[t]; skip {
						continue
					}
					freq[t]++
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating denied reasons: %w", err)
			}

			out := make([]deniedToken, 0, len(freq))
			for t, c := range freq {
				out = append(out, deniedToken{Token: t, Count: c})
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Count != out[j].Count {
					return out[i].Count > out[j].Count
				}
				return out[i].Token < out[j].Token
			})
			if len(out) > topN {
				out = out[:topN]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Top %d tokens across %d denied requests\n", len(out), samples)
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %d\n", r.Token, r.Count)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&topN, "top", 20, "Return this many top tokens")
	cmd.Flags().IntVar(&minLen, "min-length", 3, "Minimum token length to include")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard CLI location)")
	return cmd
}
