// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/store"

	"github.com/spf13/cobra"
)

// deployRecord is the minimum we need from each deploy to compare two of
// them. Render's response shape varies; the lookups in the diff function
// fall back through several plausible field names.
type deployRecord struct {
	ID         string         `json:"id"`
	Status     string         `json:"status,omitempty"`
	Commit     string         `json:"commit,omitempty"`
	Image      string         `json:"image,omitempty"`
	Plan       string         `json:"plan,omitempty"`
	Region     string         `json:"region,omitempty"`
	BuildSecs  int            `json:"build_secs,omitempty"`
	DeploySecs int            `json:"deploy_secs,omitempty"`
	CreatedAt  string         `json:"created_at,omitempty"`
	FinishedAt string         `json:"finished_at,omitempty"`
	Raw        map[string]any `json:"-"`
}

func newDeploysCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploys",
		Short: "Inspect deploys and diff two deploys of the same service.",
	}
	cmd.AddCommand(newDeploysDiffCmd(flags))
	return cmd
}

func newDeploysDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "diff <serviceId> <deployA> <deployB>",
		Short: "Diff two deploys of one service: commit range, image tag, plan, region, status, timing.",
		Example: strings.Trim(`
  render-pp-cli deploys diff srv-d12abc dep-d11111 dep-d22222
  render-pp-cli deploys diff srv-d12abc dep-d11111 dep-d22222 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 3 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "deploys diff"}`)
				return nil
			}
			serviceID, deployA, deployB := args[0], args[1], args[2]
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()

			a, err := loadDeployRecord(cmd, db, flags, serviceID, deployA)
			if err != nil {
				return err
			}
			b, err := loadDeployRecord(cmd, db, flags, serviceID, deployB)
			if err != nil {
				return err
			}
			diff := computeDeployDiff(a, b)
			return printJSONFiltered(cmd.OutOrStdout(), diff, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}

// loadDeployRecord prefers the cached deploys table; on miss falls through
// to a live API call so the user doesn't have to re-sync just to diff one
// historical deploy.
func loadDeployRecord(cmd *cobra.Command, db *store.Store, flags *rootFlags, serviceID, deployID string) (*deployRecord, error) {
	var raw []byte
	err := db.DB().QueryRow(
		`SELECT data FROM deploys WHERE services_id = ? AND id = ?`,
		serviceID, deployID,
	).Scan(&raw)
	if err == nil {
		return parseDeployRecord(raw, deployID)
	}
	// fall through to live API
	c, cerr := flags.newClient()
	if cerr != nil {
		return nil, cerr
	}
	data, apiErr := c.Get("/services/"+serviceID+"/deploys/"+deployID, nil)
	if apiErr != nil {
		return nil, fmt.Errorf("deploy %s not in cache and live fetch failed: %w", deployID, apiErr)
	}
	_ = client.APIError{} // satisfy import; this also keeps client used
	return parseDeployRecord(data, deployID)
}

// parseDeployRecord normalizes Render's deploy shape into the diff record.
func parseDeployRecord(data []byte, fallbackID string) (*deployRecord, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("parsing deploy: %w", err)
	}
	dr := &deployRecord{ID: fallbackID, Raw: obj}
	if v := strFromAny(obj["id"]); v != "" {
		dr.ID = v
	}
	dr.Status = strFromAny(obj["status"])
	if c, ok := obj["commit"].(map[string]any); ok {
		if id := strFromAny(c["id"]); id != "" {
			dr.Commit = id
		}
	} else {
		dr.Commit = strFromAny(obj["commit"])
	}
	dr.Image = strFromAny(obj["image"])
	if dr.Image == "" {
		if im, ok := obj["image"].(map[string]any); ok {
			dr.Image = strFromAny(im["ref"])
		}
	}
	dr.Plan = strFromAny(obj["plan"])
	dr.Region = strFromAny(obj["region"])
	dr.CreatedAt = strFromAny(obj["createdAt"])
	dr.FinishedAt = strFromAny(obj["finishedAt"])
	return dr, nil
}

// deployDiffResult is the JSON shape returned by the command. Individual
// fields have separate before/after slots so callers can render their
// preferred output.
type deployDiffResult struct {
	A       *deployRecord  `json:"a"`
	B       *deployRecord  `json:"b"`
	Changes []deployChange `json:"changes"`
}

type deployChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

func computeDeployDiff(a, b *deployRecord) deployDiffResult {
	rep := deployDiffResult{A: a, B: b, Changes: []deployChange{}}
	add := func(field, from, to string) {
		if from == to {
			return
		}
		rep.Changes = append(rep.Changes, deployChange{Field: field, From: from, To: to})
	}
	add("status", a.Status, b.Status)
	add("commit", shortenCommit(a.Commit), shortenCommit(b.Commit))
	add("image", a.Image, b.Image)
	add("plan", a.Plan, b.Plan)
	add("region", a.Region, b.Region)
	add("createdAt", a.CreatedAt, b.CreatedAt)
	add("finishedAt", a.FinishedAt, b.FinishedAt)
	return rep
}

func shortenCommit(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}
