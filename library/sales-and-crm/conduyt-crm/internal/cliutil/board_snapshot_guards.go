package cliutil

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Hand-written board snapshot guards (board #116 for app #111, contract frozen at app 5729bc7b4), shared by `deals list
// --board-snapshot` and the MCP code-orchestration executor's deals.list. Not generated: the generator emits a plain string
// flag and a bool, so a reprint would send any board_columns value and every conflicting parameter to the API. A reprint
// must keep these.

// The app's lib/deal-board-snapshot-limits.
const (
	BoardSnapshotMaxColumns    = 50
	BoardSnapshotMaxColumnRows = 1000
	BoardSnapshotMaxTotalRows  = 5000
)

// ParseBoardColumns checks a board_columns value (<stageId>[:<limit>],... ) the way the server does: 1-50 entries, each a
// stage UUID, an optional limit of digits only in 1-1000 (default defaultLimit), no stage twice, at most 5000 rows in total
// counting the default. It returns the trimmed wire value.
func ParseBoardColumns(raw string, defaultLimit int) (string, error) {
	entries := strings.Split(raw, ",")
	for i := range entries {
		entries[i] = strings.TrimSpace(entries[i])
		if entries[i] == "" {
			return "", fmt.Errorf("board-columns must be <stageId>[:<limit>],... with no blank entry")
		}
	}
	if len(entries) > BoardSnapshotMaxColumns {
		return "", fmt.Errorf("board-columns names at most %d columns (got %d)", BoardSnapshotMaxColumns, len(entries))
	}
	seen := map[string]bool{}
	total := 0
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts := strings.Split(entry, ":")
		id := strings.TrimSpace(parts[0])
		if len(parts) > 2 || !uuidRE.MatchString(id) {
			return "", fmt.Errorf("board-columns expects <stageId>[:<limit>] entries with a stage UUID; got %q", entry)
		}
		limit := defaultLimit
		if len(parts) == 2 {
			raw := strings.TrimSpace(parts[1])
			n, err := strconv.Atoi(raw)
			if raw == "" || strings.Trim(raw, "0123456789") != "" || err != nil || n < 1 || n > BoardSnapshotMaxColumnRows {
				return "", fmt.Errorf("board-columns limit for %s must be a whole number 1-%d (got %q)", id, BoardSnapshotMaxColumnRows, parts[1])
			}
			limit = n
		}
		key := strings.ToLower(id)
		if seen[key] {
			return "", fmt.Errorf("board-columns names stage %s more than once", id)
		}
		seen[key] = true
		total += limit
		if len(parts) == 2 {
			out = append(out, id+":"+strconv.Itoa(limit))
		} else {
			out = append(out, id)
		}
	}
	if total > BoardSnapshotMaxTotalRows {
		return "", fmt.Errorf("board-columns asks for %d rows; one snapshot answers at most %d (read deeper columns with --stage)", total, BoardSnapshotMaxTotalRows)
	}
	return strings.Join(out, ","), nil
}

// boardSnapshotConflicts are the executor params a snapshot cannot answer (the server's 400 list plus the stage aliases and
// every health_* parameter); page is refused even as 1, as on the CLI and the npm CLI.
var boardSnapshotConflicts = []string{
	"page", "stage", "stage_id", "stageId", "stage_name", "stageName", "rows_only", "summary", "system_occupancy",
	"health_summary", "health_check", "health_limit", "health_offset", "health_cursor", "health_rows_only",
}

var boardSnapshotPipelineKeys = []string{"pipeline", "pipeline_id", "pipelineId", "pipeline_name", "pipelineName"}

// CheckBoardSnapshotParams applies the snapshot rules to an executor params map for deals.list: board_snapshot is a JSON
// boolean; with true, no conflicting parameter, a pipeline, and a valid board_columns string (default limit per_page, else
// 50); board_columns without board_snapshot=true is refused. Without either parameter it is a no-op.
func CheckBoardSnapshotParams(params map[string]any) error {
	snapRaw, hasSnap := params["board_snapshot"]
	cols, hasCols := params["board_columns"]
	if !hasSnap && !hasCols {
		return nil
	}
	snap := false
	if hasSnap {
		b, isBool := snapRaw.(bool)
		if !isBool {
			return fmt.Errorf("board_snapshot must be true or false (a JSON boolean)")
		}
		snap = b
	}
	if !snap {
		if hasCols {
			return fmt.Errorf("board_columns needs board_snapshot=true")
		}
		return nil
	}
	var conflicts []string
	for _, k := range boardSnapshotConflicts {
		if _, ok := params[k]; ok {
			conflicts = append(conflicts, k)
		}
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return fmt.Errorf("board_snapshot=true cannot be combined with %s: it answers every named column's first rows, rows only, from one snapshot", strings.Join(conflicts, ", "))
	}
	// At least one alias must be a non-blank string: "", "  " and non-strings do not name a pipeline.
	hasPipeline := false
	for _, k := range boardSnapshotPipelineKeys {
		if v, ok := params[k]; ok {
			if str, isStr := v.(string); isStr && strings.TrimSpace(str) != "" {
				hasPipeline = true
			}
		}
	}
	if !hasPipeline {
		return fmt.Errorf("board_snapshot=true needs pipeline (or pipeline_name)")
	}
	if !hasCols {
		return fmt.Errorf("board_snapshot=true needs board_columns: <stageId>[:<limit>],...")
	}
	s, isStr := cols.(string)
	if !isStr {
		return fmt.Errorf("board_columns must be a string: <stageId>[:<limit>],...")
	}
	defaultLimit := 50
	if v, ok := params["per_page"]; ok {
		f, isNum := v.(float64)
		if !isNum || f != math.Trunc(f) || f < 1 || f > BoardSnapshotMaxColumnRows {
			return fmt.Errorf("per_page must be a whole number 1-%d with board_snapshot=true", BoardSnapshotMaxColumnRows)
		}
		defaultLimit = int(f)
	}
	_, err := ParseBoardColumns(s, defaultLimit)
	return err
}
