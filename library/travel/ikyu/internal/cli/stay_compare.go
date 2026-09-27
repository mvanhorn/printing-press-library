// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

type comparisonPair struct {
	Left       string           `json:"left"`
	Right      string           `json:"right"`
	Assessment offerEquivalence `json:"assessment"`
}
type compatibleGroup struct {
	Selectors []string `json:"selectors"`
	Basis     string   `json:"basis"`
}

func newNovelStayCompareCmd(f *rootFlags) *cobra.Command {
	s := &stayFlags{}
	var offers string
	cmd := &cobra.Command{Use: "compare", Short: "Compare 2–5 exact selections on one stay; show alternatives and unknowns without ranking", Example: "  ikyu-pp-cli stay compare --offers 00002889:10193741:11055986,00002889:10193741:11055981 --check-in 2026-10-18 --check-out 2026-10-19 --adults 2 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--offers=00002889:10193741:11055986,00002889:10193741:11055981;--check-in=2026-10-18;--check-out=2026-10-19;--adults=2"}}
	bindStayFlags(cmd, s, true, false, false)
	cmd.Flags().StringVar(&offers, "offers", "", "Comma-separated property:room:plan selections (2–5 distinct offers)")
	addFieldsAlias(cmd, f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return stayDryRun(cmd, f, map[string]any{"operation": "exact offer comparison", "offers": offers, "stay": s.stay, "concurrency": 2, "maximum_selections": 5})
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("compare uses --offers, with no positional arguments"))
		}
		if e := stayValidate(s.stay); e != nil {
			return e
		}
		parts := strings.Split(offers, ",")
		if len(parts) < 2 || len(parts) > 5 {
			return usageErr(fmt.Errorf("--offers requires 2–5 distinct property:room:plan selections"))
		}
		jobs := []offerJob{}
		seen := map[string]bool{}
		for _, part := range parts {
			selection, e := parseSelection(part)
			if e != nil {
				return e
			}
			key := selection.String()
			if seen[key] {
				return usageErr(fmt.Errorf("duplicate offer selector %s; each selection must be distinct", key))
			}
			seen[key] = true
			jobs = append(jobs, offerJob{Key: key, Selection: selection, Stay: s.stay})
		}
		ctx, cancel := stayContext(cmd, f)
		defer cancel()
		c, e := stayClient(cmd, f, s)
		if e != nil {
			return e
		}
		rows, failures, results := fetchOfferRows(ctx, c, jobs)
		pairs := []comparisonPair{}
		equivalent := map[string]map[string]bool{}
		for i := 0; i < len(jobs); i++ {
			a, ok := results[jobs[i].Key]
			if !ok {
				continue
			}
			for j := i + 1; j < len(jobs); j++ {
				b, ok := results[jobs[j].Key]
				if !ok {
					continue
				}
				assessment := assessOffers(a, b)
				pairs = append(pairs, comparisonPair{Left: jobs[i].Key, Right: jobs[j].Key, Assessment: assessment})
				if assessment.Equivalent {
					if equivalent[jobs[i].Key] == nil {
						equivalent[jobs[i].Key] = map[string]bool{}
					}
					if equivalent[jobs[j].Key] == nil {
						equivalent[jobs[j].Key] = map[string]bool{}
					}
					equivalent[jobs[i].Key][jobs[j].Key] = true
					equivalent[jobs[j].Key][jobs[i].Key] = true
				}
			}
		}
		groups := []compatibleGroup{}
		used := map[string]bool{}
		for _, j := range jobs {
			if used[j.Key] {
				continue
			}
			members := []string{j.Key}
			for _, k := range jobs {
				if k.Key == j.Key || used[k.Key] {
					continue
				}
				fits := true
				for _, m := range members {
					fits = fits && equivalent[m][k.Key]
				}
				if fits {
					members = append(members, k.Key)
				}
			}
			if len(members) > 1 {
				for _, m := range members {
					used[m] = true
				}
				groups = append(groups, compatibleGroup{Selectors: members, Basis: "known compatible published source terms"})
			}
		}
		reportOfferFailures(cmd, failures, len(jobs))
		stats := c.Stats()
		result := map[string]any{"data": rows, "stay": s.stay, "comparisons": pairs, "compatible_groups": groups, "fetch_failures": failures, "partial": len(failures) > 0, "successful": len(results), "requested": len(jobs)}
		if e := stayEmit(cmd, f, result, &stats); e != nil {
			return e
		}
		if len(results) == 0 {
			return allOfferFailures(failures, len(jobs))
		}
		return nil
	}
	return cmd
}
