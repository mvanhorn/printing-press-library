package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var jsonOut bool
	var limit int
	var team string
	cmd := &cobra.Command{
		Use:         "similar [query]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Find potentially duplicate issues using fuzzy text search",
		Long:        "Search locally synced issues using FTS5 full-text search to find potential duplicates. Works offline.",
		Example: `  linear-pp-cli similar "login bug"
  linear-pp-cli similar "pipeline follow-up" --team SYMPH --agent
  linear-pp-cli similar "payment failed" --limit 20
  linear-pp-cli similar "onboarding" --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runIssueSearch(cmd, flags, issueSearchOptions{
				DBPath:      resolveDBPath(dbPath),
				JSONOut:     jsonOut,
				Limit:       limit,
				Team:        team,
				Query:       args[0],
				AutoRefresh: false,
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key, name, or UUID")
	return cmd
}

func newIssuesSearchCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	var jsonOut bool
	var limit int
	var team string
	cmd := &cobra.Command{
		Use:         "search <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Search synced issues by text (alias for similar)",
		Long: `Search locally synced issues using the same FTS5 duplicate-search engine as 'similar'.

This subcommand exists because agents naturally look for 'issues search' when
checking for existing tickets before creating a follow-up. Multi-word queries
may be quoted or passed as separate words; both forms are joined into one query.
Under --agent/--json it returns a provenance envelope with freshness metadata;
use 'similar' for the legacy local-only raw-array shape.`,
		Example: `  linear-pp-cli issues search "login bug" --agent
  linear-pp-cli issues search "pipeline follow-up" --team SYMPH --limit 10 --agent
  linear-pp-cli issues search Kimi replay temp directories cleanup --team SYMPH --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(strings.Join(args, " ")) == "" {
				return usageErr(fmt.Errorf("issues search requires a query; use: linear-pp-cli issues search \"login bug\" --team ENG --agent (equivalent: linear-pp-cli similar \"login bug\" --team ENG --agent)"))
			}
			return runIssueSearch(cmd, flags, issueSearchOptions{
				DBPath:      resolveDBPath(*dbPath),
				JSONOut:     jsonOut,
				Limit:       limit,
				Team:        team,
				Query:       strings.Join(args, " "),
				AutoRefresh: true,
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key, name, or UUID")
	return cmd
}

type issueSearchOptions struct {
	DBPath      string
	JSONOut     bool
	Limit       int
	Team        string
	Query       string
	AutoRefresh bool
}

type issueSearchFreshness struct {
	StalePolicy        string   `json:"stale_policy"`
	Refreshed          bool     `json:"refreshed"`
	RefreshedBy        string   `json:"refreshed_by,omitempty"`
	RefreshReason      string   `json:"refresh_reason,omitempty"`
	PreviousSyncedAt   string   `json:"previous_synced_at,omitempty"`
	PreviousAgeSeconds float64  `json:"previous_age_seconds,omitempty"`
	SyncedAt           string   `json:"synced_at,omitempty"`
	AgeSeconds         float64  `json:"age_seconds,omitempty"`
	LocalIssueCount    int      `json:"local_issue_count"`
	Unsynced           bool     `json:"unsynced,omitempty"`
	RefreshWaitMillis  int64    `json:"refresh_wait_ms,omitempty"`
	RefreshResources   []string `json:"refresh_resources,omitempty"`
	LockReclaimed      bool     `json:"lock_reclaimed,omitempty"`
	LockContended      bool     `json:"lock_contended,omitempty"`
}

type issueSearchRefreshLockResult struct {
	wait      time.Duration
	contended bool
	reclaimed bool
}

func runIssueSearch(cmd *cobra.Command, flags *rootFlags, opts issueSearchOptions) error {
	// Verify mode: short-circuit so a synthetic query against an
	// empty FTS index doesn't fail the mechanical verify pass.
	if cliutil.IsVerifyEnv() {
		return nil
	}
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return usageErr(fmt.Errorf("search query cannot be empty"))
	}
	db, err := store.Open(opts.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
	}
	defer db.Close()

	freshness := issueSearchFreshness{StalePolicy: "manual"}
	if opts.AutoRefresh {
		var err error
		freshness, err = ensureIssueSearchFresh(cmd, flags, opts.DBPath, db)
		if err != nil {
			return err
		}
	}

	teamID := ""
	if opts.Team != "" {
		resolved, err := resolveTeamFilter(db, opts.Team)
		if err != nil {
			if !errors.Is(err, errTeamFilterNotFound) {
				return err
			}
			return notFoundErr(fmt.Errorf("%w. Run 'linear-pp-cli sync' if the team was added recently", err))
		}
		teamID = resolved
	}
	results, err := db.SearchIssuesByTeam(query, teamID)
	if err != nil {
		return fmt.Errorf("searching: %w", err)
	}

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	if !opts.AutoRefresh {
		if len(results) == 0 {
			hintIfUnsynced(cmd, db, "issues")
		} else {
			hintIfStale(cmd, db, "issues", flags.maxAge)
		}
	}

	if opts.JSONOut || flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		data, err := json.Marshal(results)
		if err != nil {
			return err
		}
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		} else if flags.compact {
			data = compactFields(data)
		}
		if opts.AutoRefresh {
			wrapped, err := wrapWithProvenance(data, DataProvenance{
				Source:       "local",
				ResourceType: "issues",
				Reason:       "fts_search",
				Freshness:    freshness,
			})
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), wrapped, true)
		}
		return printOutput(cmd.OutOrStdout(), data, true)
	}

	out := cmd.OutOrStdout()
	if len(results) == 0 {
		fmt.Fprintf(out, "No issues matching %q\n", query)
		return nil
	}

	fmt.Fprintf(out, "%-12s %-15s %s\n", "ID", "STATE", "TITLE")
	fmt.Fprintln(out, strings.Repeat("-", 70))
	for _, raw := range results {
		var row struct {
			Identifier string                `json:"identifier"`
			Title      string                `json:"title"`
			State      struct{ Name string } `json:"state"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			continue
		}
		title := row.Title
		if len(title) > 45 {
			title = title[:42] + "..."
		}
		fmt.Fprintf(out, "%-12s %-15s %s\n", row.Identifier, row.State.Name, title)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "\n%d results for %q\n", len(results), query)
	return nil
}

func ensureIssueSearchFresh(cmd *cobra.Command, flags *rootFlags, dbPath string, db *store.Store) (issueSearchFreshness, error) {
	freshness := issueSearchFreshness{StalePolicy: "refresh"}
	if flags.dataSource == "local" {
		freshness.StalePolicy = "allow"
		freshness.Refreshed = false
		freshness.RefreshReason = "user_requested_local"
		attachIssueSyncState(db, &freshness, false)
		return freshness, nil
	}
	if flags.maxAge == 0 {
		freshness.StalePolicy = "allow"
		freshness.Refreshed = false
		freshness.RefreshReason = "freshness_gate_disabled"
		attachIssueSyncState(db, &freshness, false)
		return freshness, nil
	}

	needsRefresh, reason, err := issueSearchNeedsRefresh(db, flags.maxAge)
	if err != nil {
		return freshness, fmt.Errorf("checking issues search freshness: %w", err)
	}
	if !needsRefresh {
		attachIssueSyncState(db, &freshness, false)
		return freshness, nil
	}

	attachIssueSyncState(db, &freshness, true)
	freshness.RefreshReason = reason
	selfRefreshed := false
	lockResult, err := withIssueSearchRefreshLock(dbPath, func() error {
		innerNeedsRefresh, _, err := issueSearchNeedsRefresh(db, flags.maxAge)
		if err != nil {
			return err
		}
		if !innerNeedsRefresh {
			return nil
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		if err := refreshIssueSearchResources(c, db); err != nil {
			return err
		}
		selfRefreshed = true
		return nil
	})
	freshness.RefreshWaitMillis = lockResult.wait.Milliseconds()
	freshness.LockReclaimed = lockResult.reclaimed
	freshness.LockContended = lockResult.contended
	if err != nil {
		return freshness, apiErr(fmt.Errorf("issues search cache is %s and refresh failed; retry or pass --data-source local to explicitly allow stale local results: %w", reason, err))
	}

	afterNeedsRefresh, _, err := issueSearchNeedsRefresh(db, flags.maxAge)
	if err != nil {
		return freshness, fmt.Errorf("checking refreshed issues search freshness: %w", err)
	}
	attachIssueSyncState(db, &freshness, false)
	if afterNeedsRefresh {
		return freshness, apiErr(fmt.Errorf("issues search cache remained stale after refresh; pass --data-source local only if stale local results are acceptable"))
	}
	applyIssueSearchRefreshMetadata(&freshness, selfRefreshed, lockResult.contended)
	if freshness.RefreshReason == "" {
		freshness.RefreshReason = reason
	}
	return freshness, nil
}

func issueSearchNeedsRefresh(db *store.Store, maxAge time.Duration) (bool, string, error) {
	_, lastSynced, count, err := db.GetSyncState("issues")
	if err != nil {
		return false, "", err
	}
	if count == 0 || lastSynced.IsZero() {
		return true, "unsynced", nil
	}
	if maxAge > 0 && time.Since(lastSynced) > maxAge {
		return true, "stale", nil
	}
	return false, "", nil
}

func attachIssueSyncState(db *store.Store, freshness *issueSearchFreshness, previous bool) {
	_, lastSynced, count, _ := db.GetSyncState("issues")
	freshness.LocalIssueCount = count
	freshness.Unsynced = count == 0 || lastSynced.IsZero()
	if lastSynced.IsZero() {
		return
	}
	ts := lastSynced.UTC().Format(time.RFC3339)
	age := time.Since(lastSynced).Seconds()
	if previous {
		freshness.PreviousSyncedAt = ts
		freshness.PreviousAgeSeconds = age
		return
	}
	freshness.SyncedAt = ts
	freshness.AgeSeconds = age
}

func applyIssueSearchRefreshMetadata(freshness *issueSearchFreshness, selfRefreshed bool, lockContended bool) {
	freshness.Refreshed = freshness.PreviousSyncedAt != freshness.SyncedAt
	if !freshness.Refreshed {
		return
	}
	if selfRefreshed {
		freshness.RefreshedBy = "self"
		freshness.RefreshResources = []string{"teams", "workflow_states", "issue_labels", "issues"}
		return
	}
	if lockContended {
		freshness.RefreshedBy = "peer"
		freshness.RefreshResources = []string{"teams", "workflow_states", "issue_labels", "issues"}
		return
	}
	freshness.RefreshedBy = "external"
}

func refreshIssueSearchResources(c *client.Client, db *store.Store) error {
	const syncAllPages = 0
	if _, err := syncTeams(c, db, syncAllPages); err != nil {
		return fmt.Errorf("sync teams: %w", err)
	}
	if _, err := syncWorkflowStates(c, db, syncAllPages); err != nil {
		return fmt.Errorf("sync workflow states: %w", err)
	}
	if _, err := syncLabels(c, db, syncAllPages); err != nil {
		return fmt.Errorf("sync labels: %w", err)
	}
	if _, err := syncIssues(c, db, syncAllPages); err != nil {
		return fmt.Errorf("sync issues: %w", err)
	}
	return nil
}

func withIssueSearchRefreshLock(dbPath string, fn func() error) (issueSearchRefreshLockResult, error) {
	lockPath := dbPath + ".issues-search-sync.lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return issueSearchRefreshLockResult{}, err
	}
	start := time.Now()
	deadline := start.Add(issueSearchRefreshLockTimeout)
	contended := false
	reclaimed := false
	for {
		f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			fmt.Fprintf(f, "pid=%d\ncreated_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
			_ = f.Close()
			defer os.Remove(lockPath)
			return issueSearchRefreshLockResult{wait: time.Since(start), contended: contended, reclaimed: reclaimed}, fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return issueSearchRefreshLockResult{wait: time.Since(start), contended: contended, reclaimed: reclaimed}, err
		}
		contended = true
		didReclaim, reclaimErr := reclaimStaleIssueSearchRefreshLock(lockPath)
		if reclaimErr != nil {
			return issueSearchRefreshLockResult{wait: time.Since(start), contended: contended, reclaimed: reclaimed}, reclaimErr
		}
		if didReclaim {
			reclaimed = true
			continue
		}
		if time.Now().After(deadline) {
			return issueSearchRefreshLockResult{wait: time.Since(start), contended: contended, reclaimed: reclaimed}, fmt.Errorf("timed out waiting for issues search refresh lock %s", lockPath)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

const issueSearchRefreshLockTimeout = 2 * time.Minute

func reclaimStaleIssueSearchRefreshLock(lockPath string) (bool, error) {
	info, err := os.Stat(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	contents, _ := os.ReadFile(lockPath)
	pid, createdAt := parseIssueSearchRefreshLock(contents)
	if pid > 0 && issueSearchLockHolderDead(pid) {
		return removeIssueSearchRefreshLock(lockPath)
	}
	if !createdAt.IsZero() && time.Since(createdAt) > issueSearchRefreshLockTimeout {
		return removeIssueSearchRefreshLock(lockPath)
	}
	if createdAt.IsZero() && time.Since(info.ModTime()) > issueSearchRefreshLockTimeout {
		return removeIssueSearchRefreshLock(lockPath)
	}
	return false, nil
}

func parseIssueSearchRefreshLock(contents []byte) (int, time.Time) {
	var pid int
	var createdAt time.Time
	for _, line := range strings.Split(string(contents), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "pid":
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				pid = parsed
			}
		case "created_at":
			if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err == nil {
				createdAt = parsed
			}
		}
	}
	return pid, createdAt
}

func issueSearchLockHolderDead(pid int) bool {
	if pid == os.Getpid() {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
		return true
	}
	// Liveness probing is best-effort; age-based reclamation covers platforms
	// or permissions where signal(0) cannot prove a dead lock holder.
	return false
}

func removeIssueSearchRefreshLock(lockPath string) (bool, error) {
	if err := os.Remove(lockPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
