package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/postman-explore/internal/store"
)

// validMetric reports whether name is one of the metric dimensions Postman
// embeds in entity payloads. This is the floor: top, velocity, and similar
// ranking commands reject anything else with a usage error.
func validMetric(name string) bool {
	switch name {
	case "forkCount", "monthForkCount", "monthViewCount", "monthWatchCount",
		"publicViewCount", "viewCount", "watchCount",
		"weekForkCount", "weekViewCount", "weekWatchCount":
		return true
	}
	return false
}

func metricNamesList() string {
	return "forkCount, monthForkCount, monthViewCount, monthWatchCount, publicViewCount, viewCount, watchCount, weekForkCount, weekViewCount, weekWatchCount"
}

// validEntityType reports whether t is a valid networkentity type.
func validEntityType(t string) bool {
	switch t {
	case "collection", "workspace", "api", "flow":
		return true
	}
	return false
}

// extractMetricValue pulls metricName's metricValue out of a metrics array
// stored as JSON in the entity. Returns 0 when the metric is missing.
func extractMetricValue(data json.RawMessage, metricName string) int64 {
	var obj struct {
		Metrics []struct {
			MetricName  string `json:"metricName"`
			MetricValue int64  `json:"metricValue"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return 0
	}
	for _, m := range obj.Metrics {
		if m.MetricName == metricName {
			return m.MetricValue
		}
	}
	return 0
}

// openLocalStore opens the standard CLI database. Centralized so every novel
// command uses the same default path and error message.
func localStorePath(flags *rootFlags) string {
	if flags != nil {
		return store.ResolvePath("postman-explore-pp-cli", flags.dbPath)
	}
	return store.DefaultPath("postman-explore-pp-cli")
}

func openLocalStore(flags *rootFlags) (*store.Store, error) {
	return store.Open(localStorePath(flags))
}

// queryNetworkEntities runs a SELECT against the synced entity rows and
// returns the JSON `data` blobs. The sync command stores per-type entities
// (collection / workspace / api / flow) in the generic `resources` table
// under their own resource_type, so this helper queries there rather than
// the typed `networkentity` table to match what's actually populated.
//
// extraWhere is appended after `resource_type = ?` (no leading AND); pass an
// empty string when no extra filtering is needed.
func queryNetworkEntities(db *store.Store, entityType, extraWhere string) ([]json.RawMessage, error) {
	q := `SELECT data FROM resources WHERE resource_type = ?`
	if entityType != "" && extraWhere != "" {
		q += " AND " + extraWhere
	} else if extraWhere != "" {
		q += " AND " + extraWhere
	}
	rows, err := db.DB().Query(q, entityType)
	if err != nil {
		return nil, fmt.Errorf("querying resources: %w", err)
	}
	defer rows.Close()
	return scanNetworkentityRows(rows)
}

// queryAllNetworkEntities returns all rows whose resource_type is one of the
// four networkentity sub-types. Used by `publishers top` and `category
// landscape` which aggregate across types.
func queryAllNetworkEntities(db *store.Store, extraWhere string) ([]json.RawMessage, error) {
	q := `SELECT data FROM resources WHERE resource_type IN ('collection','workspace','api','flow')`
	if extraWhere != "" {
		q += " AND " + extraWhere
	}
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, fmt.Errorf("querying resources: %w", err)
	}
	defer rows.Close()
	return scanNetworkentityRows(rows)
}

// scanNetworkentityRows reads (data) JSON columns from a networkentity query
// into a slice of raw messages. The resources table stores `data` as TEXT
// (modernc.org/sqlite's driver hands back a string for TEXT columns), so we
// scan into a string and convert. Caller closes rows.
func scanNetworkentityRows(rows *sql.Rows) ([]json.RawMessage, error) {
	var out []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(data))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// publisherInfoMap parses the meta.publisherInfo block from a browse-style
// response into a {publisherID -> isVerified} map. Returns nil when the
// response carries no publisherInfo block.
func publisherInfoMap(raw json.RawMessage) map[string]bool {
	var resp struct {
		Meta struct {
			PublisherInfo map[string][]struct {
				ID         string `json:"id"`
				IsVerified bool   `json:"isVerified"`
			} `json:"publisherInfo"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	if resp.Meta.PublisherInfo == nil {
		return nil
	}
	out := make(map[string]bool)
	for _, group := range resp.Meta.PublisherInfo {
		for _, p := range group {
			if p.ID == "" {
				continue
			}
			out[p.ID] = p.IsVerified
		}
	}
	return out
}

// entityPublisherID extracts the publisherId field as a string for lookup
// against publisherInfoMap. Returns "" when not present.
func entityPublisherID(data json.RawMessage) string {
	var obj struct {
		PublisherID any `json:"publisherId"`
		Meta        struct {
			PublisherID string `json:"publisherId"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	if obj.Meta.PublisherID != "" {
		return obj.Meta.PublisherID
	}
	switch v := obj.PublisherID.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%d", int64(v))
	}
	return ""
}

// emptyMessage prints a friendly empty-result line with a reason when humans
// run a command in non-JSON mode.
func emptyMessage(reason string) string {
	return fmt.Sprintf("(no results: %s)", strings.TrimSpace(reason))
}

// printJSONFiltered marshals v to JSON, applies --select / --compact filters
// from the rootFlags, and prints the result. Use this from novel commands
// that build Go-typed slices/structs rather than passing through raw API
// payloads — it keeps --select honoring on parity with the endpoint-mirror
// commands. The cobra command interface is reduced to the OutOrStdout
// method we actually need so the helper stays testable.
func printJSONFiltered(cmd interface {
	OutOrStdout() io.Writer
}, v any, flags *rootFlags) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	filtered := json.RawMessage(raw)
	if flags.selectFields != "" {
		filtered = filterFields(filtered, flags.selectFields)
	} else if flags.compact {
		filtered = compactFields(filtered)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(filtered)
}
