package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// Cycle read and write surface (GAP-033). Before this file `cycles` exposed
// only the local `compare` transcendence command, even though sync already
// pulled cycles into a typed table and every cycle mutation was live.
//
// The subcommands are registered on the cycles parent in linear_groups.go.

// cycleRow is the projection `cycles list` renders. It mirrors the node shape
// client.CyclesQuery syncs into the local cycles table so a live read and a
// --data-source local read produce the same fields.
type cycleRow struct {
	ID          string  `json:"id"`
	Number      float64 `json:"number"`
	Name        string  `json:"name,omitempty"`
	StartsAt    string  `json:"startsAt,omitempty"`
	EndsAt      string  `json:"endsAt,omitempty"`
	CompletedAt string  `json:"completedAt,omitempty"`
	Progress    float64 `json:"progress"`
	Team        struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"team"`
}

func newCyclesListCmd(flags *rootFlags) *cobra.Command {
	var team, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "List cycles with their dates and progress, optionally filtered by team",
		Long: `List Linear cycles with their number, name, window, and progress.

With --data-source live (or auto, the default), cycles come from the Linear
GraphQL API and are written through to the local store. With --data-source
local, cycles are read from the synced cycles table, so run 'linear-pp-cli sync'
first.

--team accepts a team key or a team UUID and is applied server-side through
CycleFilter.team on live reads.`,
		Example: `  linear-pp-cli cycles list --team ENG --agent
  linear-pp-cli cycles list --agent --select id,number,name,startsAt,endsAt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				limit = 50
			}
			return runCyclesList(cmd, flags, resolveDBPath(dbPath), team, limit)
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key or UUID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum cycles per live API page")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func runCyclesList(cmd *cobra.Command, flags *rootFlags, dbPath, team string, limit int) error {
	var raw []json.RawMessage
	var prov DataProvenance

	db, openErr := openStoreAt(dbPath)
	if db != nil {
		defer db.Close()
	}

	fetchLocal := func() ([]json.RawMessage, error) {
		if openErr != nil {
			return nil, fmt.Errorf("opening local database: %w\nRun 'linear-pp-cli sync' first", openErr)
		}
		if db == nil {
			return nil, fmt.Errorf("no local data. Run 'linear-pp-cli sync' first")
		}
		teamID := team
		if team != "" && !store.IsUUID(team) {
			resolved, err := resolveTeamFilter(db, team)
			if err != nil {
				return nil, err
			}
			teamID = resolved
		}
		return db.ListCycles(teamID)
	}

	switch flags.dataSource {
	case "local":
		rows, err := fetchLocal()
		if err != nil {
			return err
		}
		raw = rows
		prov = localProvenance(db, "cycles", "user_requested")
	default:
		rows, err := fetchCyclesLive(flags, team, limit)
		if err != nil {
			if flags.dataSource == "live" || !isNetworkError(err) {
				return classifyLiveReadError(err, flags)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "  (live API unreachable, falling back to local store)")
			rows, err = fetchLocal()
			if err != nil {
				return err
			}
			raw = rows
			prov = localProvenance(db, "cycles", "api_unreachable")
			break
		}
		raw = rows
		prov = DataProvenance{Source: "live", ResourceType: "cycles", Reason: "user_requested"}
		// Write-through so a follow-up --data-source local read is fresh.
		if db != nil {
			for _, n := range rows {
				var meta struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(n, &meta) == nil && meta.ID != "" {
					_ = db.UpsertCycle(meta.ID, n)
				}
			}
		}
	}

	rows := make([]cycleRow, 0, len(raw))
	for _, r := range raw {
		var row cycleRow
		if err := json.Unmarshal(r, &row); err != nil {
			continue
		}
		rows = append(rows, row)
	}

	prov = attachFreshness(prov, flags)
	printProvenance(cmd, len(rows), prov)
	return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
}

// fetchCyclesLive reuses client.CyclesQuery, the same document sync uses, so
// live rows and synced rows are field-identical. The team filter rides on
// CycleFilter.team, which accepts a TeamFilter, so a team key resolves
// server-side without a local sync.
func fetchCyclesLive(flags *rootFlags, team string, limit int) ([]json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	vars := map[string]any{"first": limit}
	if team != "" {
		if store.IsUUID(team) {
			vars["filter"] = map[string]any{"team": map[string]any{"id": map[string]any{"eq": team}}}
		} else {
			vars["filter"] = map[string]any{"team": map[string]any{"key": map[string]any{"eq": team}}}
		}
	}
	return c.PaginatedQueryMax(client.CyclesQuery, vars, "cycles", limit, 10)
}

// Cycle write surface. cycleCreate, cycleUpdate, cycleArchive, cycleShiftAll,
// and cycleStartUpcomingCycleToday are all live in api-inventory.json.

