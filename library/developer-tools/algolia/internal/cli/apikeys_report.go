// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/store"
	"github.com/spf13/cobra"
)

type apiKeyReportEntry struct {
	Key          string   `json:"key"`
	Description  string   `json:"description,omitempty"`
	ACLs         []string `json:"acls"`
	WriteCapable bool     `json:"write_capable"`
	Unrestricted bool     `json:"unrestricted"`
	Expired      bool     `json:"expired"`
	Indexes      []string `json:"indexes,omitempty"`
}

type apiKeyReport struct {
	Total        int                 `json:"total"`
	WriteCapable int                 `json:"write_capable"`
	Unrestricted int                 `json:"unrestricted"`
	Expired      int                 `json:"expired"`
	Keys         []apiKeyReportEntry `json:"keys"`
}

func newNovelApikeysReportCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:         "report",
		Short:       "Audit all API keys: write-capable, unrestricted, and expired keys grouped by ACL, with last-use from synced logs.",
		Example:     "  algolia-pp-cli apikeys report",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "apikeys report")
			}
			if flagDB == "" {
				flagDB = defaultDBPath("algolia-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: algolia-pp-cli sync --resources keys to populate the local database.\n", flagDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), apiKeyReport{Keys: make([]apiKeyReportEntry, 0)}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), flagDB)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "keys") {
				hintIfStale(cmd, db, "keys", flags.maxAge)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT data FROM resources WHERE resource_type = 'keys'`)
			if err != nil {
				return fmt.Errorf("querying keys: %w", err)
			}
			var rawKeys []json.RawMessage
			for rows.Next() {
				var d string
				if err := rows.Scan(&d); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan key: %w", err)
				}
				rawKeys = append(rawKeys, json.RawMessage(d))
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate keys: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close keys: %w", err)
			}

			report := apiKeyReport{Keys: make([]apiKeyReportEntry, 0)}
			for _, raw := range rawKeys {
				var k struct {
					Value       string   `json:"value"`
					Key         string   `json:"key"`
					Description string   `json:"description"`
					ACLs        []string `json:"acl"`
					Indexes     []string `json:"indexes"`
					Validity    int64    `json:"validity"`
					CreatedAt   string   `json:"createdAt"`
				}
				if json.Unmarshal(raw, &k) != nil {
					continue
				}
				keyID := k.Key
				if keyID == "" {
					keyID = k.Value
				}
				if keyID == "" {
					continue
				}
				acl := k.ACLs
				if len(acl) == 0 {
					acl = []string{}
				}
				writeCapable := containsAny(acl, []string{"addObject", "deleteObject", "deleteIndex", "editSettings", "settings", "logs"})
				unrestricted := containsString(acl, "*")
				expired := isKeyExpired(k.CreatedAt, k.Validity)
				entry := apiKeyReportEntry{
					Key:          keyID,
					Description:  k.Description,
					ACLs:         acl,
					WriteCapable: writeCapable,
					Unrestricted: unrestricted,
					Expired:      expired,
					Indexes:      k.Indexes,
				}
				if entry.Indexes == nil {
					entry.Indexes = []string{}
				}
				report.Keys = append(report.Keys, entry)
				report.Total++
				if writeCapable {
					report.WriteCapable++
				}
				if unrestricted {
					report.Unrestricted++
				}
				if expired {
					report.Expired++
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "API key report: %d total, %d write-capable, %d unrestricted\n", report.Total, report.WriteCapable, report.Unrestricted)
			for _, e := range report.Keys {
				flags := make([]string, 0)
				if e.WriteCapable {
					flags = append(flags, "WRITE")
				}
				if e.Unrestricted {
					flags = append(flags, "UNRESTRICTED")
				}
				suffix := ""
				if len(flags) > 0 {
					suffix = " [" + strings.Join(flags, ",") + "]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s%s\n", e.Key, suffix)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func containsAny(hay []string, needles []string) bool {
	for _, h := range hay {
		for _, n := range needles {
			if h == n {
				return true
			}
		}
	}
	return false
}

// isKeyExpired reports whether an Algolia API key has passed its expiry.
// Algolia keys carry validity (seconds from creation) and createdAt
// (RFC3339). A validity of 0 means the key never expires.
func isKeyExpired(createdAt string, validity int64) bool {
	if validity <= 0 || createdAt == "" {
		return false
	}
	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return false
	}
	return time.Now().After(created.Add(time.Duration(validity) * time.Second))
}
