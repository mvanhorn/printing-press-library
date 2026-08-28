// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

// Project changelog.
//
// Rightbrain's audit log answers "what changed" only in principle: every event
// names its subject by bare UUID, so a week of activity reads as a wall of
// hex that tells you nothing about which task, agent, skill, or collection was
// touched. Correlating those UUIDs by hand means a separate lookup per row.
// This command reads the window out of the local mirror once, then joins every
// resource_id against the mirror's task, agent, skill, and collection tables in
// a single second pass, so the log comes back in names. A UUID with no match in
// the mirror is reported as the bare UUID with resolved=false — never as a
// guessed or blanked name, because a wrong name in a changelog is worse than no
// name at all.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/store"
)

// changelogNameResourceTypes lists the mirror tables whose rows carry a
// human-readable `name` an audit event's resource_id can point at. Adding a
// resource type here is the only change needed to widen name resolution.
var changelogNameResourceTypes = []string{
	"project_task",
	"project_task_agent",
	"project_skill",
	"project_collection",
}

// auditRow is the subset of AuditEventListItem this command consumes, decoded
// from the local mirror's `data` JSON column.
type auditRow struct {
	ID           string          `json:"id"`
	ProjectID    string          `json:"project_id"`
	OrgID        string          `json:"org_id"`
	ActorUserID  string          `json:"actor_user_id"`
	EventType    string          `json:"event_type"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Message      string          `json:"message"`
	Labels       json.RawMessage `json:"labels"`
	OccurredAt   string          `json:"occurred_at"`
	Created      string          `json:"created"`
}

// changelogEvent is one rendered row of the changelog. ResourceName is the
// resolved name when Resolved is true and the bare UUID when it is false.
type changelogEvent struct {
	ID           string `json:"id"`
	OccurredAt   string `json:"occurred_at"`
	EventType    string `json:"event_type"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	Resolved     bool   `json:"resolved"`
	ActorUserID  string `json:"actor_user_id"`
	Message      string `json:"message"`
	Age          string `json:"age"`
}

type changelogReport struct {
	Window           string           `json:"window"`
	Events           []changelogEvent `json:"events"`
	ByResourceType   map[string]int   `json:"by_resource_type"`
	ByActor          map[string]int   `json:"by_actor"`
	Total            int              `json:"total"`
	ResolvedCount    int              `json:"resolved_count"`
	UnresolvedCount  int              `json:"unresolved_count"`
	IntegrityChecked bool             `json:"integrity_checked"`
	Integrity        json.RawMessage  `json:"integrity,omitempty"`
	Note             string           `json:"note,omitempty"`
}

// changelogBucket keys the per-type and per-actor tallies. An event with no
// actor or no resource type still has to be counted somewhere, and dropping it
// would understate the window.
func changelogBucket(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown"
	}
	return s
}

// buildChangelogReport resolves each event's resource_id against the name map
// and rolls the window up by resource type and actor. Split out from RunE so it
// is unit-testable with neither a database nor an API.
func buildChangelogReport(events []auditRow, names map[string]string, now time.Time) changelogReport {
	report := changelogReport{
		Events:         make([]changelogEvent, 0, len(events)),
		ByResourceType: map[string]int{},
		ByActor:        map[string]int{},
	}

	for _, ev := range events {
		row := changelogEvent{
			ID:           ev.ID,
			OccurredAt:   ev.OccurredAt,
			EventType:    ev.EventType,
			ResourceType: ev.ResourceType,
			ResourceID:   ev.ResourceID,
			ActorUserID:  ev.ActorUserID,
			Message:      ev.Message,
		}
		// Never invent a name. An id the mirror does not know stays the bare
		// UUID and is flagged unresolved, so a reader can tell "not synced"
		// apart from "renamed to something that looks like a UUID".
		if name, ok := names[ev.ResourceID]; ok && strings.TrimSpace(name) != "" {
			row.ResourceName = name
			row.Resolved = true
			report.ResolvedCount++
		} else {
			row.ResourceName = ev.ResourceID
			row.Resolved = false
			report.UnresolvedCount++
		}
		if occurred, ok := parseAPITime(ev.OccurredAt); ok {
			row.Age = humanizeDuration(now.Sub(occurred))
		} else {
			row.Age = "unknown"
		}

		report.ByResourceType[changelogBucket(ev.ResourceType)]++
		report.ByActor[changelogBucket(ev.ActorUserID)]++
		report.Events = append(report.Events, row)
	}
	report.Total = len(report.Events)

	// Grouped by resource type, then by resource, most recent change first
	// within each resource — the order the human renderer prints sections in.
	sort.SliceStable(report.Events, func(i, j int) bool {
		a, b := report.Events[i], report.Events[j]
		if a.ResourceType != b.ResourceType {
			return a.ResourceType < b.ResourceType
		}
		if a.ResourceName != b.ResourceName {
			return a.ResourceName < b.ResourceName
		}
		return apiTimeAfter(a.OccurredAt, b.OccurredAt)
	})

	if report.Total == 0 {
		report.Note = "no audit events in this window; widen --since, relax --actor/--resource-type, " +
			"or run 'rightbrain-pp-cli sync --resources project_audit_event' to refresh the mirror"
	}
	return report
}