func newCyclesCreateCmd(flags *rootFlags) *cobra.Command {
	var teamFlag, startsAt, endsAt, nameFlag, descFlag, dbPath string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a cycle on a team with an explicit start and end",
		Long: `Create a Linear cycle via the cycleCreate mutation.

CycleCreateInput requires teamId, startsAt, and endsAt. Dates are DateTime
values, so pass an ISO 8601 timestamp or date (2026-09-01 or
2026-09-01T00:00:00Z). --name is optional: Linear numbers cycles automatically
and a custom name only overrides the display label.`,
		Example: `  linear-pp-cli cycles create --team ENG --starts-at 2026-09-01 --ends-at 2026-09-14 --agent
  linear-pp-cli cycles create --team ENG --starts-at 2026-09-01 --ends-at 2026-09-14 --name "Hardening" --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(teamFlag) == "" {
				return usageErr(fmt.Errorf("--team is required (team key like ENG or team UUID)"))
			}
			if strings.TrimSpace(startsAt) == "" || strings.TrimSpace(endsAt) == "" {
				return usageErr(fmt.Errorf("--starts-at and --ends-at are both required (CycleCreateInput.startsAt and .endsAt are non-null)"))
			}
			buildInput := func(c graphqlQueryer) (map[string]any, error) {
				teamID, err := resolveWriteTeamID(c, dbPath, teamFlag)
				if err != nil {
					return nil, err
				}
				input := map[string]any{
					"teamId":   teamID,
					"startsAt": startsAt,
					"endsAt":   endsAt,
				}
				setOptionalString(input, "name", nameFlag)
				setOptionalString(input, "description", descFlag)
				return input, nil
			}
			if flags.dryRun {
				input, err := buildInput(nil)
				if err != nil {
					return err
				}
				return renderMutationDryRun(cmd, flags, "would_create_cycle", "cycleCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			input, err := buildInput(c)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			resp, err := c.Mutate(client.CycleCreateMutation, map[string]any{"input": input})
			if err != nil {
				return classifyGraphQLCreateError("cycleCreate", err, flags)
			}
			cycle, err := extractMutationObject(resp, "cycleCreate", "cycle")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, cycle, "cycles")
		},
	}
	cmd.Flags().StringVar(&teamFlag, "team", "", "Team key or UUID that owns the cycle (required)")
	cmd.Flags().StringVar(&startsAt, "starts-at", "", "Cycle start as an ISO 8601 date or datetime (required)")
	cmd.Flags().StringVar(&endsAt, "ends-at", "", "Cycle end as an ISO 8601 date or datetime (required)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "Custom cycle name")
	cmd.Flags().StringVar(&descFlag, "description", "", "Cycle description")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (for team-key resolution)")
	return cmd
}

// newCyclesUpdateCmd exposes no --team flag: CycleUpdateInput has no teamId,
// so a cycle cannot be moved between teams.
func newCyclesUpdateCmd(flags *rootFlags) *cobra.Command {
	var nameFlag, descFlag, startsAt, endsAt, completedAt string
	cmd := &cobra.Command{
		Use:   "update <cycle-id>",
		Short: "Update a cycle's name, description, window, or completion time",
		Long: `Update a Linear cycle via the cycleUpdate mutation.

Only the fields you pass are sent. CycleUpdateInput carries name, description,
startsAt, endsAt, and completedAt, and nothing else: a cycle's team is fixed.

To move a whole run of cycles rather than one, use 'cycles shift-all'.`,
		Example: `  linear-pp-cli cycles update <cycle-uuid> --ends-at 2026-09-21 --agent
  linear-pp-cli cycles update <cycle-uuid> --name "Hardening" --dry-run --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := map[string]any{}
			setChangedString(cmd, input, "name", "name", nameFlag)
			setChangedString(cmd, input, "description", "description", descFlag)
			setChangedString(cmd, input, "starts-at", "startsAt", startsAt)
			setChangedString(cmd, input, "ends-at", "endsAt", endsAt)
			setChangedString(cmd, input, "completed-at", "completedAt", completedAt)
			if len(input) == 0 {
				return usageErr(fmt.Errorf("no cycle fields supplied, pass --name, --description, --starts-at, --ends-at, or --completed-at"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_cycle", "cycleUpdate", map[string]any{"cycle": args[0], "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.CycleUpdateMutation, map[string]any{"id": args[0], "input": input})
			if err != nil {
				return classifyGraphQLMutationError("cycleUpdate", err, flags)
			}
			cycle, err := extractMutationObject(resp, "cycleUpdate", "cycle")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, cycle, "cycles")
		},
	}
	cmd.Flags().StringVar(&nameFlag, "name", "", "New cycle name")
	cmd.Flags().StringVar(&descFlag, "description", "", "New cycle description")
	cmd.Flags().StringVar(&startsAt, "starts-at", "", "New cycle start as an ISO 8601 date or datetime")
	cmd.Flags().StringVar(&endsAt, "ends-at", "", "New cycle end as an ISO 8601 date or datetime")
	cmd.Flags().StringVar(&completedAt, "completed-at", "", "Mark the cycle completed at this ISO 8601 timestamp")
	return cmd
}

func newCyclesArchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <cycle-id>",
		Short: "Archive a cycle, unlinking every issue still assigned to it",
		Long: `Archive a Linear cycle via the cycleArchive mutation.

Linear unlinks every issue currently assigned to the cycle before archiving it,
so those issues end up with no cycle rather than being archived themselves.
Requires confirmation unless --yes is set.`,
		Example: `  linear-pp-cli cycles archive <cycle-uuid> --yes --agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_archive_cycle", "cycleArchive", map[string]any{"input": map[string]any{"id": args[0]}})
			}
			if err := confirmMutation(cmd, flags, fmt.Sprintf("Archive cycle %s and unlink every issue assigned to it?", args[0])); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.CycleArchiveMutation, map[string]any{"id": args[0]})
			if err != nil {
				return classifyGraphQLMutationError("cycleArchive", err, flags)
			}
			cycle, err := extractMutationObject(resp, "cycleArchive", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, cycle, "cycles")
		},
	}
	return cmd
}

// newCyclesShiftAllCmd exposes exactly the two fields CycleShiftAllInput has:
// id (the cycle to start shifting from) and daysToShift. There is no team
// field on the input, so the starting cycle id is what scopes the shift.
func newCyclesShiftAllCmd(flags *rootFlags) *cobra.Command {
	var fromCycle string
	var days float64
	cmd := &cobra.Command{
		Use:   "shift-all",
		Short: "Shift a cycle and every cycle after it by a number of days",
		Long: `Shift Linear cycle windows via the cycleShiftAll mutation.

CycleShiftAllInput has exactly two fields and both are required: id, the cycle
at which the shift starts, and daysToShift, the number of days to move that
cycle and every cycle after it. There is no team field: the starting cycle
determines which team's schedule moves.

Negative values pull the schedule earlier. Requires confirmation unless --yes
is set, because it rewrites every future cycle window on the team.`,
		Example: `  linear-pp-cli cycles shift-all --from-cycle <cycle-uuid> --days 7 --yes --agent
  linear-pp-cli cycles shift-all --from-cycle <cycle-uuid> --days -3 --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(fromCycle) == "" {
				return usageErr(fmt.Errorf("--from-cycle is required (CycleShiftAllInput.id, the cycle the shift starts at)"))
			}
			if !cmd.Flags().Changed("days") {
				return usageErr(fmt.Errorf("--days is required (CycleShiftAllInput.daysToShift)"))
			}
			input := map[string]any{"id": fromCycle, "daysToShift": days}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_shift_cycles", "cycleShiftAll", map[string]any{"input": input})
			}
			if err := confirmMutation(cmd, flags, fmt.Sprintf("Shift cycle %s and every cycle after it by %g days?", fromCycle, days)); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.CycleShiftAllMutation, map[string]any{"input": input})
			if err != nil {
				return classifyGraphQLMutationError("cycleShiftAll", err, flags)
			}
			cycle, err := extractMutationObject(resp, "cycleShiftAll", "cycle")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, cycle, "cycles")
		},
	}
	cmd.Flags().StringVar(&fromCycle, "from-cycle", "", "UUID of the cycle the shift starts at (required)")
	cmd.Flags().Float64Var(&days, "days", 0, "Days to shift by, negative pulls the schedule earlier (required)")
	return cmd
}

func newCyclesStartUpcomingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-upcoming <cycle-id>",
		Short: "Start the next upcoming cycle as of midnight today",
		Long: `Start a Linear cycle early via the cycleStartUpcomingCycleToday mutation.

Only the team's next not-yet-started cycle can be started this way. Linear
completes the previous cycle first if it has not already ended, so this closes
one cycle and opens the next in a single call. Requires confirmation unless
--yes is set.`,
		Example: `  linear-pp-cli cycles start-upcoming <cycle-uuid> --yes --agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_start_upcoming_cycle", "cycleStartUpcomingCycleToday", map[string]any{"input": map[string]any{"id": args[0]}})
			}
			if err := confirmMutation(cmd, flags, fmt.Sprintf("Start cycle %s today, completing the current cycle if it has not ended?", args[0])); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.CycleStartUpcomingCycleTodayMutation, map[string]any{"id": args[0]})
			if err != nil {
				return classifyGraphQLMutationError("cycleStartUpcomingCycleToday", err, flags)
			}
			cycle, err := extractMutationObject(resp, "cycleStartUpcomingCycleToday", "cycle")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, cycle, "cycles")
		},
	}
	return cmd
}
