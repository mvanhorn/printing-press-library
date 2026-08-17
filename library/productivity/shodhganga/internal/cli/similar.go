// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: find theses sharing the most subject keywords with a given one.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/dspace"
)

type similarHit struct {
	Handle        string   `json:"handle"`
	Title         string   `json:"title"`
	Researcher    string   `json:"researcher"`
	University    string   `json:"university,omitempty"`
	SharedCount   int      `json:"shared_keywords"`
	Shared        []string `json:"shared"`
	CompletedDate string   `json:"completed_date,omitempty"`
}

type similarResult struct {
	Handle string       `json:"handle"`
	Title  string       `json:"title"`
	Count  int          `json:"count"`
	Hits   []similarHit `json:"hits"`
}

// pp:data-source auto
func newNovelSimilarCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "similar <handle>",
		Short: "Find theses in the local store that share the most subject keywords with a given thesis.",
		Long: "Rank harvested theses by how many subject keywords they share with <handle>.\n" +
			"The target thesis is read from the store, or fetched live if not yet harvested.\n" +
			"Populate the store first with: shodhganga-pp-cli harvest <query>.",
		Example:     "  shodhganga-pp-cli similar 10603/305247 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "handle=10603/305247"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a thesis handle is required"))
			}
			id, err := dspace.NormalizeID(args[0])
			if err != nil {
				return usageErr(err)
			}
			if limit <= 0 {
				limit = 10
			}

			s, ok, err := openThesisStoreRead(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer s.Close()

			theses, err := loadTheses(s)
			if err != nil {
				return err
			}

			// Find the target in the store; if absent, fetch it live.
			var target *dspace.Thesis
			for i := range theses {
				if theses[i].ID == id {
					target = &theses[i]
					break
				}
			}
			if target == nil {
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				c, cerr := newDSpaceClient(flags)
				if cerr != nil {
					return cerr
				}
				target, err = c.Item(ctx, id)
				if err != nil {
					if err == dspace.ErrNotFound {
						return notFoundErr(fmt.Errorf("thesis %s not found", id))
					}
					return classifyAPIError(err, flags)
				}
			}

			targetKW := keywordSet(target.Keywords)
			res := similarResult{Handle: target.Handle, Title: target.Title, Hits: []similarHit{}}
			for _, t := range theses {
				if t.ID == id {
					continue
				}
				shared := sharedKeywords(targetKW, t.Keywords)
				if len(shared) == 0 {
					continue
				}
				res.Hits = append(res.Hits, similarHit{
					Handle:        t.Handle,
					Title:         t.Title,
					Researcher:    t.Researcher,
					University:    t.University,
					SharedCount:   len(shared),
					Shared:        shared,
					CompletedDate: t.CompletedDate,
				})
			}
			sort.SliceStable(res.Hits, func(i, j int) bool {
				return res.Hits[i].SharedCount > res.Hits[j].SharedCount
			})
			if len(res.Hits) > limit {
				res.Hits = res.Hits[:limit]
			}
			res.Count = len(res.Hits)

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if res.Count == 0 {
				fmt.Fprintf(cmd.OutOrStdout(),
					"No theses in the local store share keywords with %s. Harvest more with: shodhganga-pp-cli harvest <query>\n", target.Handle)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Theses similar to %q [%s]:\n", target.Title, target.Handle)
			for _, h := range res.Hits {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s (%d shared: %s)\n",
					h.Handle, h.Title, h.SharedCount, strings.Join(h.Shared, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path (default: standard cache location)")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum similar theses to return")
	return cmd
}

func keywordSet(kw []string) map[string]string {
	m := make(map[string]string, len(kw))
	for _, k := range kw {
		m[strings.ToLower(strings.TrimSpace(k))] = k
	}
	return m
}

func sharedKeywords(target map[string]string, other []string) []string {
	var shared []string
	seen := map[string]bool{}
	for _, k := range other {
		lk := strings.ToLower(strings.TrimSpace(k))
		if orig, ok := target[lk]; ok && !seen[lk] {
			seen[lk] = true
			shared = append(shared, orig)
		}
	}
	return shared
}
