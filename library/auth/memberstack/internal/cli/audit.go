// Hand-written novel command: diff the local snapshot against live for drift.

package cli

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/auth/memberstack/internal/store"
)

type auditReport struct {
	LocalCount  int         `json:"localCount"`
	LiveCount   int         `json:"liveCount"`
	OnlyLocal   []string    `json:"onlyLocal"`
	OnlyLive    []string    `json:"onlyLive"`
	Changed     []auditDiff `json:"changed"`
	Unchanged   int         `json:"unchanged"`
	SampledLive int         `json:"sampledLive"`
}

type auditDiff struct {
	ID            string   `json:"id"`
	Email         string   `json:"email,omitempty"`
	ChangedFields []string `json:"changedFields"`
}

func newAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sampleLimit int

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Diff the local snapshot against live to surface what changed since the last sync.",
		Long: `For every member present in the local store, fetches the live record (capped
at --sample for safety) and compares the JSON hash of customFields, metaData,
permissions, planConnections, and verified status. Reports onlyLocal /
onlyLive / changed / unchanged counts.

Run 'memberstack-pp-cli sync --full' first to populate the local store.`,
		Example: `  memberstack-pp-cli audit --sample 50 --json
  memberstack-pp-cli audit --json | jq '.changed | length'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would audit up to %d members against live\n", sampleLimit)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("memberstack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w (hint: run 'sync --full' first)", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, data FROM resources
				WHERE resource_type IN ('members','member')`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			localHashes := map[string]string{}
			localEmails := map[string]string{}
			for rows.Next() {
				var id string
				var data sql.NullString
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				if !data.Valid {
					continue
				}
				var m map[string]any
				if err := json.Unmarshal([]byte(data.String), &m); err != nil {
					continue
				}
				localHashes[id] = memberDriftHash(m)
				if auth, ok := m["auth"].(map[string]any); ok {
					localEmails[id] = stringFromAny(auth["email"])
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			report := auditReport{LocalCount: len(localHashes)}

			liveIDs := map[string]struct{}{}
			liveCursor := 0
			sampled := 0
			for {
				params := map[string]string{"limit": "200", "order": "ASC"}
				if liveCursor > 0 {
					params["after"] = fmt.Sprintf("%d", liveCursor)
				}
				data, err := c.Get(cmd.Context(), "/members", params)
				if err != nil {
					return fmt.Errorf("fetching live members: %w", err)
				}
				var env struct {
					Data struct {
						Data        []map[string]any `json:"data"`
						EndCursor   *int             `json:"endCursor"`
						HasNextPage bool             `json:"hasNextPage"`
						TotalCount  int              `json:"totalCount"`
					} `json:"data"`
				}
				// Try the wrapped shape first.
				if err := json.Unmarshal(data, &env); err != nil || (env.Data.Data == nil && env.Data.TotalCount == 0) {
					// Fallback: some endpoints return a flatter shape.
					var alt struct {
						Data        []map[string]any `json:"data"`
						EndCursor   *int             `json:"endCursor"`
						HasNextPage bool             `json:"hasNextPage"`
						TotalCount  int              `json:"totalCount"`
					}
					if err := json.Unmarshal(data, &alt); err == nil && alt.Data != nil {
						env.Data.Data = alt.Data
						env.Data.EndCursor = alt.EndCursor
						env.Data.HasNextPage = alt.HasNextPage
						env.Data.TotalCount = alt.TotalCount
					}
				}
				report.LiveCount = env.Data.TotalCount
				for _, m := range env.Data.Data {
					id := stringFromAny(m["id"])
					if id == "" {
						continue
					}
					liveIDs[id] = struct{}{}
					sampled++
					localHash, hasLocal := localHashes[id]
					if !hasLocal {
						continue
					}
					liveHash := memberDriftHash(m)
					if liveHash != localHash {
						email := ""
						if auth, ok := m["auth"].(map[string]any); ok {
							email = stringFromAny(auth["email"])
						}
						report.Changed = append(report.Changed, auditDiff{
							ID:            id,
							Email:         email,
							ChangedFields: diffMemberFields(localHash, liveHash, m),
						})
					}
				}
				if env.Data.HasNextPage && env.Data.EndCursor != nil && sampled < sampleLimit {
					liveCursor = *env.Data.EndCursor
					continue
				}
				break
			}

			report.SampledLive = sampled
			report.Unchanged = sampled - len(report.Changed)

			for id := range localHashes {
				if _, ok := liveIDs[id]; !ok {
					report.OnlyLocal = append(report.OnlyLocal, id)
				}
			}
			for id := range liveIDs {
				if _, ok := localHashes[id]; !ok {
					report.OnlyLive = append(report.OnlyLive, id)
				}
			}
			sort.Strings(report.OnlyLocal)
			sort.Strings(report.OnlyLive)

			out, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(out), flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "audit: local=%d  live=%d  sampled=%d  changed=%d  only-local=%d  only-live=%d\n",
				report.LocalCount, report.LiveCount, report.SampledLive,
				len(report.Changed), len(report.OnlyLocal), len(report.OnlyLive))
			for _, d := range report.Changed {
				fmt.Fprintf(cmd.OutOrStdout(), "  changed: %s\t%s\t[%v]\n", d.ID, d.Email, d.ChangedFields)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	cmd.Flags().IntVar(&sampleLimit, "sample", 200, "Maximum live members to walk (caps API spend)")
	return cmd
}

func memberDriftHash(m map[string]any) string {
	driftSubset := map[string]any{
		"customFields":    m["customFields"],
		"metaData":        m["metaData"],
		"permissions":     m["permissions"],
		"planConnections": m["planConnections"],
		"verified":        m["verified"],
	}
	b, _ := json.Marshal(driftSubset)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func diffMemberFields(localHash, liveHash string, live map[string]any) []string {
	// Without the local map cached here, list the drift surface; future
	// versions can carry the local snapshot in memory for field-level diffs.
	_ = localHash
	_ = liveHash
	_ = live
	return []string{"customFields", "metaData", "permissions", "planConnections", "verified"}
}