// sortAuditRowsRecentFirst orders drained rows newest-first so --limit keeps the
// most recent changes rather than an arbitrary slice of the window. Rows with an
// unparseable occurred_at sort last but are never dropped.
func sortAuditRowsRecentFirst(rows []auditRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		ti, iOK := parseAPITime(rows[i].OccurredAt)
		tj, jOK := parseAPITime(rows[j].OccurredAt)
		if iOK != jOK {
			return iOK
		}
		if !iOK {
			return false
		}
		return ti.After(tj)
	})
}

func newNovelChangelogCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagVerify bool
	var flagActor string
	var flagResourceType string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "What changed in the project over a window, with resource UUIDs resolved to names",
		Long: "Read the project audit log for a time window and report it in names.\n\n" +
			"Every audit event names its subject by bare UUID. This command joins each " +
			"resource_id against the locally mirrored tasks, task agents, skills, and " +
			"collections, groups the window by resource type and resource, and summarizes " +
			"who made the changes. A UUID the mirror does not know is reported as the bare " +
			"UUID with resolved=false — no name is ever guessed.\n\n" +
			"Runs entirely against the local mirror; --verify additionally calls the live " +
			"audit-integrity endpoint and attaches its result, which requires a configured scope.\n\n" +
			"Use this command for what changed in the project's configuration over a window, " +
			"with UUIDs resolved to names. Do NOT use it for changes in cost or latency " +
			"metrics; use 'drift' instead.",
		Example: "  rightbrain-pp-cli changelog --since 7d --verify --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list audit events for the window with UUIDs resolved to names")
				return nil
			}

			since, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			if since <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: the window must be positive (e.g. 7d, 1w, 24h)", flagSince))
			}
			if flagLimit < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --limit %d: must be zero (no limit) or positive", flagLimit))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("rightbrain-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: rightbrain-pp-cli sync --resources project_audit_event --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local mirror %s: %w", dbPath, err)
			}
			defer db.Close()

			now := time.Now().UTC()
			cutoff := now.Add(-since)

			rows, err := db.DB().QueryContext(ctx,
				`SELECT id, data FROM resources WHERE resource_type = 'project_audit_event'`)
			if err != nil {
				return fmt.Errorf("reading audit events from the local mirror: %w", err)
			}

			// Drain the cursor completely before running any other query: SQLite
			// serves this store from a single connection, so a name lookup issued
			// inside the loop below would fail mid-scan and silently truncate the
			// changelog.
			events := make([]auditRow, 0)
			malformed := 0
			for rows.Next() {
				// Both columns are scanned NULL-safe. A bare string scan errors on
				// a NULL and the surrounding loop would drop the row, reporting
				// "nothing changed" for a window that did change.
				var rowID, raw sql.NullString
				if scanErr := rows.Scan(&rowID, &raw); scanErr != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning audit event row: %w", scanErr)
				}
				if !raw.Valid || strings.TrimSpace(raw.String) == "" {
					malformed++
					continue
				}
				var ev auditRow
				if jsonErr := json.Unmarshal([]byte(raw.String), &ev); jsonErr != nil {
					malformed++
					continue
				}
				if ev.ID == "" && rowID.Valid {
					ev.ID = rowID.String
				}
				occurred, occurredOK := parseAPITime(ev.OccurredAt)
				if !occurredOK {
					// An event whose timestamp cannot be read cannot be placed in
					// the window; count it rather than pretend it was not there.
					malformed++
					continue
				}
				if occurred.Before(cutoff) || occurred.After(now) {
					continue
				}
				if flagActor != "" && ev.ActorUserID != flagActor {
					continue
				}
				if flagResourceType != "" && ev.ResourceType != flagResourceType {
					continue
				}
				events = append(events, ev)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("reading audit events from the local mirror: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing audit event cursor: %w", err)
			}

			if malformed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d audit row(s) in the local mirror had no readable payload or occurred_at and were excluded\n",
					malformed)
			}

			sortAuditRowsRecentFirst(events)
			if flagLimit > 0 && len(events) > flagLimit {
				events = events[:flagLimit]
			}

			// Second pass, now that the audit cursor is closed: resolve the UUIDs.
			names, err := changelogNameMap(ctx, db)
			if err != nil {
				return fmt.Errorf("resolving resource names from the local mirror: %w", err)
			}

			if !hintIfUnsynced(cmd, db, "project_audit_event") {
				hintIfStale(cmd, db, "project_audit_event", flags.maxAge)
			}

			report := buildChangelogReport(events, names, now)
			report.Window = flagSince

			if flagVerify {
				orgID, projectID, scopeErr := requireScope(flags)
				if scopeErr != nil {
					return scopeErr
				}
				c, clientErr := flags.newClient()
				if clientErr != nil {
					return clientErr
				}
				// verifyProjectAuditIntegrity is POST-only. The GET sibling at
				// /audit_event/integrity is the status probe, not the verifier,
				// so issuing a GET here would 405 at runtime.
				path := fmt.Sprintf("/org/%s/project/%s/audit_event/integrity/verify", orgID, projectID)
				data, _, apiErr := c.Post(ctx, path, map[string]any{})
				if apiErr != nil {
					return classifyAPIError(apiErr, flags)
				}
				report.IntegrityChecked = true
				if len(data) > 0 {
					report.Integrity = json.RawMessage(data)
				}
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printChangelogHuman(cmd, report)
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "7d",
		"Window to report, accepting day and week shorthand (e.g. 24h, 7d, 1w)")
	cmd.Flags().BoolVar(&flagVerify, "verify", false,
		"Also call the live audit-integrity endpoint and attach its result (requires a configured scope)")
	cmd.Flags().StringVar(&flagActor, "actor", "",
		"Only include events performed by this actor user ID")
	cmd.Flags().StringVar(&flagResourceType, "resource-type", "",
		"Only include events whose audit resource_type matches this value exactly")
	cmd.Flags().IntVar(&flagLimit, "limit", 0,
		"Keep only the N most recent events in the window (0 means no limit)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}

// changelogNameMap reads id -> name for every mirror table that carries a name.
// It must run only after the audit-event cursor has been fully drained and
// closed; see the drain comment in RunE.
func changelogNameMap(ctx context.Context, db *store.Store) (map[string]string, error) {
	names := map[string]string{}
	placeholders := make([]string, 0, len(changelogNameResourceTypes))
	args := make([]any, 0, len(changelogNameResourceTypes))
	for _, rt := range changelogNameResourceTypes {
		placeholders = append(placeholders, "?")
		args = append(args, rt)
	}
	query := `SELECT id, json_extract(data, '$.name') FROM resources WHERE resource_type IN (` +
		strings.Join(placeholders, ",") + `)`

	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return names, err
	}
	defer rows.Close()

	for rows.Next() {
		// json_extract returns NULL for any row without a name field, so both
		// columns are scanned NULL-safe; a bare scan would abort the loop and
		// silently leave every later UUID unresolved.
		var id, name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			return names, err
		}
		if !id.Valid || !name.Valid {
			continue
		}
		if strings.TrimSpace(name.String) == "" {
			continue
		}
		names[id.String] = name.String
	}
	if err := rows.Err(); err != nil {
		return names, err
	}
	return names, nil
}

