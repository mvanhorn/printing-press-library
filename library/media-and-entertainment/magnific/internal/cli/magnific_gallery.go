package cli

import (
	"database/sql"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"

	"github.com/spf13/cobra"
)

func newMagnificGalleryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gallery",
		Short: "Query and open downloaded Magnific outputs from the local archive",
		Long: `gallery is the offline browser for outputs you have downloaded. Tags,
orientation, model, and dates are all queryable filters over the local
magnific_assets table. 'gallery open' honors PRINTING_PRESS_VERIFY and
requires --launch to actually open the file.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newMagnificGalleryListCmd(flags))
	cmd.AddCommand(newMagnificGalleryOpenCmd(flags))
	return cmd
}

func newMagnificGalleryListCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var tag string
	var orientation string
	var modelFilter string
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List downloaded outputs filtered by tag, date, orientation, or model",
		Example:     "  magnific-pp-cli gallery list --tag client-acme --since 7d --orientation landscape --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}

			where := []string{}
			args2 := []any{}
			if sinceStr != "" {
				dur, err := parseDurationFlag(sinceStr)
				if err != nil {
					return usageErr(fmt.Errorf("--since %q: %w", sinceStr, err))
				}
				cutoff := time.Now().Add(-dur).UTC().Format(time.RFC3339)
				where = append(where, "downloaded_at >= ?")
				args2 = append(args2, cutoff)
			}
			if tag != "" {
				where = append(where, "tag = ?")
				args2 = append(args2, tag)
			}
			if orientation != "" {
				where = append(where, "orientation = ?")
				args2 = append(args2, orientation)
			}
			if modelFilter != "" {
				where = append(where, "model = ?")
				args2 = append(args2, modelFilter)
			}
			args2 = append(args2, limit)

			whereStr := ""
			if len(where) > 0 {
				whereStr = "WHERE " + strings.Join(where, " AND ")
			}
			q := fmt.Sprintf(`
				SELECT id, local_path, COALESCE(model,''), COALESCE(tag,''),
					COALESCE(orientation,''), COALESCE(downloaded_at,''),
					COALESCE(task_id,''), COALESCE(size_bytes, 0)
				FROM magnific_assets %s
				ORDER BY downloaded_at DESC LIMIT ?`, whereStr)
			rows, err := db.DB().QueryContext(ctx, q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			type row struct {
				ID           string `json:"id"`
				LocalPath    string `json:"local_path"`
				Model        string `json:"model,omitempty"`
				Tag          string `json:"tag,omitempty"`
				Orientation  string `json:"orientation,omitempty"`
				DownloadedAt string `json:"downloaded_at"`
				TaskID       string `json:"task_id,omitempty"`
				SizeBytes    int64  `json:"size_bytes,omitempty"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var id, lp, m, t, o, dl, tid sql.NullString
				var sz sql.NullInt64
				if err := rows.Scan(&id, &lp, &m, &t, &o, &dl, &tid, &sz); err != nil {
					continue
				}
				r.ID = id.String
				r.LocalPath = lp.String
				r.Model = m.String
				r.Tag = t.String
				r.Orientation = o.String
				r.DownloadedAt = dl.String
				r.TaskID = tid.String
				r.SizeBytes = sz.Int64
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "", "Filter to assets newer than this (e.g. 7d)")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	cmd.Flags().StringVar(&orientation, "orientation", "", "Filter by orientation (landscape/portrait/square)")
	cmd.Flags().StringVar(&modelFilter, "model", "", "Filter by model slug")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

func newMagnificGalleryOpenCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:   "open <asset-id>",
		Short: "Print the local path for an asset (use --launch to also open it in the OS default app)",
		Long: `open prints the local file path for an asset by default. Pass --launch
to also open it with the OS default application. This command short-
circuits when PRINTING_PRESS_VERIFY=1 so verify and dogfood runs do not
spam your environment.`,
		Example: "  magnific-pp-cli gallery open asset-1234",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			id := args[0]
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			var lp sql.NullString
			err = db.DB().QueryRowContext(ctx, `SELECT local_path FROM magnific_assets WHERE id = ?`, id).Scan(&lp)
			if err != nil {
				if err == sql.ErrNoRows {
					return notFoundErr(fmt.Errorf("asset %q not found", id))
				}
				return err
			}
			out := map[string]any{
				"id":         id,
				"local_path": lp.String,
				"launched":   false,
			}
			if !launch || cliutil.IsVerifyEnv() {
				if cliutil.IsVerifyEnv() {
					out["note"] = "would launch (skipped under PRINTING_PRESS_VERIFY)"
				} else {
					out["note"] = "use --launch to open in default app"
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if err := osOpen(lp.String); err != nil {
				out["launch_error"] = err.Error()
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			out["launched"] = true
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Open the file with the OS default application")
	return cmd
}

func osOpen(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		cmd = exec.Command("cmd", "/C", "start", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Run()
}
