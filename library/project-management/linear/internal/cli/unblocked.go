package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/groups"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// `unblocked`: which blocked issues are now actionable.
//
// One paged query, filtered client-side. IssueFilter.hasBlockedByRelations
// looks like the server-side shortcut for "is this blocked" and is a trap:
// Linear stops matching it once every blocker of an issue has closed, which
// is exactly the state this command exists to report. Filtering the crawl on
// it made the command structurally incapable of returning a row. Proven live
// on 2026-08-11: completing blocker ACC-256 made blocked ACC-257 disappear
// from the candidate set entirely, even under --show-blocked, while `relations
// list` still showed the relation and `issues get` still showed ACC-257 open.
//
// So the query asks only for the open group (plus --team) and selects each
// issue's inverseRelations inline:
//
//	issues(filter: {state: <open group>, team: ...}) {
//	  nodes { ... inverseRelations(first: $relFirst) { nodes { type issue { state } } } }
//	}
//
// "Blocked by something" is then a local test: keep an issue only when at
// least one inverse relation carries type `blocks`. The blockers' own states
// ride along in the same selection set, so the verdict costs no extra round
// trip. When an issue's inline relation page is truncated the rest is paged
// before any verdict, because a partial blocker set yields a confident wrong
// answer.
//
// Closed is defined as the complement of the open group, not as its own list.
// An issue counts as closed when the resolved open group does NOT match its
// state. Under the shipped default `active` group (types triage, backlog,
// unstarted, started) the complement is exactly completed, canceled and
// duplicate. A workspace that redefines `active` in groups.toml moves both
// sides of the test together, which is the point: there is no second,
// hand-written "closed" predicate that could drift out of agreement.
//
// Scope: this answers which BLOCKED items became actionable. An open issue
// with no blockers at all was never blocked, so it is dropped by the local
// `blocks` test. Without a server-side relation predicate the query walks
// every open issue in scope, so --team is the way to keep the crawl small.
//
// Live only. Relations are not part of `sync`, so meta.source is always
// "live" and there is no local fallback to fall back to.

// unblockedBlocker is one blocker of a candidate issue.
type unblockedBlocker struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title,omitempty"`
	StateName  string `json:"state_name"`
	StateType  string `json:"state_type"`
	Closed     bool   `json:"closed"`
}

// unblockedRow is one issue whose blockers are all closed.
type unblockedRow struct {
	Identifier        string             `json:"identifier"`
	ID                string             `json:"id"`
	Title             string             `json:"title"`
	URL               string             `json:"url,omitempty"`
	Priority          string             `json:"priority"`
	StateName         string             `json:"state_name"`
	StateType         string             `json:"state_type"`
	Team              string             `json:"team,omitempty"`
	Blockers          []unblockedBlocker `json:"blockers"`
	AllBlockersClosed bool               `json:"all_blockers_closed"`
}