// printChangelogHuman renders the report grouped by resource type, then by
// resource, with a per-actor tally underneath.
func printChangelogHuman(cmd *cobra.Command, report changelogReport) error {
	out := cmd.OutOrStdout()
	if report.Note != "" {
		fmt.Fprintln(out, report.Note)
		if report.IntegrityChecked {
			fmt.Fprintln(out, "integrity: checked")
		}
		return nil
	}

	fmt.Fprintf(out, "window %s — %d change(s), %d named, %d still bare UUIDs\n",
		report.Window, report.Total, report.ResolvedCount, report.UnresolvedCount)

	lastType := ""
	lastResource := ""
	for _, ev := range report.Events {
		if ev.ResourceType != lastType {
			lastType = ev.ResourceType
			lastResource = ""
			fmt.Fprintf(out, "\n%s (%d)\n", changelogBucket(ev.ResourceType), report.ByResourceType[changelogBucket(ev.ResourceType)])
		}
		if ev.ResourceName != lastResource {
			lastResource = ev.ResourceName
			marker := ""
			if !ev.Resolved {
				marker = "  [unresolved uuid]"
			}
			fmt.Fprintf(out, "  %s%s\n", orDash(ev.ResourceName), marker)
		}
		fmt.Fprintf(out, "    %s ago\t%s\t%s\t%s\n",
			ev.Age, orDash(ev.EventType), orDash(ev.ActorUserID), truncate(ev.Message, 60))
	}

	if len(report.ByActor) > 0 {
		actors := make([]string, 0, len(report.ByActor))
		for actor := range report.ByActor {
			actors = append(actors, actor)
		}
		sort.SliceStable(actors, func(i, j int) bool {
			if report.ByActor[actors[i]] != report.ByActor[actors[j]] {
				return report.ByActor[actors[i]] > report.ByActor[actors[j]]
			}
			return actors[i] < actors[j]
		})
		fmt.Fprintln(out, "\nBY ACTOR")
		for _, actor := range actors {
			fmt.Fprintf(out, "%s\t%d\n", actor, report.ByActor[actor])
		}
	}

	if report.IntegrityChecked {
		fmt.Fprintf(out, "\nintegrity: %s\n", orDash(strings.TrimSpace(string(report.Integrity))))
	}
	return nil
}
