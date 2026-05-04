package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cliutil"
)

// collaboratorRow is one entry in the frequent-collaborator list.
type collaboratorRow struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Count  int      `json:"count"`
	Titles []string `json:"titles"`
	Roles  []string `json:"roles"` // unique department/role tags observed across the joint titles
}

type collaboratorsOutput struct {
	Person        careerPerson      `json:"person"`
	Collaborators []collaboratorRow `json:"collaborators"`
}

const collabCreditsCap = 50

func newCollaboratorsCmd(flags *rootFlags) *cobra.Command {
	var flagMinCount int
	var flagRole string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "collaborators <person>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "List people who appear in 2+ of the target person's credits",
		Long: `Resolve a person, walk combined_credits, fan-out /<kind>/{id}/credits per
credit (capped at 50 to respect TMDb rate limits), and tally co-occurring
people. Filter by role (actor or crew) and minimum overlap count.

Useful for finding directors-of-choice cinematographers, actor-director
recurring duos, or composers who shadow a particular director.`,
		Example: `  movie-goat-pp-cli collaborators "Christopher Nolan"
  movie-goat-pp-cli collaborators 525 --role crew --min-count 3
  movie-goat-pp-cli collaborators "Greta Gerwig" --limit 10 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			minCount := flagMinCount
			if minCount <= 0 {
				minCount = 2
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 30
			}
			role := strings.ToLower(strings.TrimSpace(flagRole))
			if role != "" && role != "actor" && role != "crew" {
				return usageErr(fmt.Errorf("--role must be \"actor\" or \"crew\", got %q", flagRole))
			}

			// Resolve target person.
			query := strings.Join(args, " ")
			var personID int
			var personName string
			if id, perr := strconv.Atoi(query); perr == nil {
				personID = id
			} else {
				p, perr := searchPersonByName(c, query)
				if perr != nil {
					return classifyAPIError(perr)
				}
				personID = p.ID
				personName = p.DisplayTitle()
			}

			// Pull combined credits to enumerate the target's titles.
			data, err := c.Get(fmt.Sprintf("/person/%d", personID),
				map[string]string{"append_to_response": "combined_credits"})
			if err != nil {
				return classifyAPIError(err)
			}
			var person tmdbPersonDetail
			if err := json.Unmarshal(data, &person); err != nil {
				return fmt.Errorf("parsing person: %w", err)
			}
			if personName == "" {
				personName = person.Name
			}

			type creditRef struct {
				ID    int
				Kind  string
				Title string
			}
			seen := map[string]bool{}
			var refs []creditRef
			collect := func(id int, kind, title string) {
				if id == 0 || kind == "" {
					return
				}
				key := fmt.Sprintf("%s-%d", kind, id)
				if seen[key] {
					return
				}
				seen[key] = true
				refs = append(refs, creditRef{ID: id, Kind: kind, Title: title})
			}
			if person.CombinedCredits != nil {
				for _, e := range person.CombinedCredits.Cast {
					collect(e.ID, e.MediaType, e.DisplayTitle())
				}
				for _, e := range person.CombinedCredits.Crew {
					collect(e.ID, e.MediaType, e.DisplayTitle())
				}
			}
			if len(refs) == 0 {
				return notFoundErr(fmt.Errorf("no credits found for person %d", personID))
			}
			if len(refs) > collabCreditsCap {
				fmt.Fprintf(cmd.ErrOrStderr(), "warn: walking only first %d of %d credits to respect TMDb rate limits\n", collabCreditsCap, len(refs))
				refs = refs[:collabCreditsCap]
			}

			// Fan-out per-credit /credits.
			ctx, cancel := context.WithTimeout(cmd.Context(), 120*time.Second)
			defer cancel()
			type personRow struct {
				ID         int    `json:"id"`
				Name       string `json:"name"`
				Department string `json:"department"`
				Job        string `json:"job"`
				Character  string `json:"character"`
			}
			type creditsPayload struct {
				Cast []personRow `json:"cast"`
				Crew []personRow `json:"crew"`
			}
			type harvest struct {
				ref  creditRef
				cast []personRow
				crew []personRow
			}
			results, errs := cliutil.FanoutRun(ctx, refs,
				func(r creditRef) string { return r.Title },
				func(_ context.Context, r creditRef) (harvest, error) {
					path := fmt.Sprintf("/%s/%d/credits", r.Kind, r.ID)
					data, gerr := c.Get(path, map[string]string{})
					if gerr != nil {
						return harvest{ref: r}, gerr
					}
					var p creditsPayload
					if jerr := json.Unmarshal(data, &p); jerr != nil {
						return harvest{ref: r}, jerr
					}
					return harvest{ref: r, cast: p.Cast, crew: p.Crew}, nil
				})
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			// Aggregate.
			type accum struct {
				name   string
				titles map[string]struct{}
				roles  map[string]struct{}
			}
			tally := map[int]*accum{}
			bump := func(p personRow, ref creditRef, source string) {
				if p.ID == 0 || p.ID == personID {
					return
				}
				a, ok := tally[p.ID]
				if !ok {
					a = &accum{name: p.Name, titles: map[string]struct{}{}, roles: map[string]struct{}{}}
					tally[p.ID] = a
				}
				a.titles[ref.Title] = struct{}{}
				if source == "cast" {
					a.roles["actor"] = struct{}{}
				} else if p.Department != "" {
					a.roles[strings.ToLower(p.Department)] = struct{}{}
				}
			}
			for _, h := range results {
				if role == "" || role == "actor" {
					for _, p := range h.Value.cast {
						bump(p, h.Value.ref, "cast")
					}
				}
				if role == "" || role == "crew" {
					for _, p := range h.Value.crew {
						bump(p, h.Value.ref, "crew")
					}
				}
			}

			// Build output rows, applying min-count filter.
			// Count is the number of unique titles the collaborator shares with the
			// target person — not raw credit rows. A single film can carry multiple
			// crew rows for the same person (e.g., editor + producer), and the
			// previous implementation incremented count per row while titles was
			// deduped by title, leaving count and len(titles) inconsistent in the
			// output.
			rows := make([]collaboratorRow, 0, len(tally))
			for id, a := range tally {
				if len(a.titles) < minCount {
					continue
				}
				row := collaboratorRow{ID: id, Name: a.name, Count: len(a.titles)}
				for t := range a.titles {
					row.Titles = append(row.Titles, t)
				}
				for r := range a.roles {
					row.Roles = append(row.Roles, r)
				}
				sort.Strings(row.Titles)
				sort.Strings(row.Roles)
				rows = append(rows, row)
			}

			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Count != rows[j].Count {
					return rows[i].Count > rows[j].Count
				}
				return rows[i].Name < rows[j].Name
			})
			if len(rows) > limit {
				rows = rows[:limit]
			}

			out := collaboratorsOutput{
				Person:        careerPerson{ID: personID, Name: personName, Birthday: person.Birthday, KnownFor: person.KnownFor},
				Collaborators: rows,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Frequent collaborators of %s\n", personName)
			fmt.Fprintln(w, strings.Repeat("=", len(personName)+27))
			if len(rows) == 0 {
				fmt.Fprintln(w, "No collaborators meet the threshold. Try lowering --min-count.")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "Count\tName\tRoles\tTitles")
			for _, r := range rows {
				titles := strings.Join(r.Titles, "; ")
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n",
					r.Count, truncate(r.Name, 30), strings.Join(r.Roles, ","), truncate(titles, 60))
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&flagMinCount, "min-count", 2, "Minimum joint credits to include")
	cmd.Flags().StringVar(&flagRole, "role", "", "Filter co-credits to actor or crew (default: both)")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Top N collaborators to return")
	return cmd
}