func newUnblockedCmd(flags *rootFlags) *cobra.Command {
	var teamFlag, stateFlag string
	var limit, relLimit int
	var includeStillBlocked bool
	cmd := &cobra.Command{
		Use:   "unblocked",
		Short: "Show blocked issues whose blockers are now all closed",
		Long: `Answer "what became actionable?".

Lists open issues that HAVE blocking relations and whose every blocker sits in
a closed state. An issue with no blockers is not a result: it was never
blocked, so it never became unblocked.

One query, filtered locally. IssueFilter's hasBlockedByRelations comparator
looks like the server-side way to ask "is this blocked" and cannot be used:
Linear stops matching it once every blocker of an issue closes, so filtering
on it hid precisely the issues this command exists to surface. Instead the
query asks for open issues and selects each one's inverseRelations inline,
then keeps the issues carrying at least one incoming 'blocks' relation and
tests every blocker's state locally.

Without a server-side relation filter the crawl covers every open issue in
scope, so pass --team to keep it small. --relation-limit sets how many blocker
relations come back inline per issue, and anything past that is paged before
the verdict rather than guessed.

Closed is the complement of the open group, never a second hand-written list.
An issue is closed when the group passed to --state does not match it. With
the default 'active' group that is exactly the completed, canceled and
duplicate state types. Redefine 'active' in groups.toml and both sides of the
test move together.

Relations are not synced, so this command is live only.`,
		Example: `  linear-pp-cli unblocked
  linear-pp-cli unblocked --team ESP --agent
  linear-pp-cli unblocked --state active --show-blocked --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be positive"))
			}
			if relLimit <= 0 {
				return usageErr(fmt.Errorf("--relation-limit must be positive"))
			}
			return runUnblocked(cmd, flags, teamFlag, stateFlag, limit, relLimit, includeStillBlocked)
		},
	}
	cmd.Flags().StringVar(&teamFlag, "team", "", "Filter to one team key or team UUID")
	cmd.Flags().StringVar(&stateFlag, "state", "active", stateGroupFlagUsage)
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum candidate issues to examine per page")
	cmd.Flags().IntVar(&relLimit, "relation-limit", 50, "Blocker relations to pull inline per candidate before paging the rest")
	cmd.Flags().BoolVar(&includeStillBlocked, "show-blocked", false, "Also emit candidates that still have at least one open blocker (all_blockers_closed=false)")
	return cmd
}

func runUnblocked(cmd *cobra.Command, flags *rootFlags, teamFlag, stateToken string, limit, relLimit int, includeStillBlocked bool) error {
	if flags.dataSource == "local" {
		return usageErr(fmt.Errorf("unblocked is live only: issue relations are not part of 'sync', so there is no local snapshot to compute blocker states from. Drop --data-source local"))
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}

	// One resolved predicate drives both sides of the answer: the server-side
	// state filter on the crawl and the client-side blocker test. Closed is
	// its complement.
	set, err := resolveStateSet(flags, teamKeyForGroups(nil, teamFlag), stateToken)
	if err != nil {
		return err
	}

	// The crawl asks for the open group and nothing about relations.
	// hasBlockedByRelations is the only relation predicate the server offers
	// and it stops matching an issue once all of its blockers close, so it
	// cannot appear here: it would filter out every row worth reporting.
	filter := map[string]any{}
	if stateFilter := groups.LiveFilter(set); stateFilter != nil {
		filter["state"] = stateFilter
	}
	if teamFlag != "" {
		filter["team"] = unblockedTeamFilter(teamFlag)
	}

	candidates, err := fetchUnblockedCandidates(c, filter, limit, relLimit)
	if err != nil {
		return classifyLiveReadError(err, flags)
	}

	// Local pass: a crawled issue is a candidate only if something blocks it,
	// and then every blocker's state is resolved against the same predicate.
	// Not matched by the open group == closed.
	rows := make([]unblockedRow, 0, len(candidates))
	for _, cand := range candidates {
		inverse := cand.InverseRelations
		if cand.RelationsTruncated {
			// The inline page was capped. Computing "all blockers closed"
			// from a partial blocker set would produce a confident wrong
			// answer, so page the rest before deciding.
			full, err := c.FetchIssueInverseRelations(cand.ID, relLimit)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			inverse = full
		}
		blockers := make([]unblockedBlocker, 0, len(inverse))
		allClosed := true
		for _, rel := range inverse {
			if rel.Type != "blocks" {
				continue
			}
			blocker := rel.Issue
			closed := !set.Match(blocker.State.Type, blocker.State.Name)
			if !closed {
				allClosed = false
			}
			blockers = append(blockers, unblockedBlocker{
				Identifier: blocker.Identifier,
				Title:      blocker.Title,
				StateName:  blocker.State.Name,
				StateType:  blocker.State.Type,
				Closed:     closed,
			})
		}
		if len(blockers) == 0 {
			// Nothing blocks this issue: either it carries no inverse
			// relations at all, or the ones it carries are `related`,
			// `similar` or `duplicate`. It was never blocked, so it never
			// became unblocked. This local test is what replaces the
			// unusable hasBlockedByRelations server filter.
			continue
		}
		if !allClosed && !includeStillBlocked {
			continue
		}
		sort.Slice(blockers, func(i, j int) bool { return blockers[i].Identifier < blockers[j].Identifier })
		rows = append(rows, unblockedRow{
			Identifier:        cand.Identifier,
			ID:                cand.ID,
			Title:             cand.Title,
			URL:               cand.URL,
			Priority:          priorityLabel(cand.Priority),
			StateName:         cand.State.Name,
			StateType:         cand.State.Type,
			Team:              cand.Team.Key,
			Blockers:          blockers,
			AllBlockersClosed: allClosed,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].AllBlockersClosed != rows[j].AllBlockersClosed {
			return rows[i].AllBlockersClosed
		}
		return rows[i].Identifier < rows[j].Identifier
	})

	prov := attachFreshness(DataProvenance{
		Source:       "live",
		ResourceType: "issues",
		Reason:       "relations_not_synced",
	}, flags)
	printProvenance(cmd, len(rows), prov)

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing became unblocked.")
			return nil
		}
		tw := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(tw, "ID\tPRI\tBLOCKERS\tSTATE\tTITLE")
		for _, r := range rows {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", r.Identifier, r.Priority, len(r.Blockers), r.StateName, truncate(r.Title, 48))
		}
		return tw.Flush()
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	return renderPayloadWithProvenance(cmd, flags, payload, prov, false)
}

// unblockedCandidate is one crawled issue plus its inline blocker page.
type unblockedCandidate struct {
	ID         string
	Identifier string
	Title      string
	URL        string
	Priority   int
	State      client.IssueRelationState
	Team       struct {
		ID  string
		Key string
	}
	InverseRelations   []client.IssueRelationNode
	RelationsTruncated bool
}

// fetchUnblockedCandidates walks the crawl to exhaustion. Every open issue in
// scope comes back, blocked or not, because the server has no usable "is
// blocked" predicate. The blocks test happens in the caller.
func fetchUnblockedCandidates(c *client.Client, filter map[string]any, limit, relLimit int) ([]unblockedCandidate, error) {
	var out []unblockedCandidate
	cursor := ""
	for {
		vars := map[string]any{"first": limit, "filter": filter, "relFirst": relLimit}
		if cursor != "" {
			vars["after"] = cursor
		}
		var resp struct {
			Issues struct {
				Nodes []struct {
					ID         string                    `json:"id"`
					Identifier string                    `json:"identifier"`
					Title      string                    `json:"title"`
					URL        string                    `json:"url"`
					Priority   int                       `json:"priority"`
					State      client.IssueRelationState `json:"state"`
					Team       struct {
						ID  string `json:"id"`
						Key string `json:"key"`
					} `json:"team"`
					InverseRelations struct {
						Nodes    []client.IssueRelationNode `json:"nodes"`
						PageInfo struct {
							HasNextPage bool `json:"hasNextPage"`
						} `json:"pageInfo"`
					} `json:"inverseRelations"`
				} `json:"nodes"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"issues"`
		}
		if err := c.QueryInto(client.UnblockedCandidatesQuery, vars, &resp); err != nil {
			return nil, err
		}
		for _, n := range resp.Issues.Nodes {
			cand := unblockedCandidate{
				ID:                 n.ID,
				Identifier:         n.Identifier,
				Title:              n.Title,
				URL:                n.URL,
				Priority:           n.Priority,
				State:              n.State,
				InverseRelations:   n.InverseRelations.Nodes,
				RelationsTruncated: n.InverseRelations.PageInfo.HasNextPage,
			}
			cand.Team.ID = n.Team.ID
			cand.Team.Key = n.Team.Key
			out = append(out, cand)
		}
		if !resp.Issues.PageInfo.HasNextPage || resp.Issues.PageInfo.EndCursor == "" {
			return out, nil
		}
		cursor = resp.Issues.PageInfo.EndCursor
	}
}

// unblockedTeamFilter builds the TeamFilter fragment for a key or a UUID.
func unblockedTeamFilter(team string) map[string]any {
	if store.IsUUID(team) {
		return map[string]any{"id": map[string]any{"eq": team}}
	}
	return map[string]any{"key": map[string]any{"eqIgnoreCase": team}}
}
