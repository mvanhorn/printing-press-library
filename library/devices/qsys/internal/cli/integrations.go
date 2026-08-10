// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type integrationPage struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type integrationResult struct {
	Model     string            `json:"model"`
	Platforms []string          `json:"platforms"`
	Pages     []integrationPage `json:"pages"`
	Note      string            `json:"note,omitempty"`
}

// ucPlatforms is the conservative set of UC/collaboration platforms whose full
// names are matched against the Application_Integration pages. Only platforms
// literally named in the indexed text are reported - substring traps like a
// bare "teams" or "meet" are avoided by requiring the full product name.
var ucPlatforms = []string{
	"Microsoft Teams",
	"Zoom",
	"Google Meet",
	"Webex",
	"RingCentral",
	"GoToMeeting",
	"Skype",
}

func newNovelIntegrationsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "integrations <model>",
		Short: "Find which UC platforms (Teams, Zoom, Meet) a device is certified or integrated with.",
		Long: strings.Trim(`
Integrations answers the room-design question "will this device work with the
client's chosen UC platform?". It matches a product model against the Q-SYS
Help Application_Integration pages (Microsoft Teams, Zoom Rooms, Google Meet,
Webex, and similar) and reports which platforms the device's pages actually
name, with the matching pages as evidence.

The 34 Application_Integration pages are indexed into the local corpus by
harvest; no page on either QSC site indexes certifications by device, so this
lookup only exists locally.
`, "\n"),
		Example:     "  qsys-pp-cli integrations TSC-70-G3 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true", "pp:happy-args": "model=TSC-70-G3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "integrations")
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a model is required, e.g. TSC-70-G3"))
			}
			model := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), integrationResult{Model: model, Platforms: make([]string, 0), Pages: make([]integrationPage, 0)}, flags)
				}
				return nil
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			db := st.DB()

			// Escape LIKE metacharacters so a model containing '%' or '_' is
			// matched literally, not as a wildcard.
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(model)
			like := "%" + escaped + "%"
			rows, err := db.QueryContext(ctx,
				`SELECT url, title, body FROM qsys_pages
				 WHERE section = 'Application_Integration'
				   AND (title LIKE ? ESCAPE '\' OR body LIKE ? ESCAPE '\')
				 ORDER BY title LIMIT 25`, like, like)
			if err != nil {
				return fmt.Errorf("searching integration pages: %w", err)
			}
			type rawPage struct {
				url, title, body string
			}
			pages := make([]rawPage, 0, 8)
			for rows.Next() {
				var p rawPage
				var title, body sql.NullString
				if err := rows.Scan(&p.url, &title, &body); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning integration page: %w", err)
				}
				p.title, p.body = title.String, body.String
				pages = append(pages, p)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating integration pages: %w", err)
			}
			if err := rows.Close(); err != nil {
				return err
			}

			res := integrationResult{
				Model:     model,
				Platforms: make([]string, 0, 3),
				Pages:     make([]integrationPage, 0, len(pages)),
			}
			joined := strings.ToLower(strings.Join(func() []string {
				out := make([]string, 0, len(pages))
				for _, p := range pages {
					out = append(out, p.title, p.body)
				}
				return out
			}(), "\n"))
			for _, platform := range ucPlatforms {
				if strings.Contains(joined, strings.ToLower(platform)) {
					res.Platforms = append(res.Platforms, platform)
				}
			}
			for _, p := range pages {
				res.Pages = append(res.Pages, integrationPage{Title: p.title, URL: p.url})
			}
			if len(pages) == 0 {
				res.Note = fmt.Sprintf("no Application_Integration pages matched %q; integration pages often name only the series, so try a series name (e.g. TSC) instead of the exact SKU", model)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(res.Platforms) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: no UC platforms matched from the local integration index\n", model)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", model, strings.Join(res.Platforms, ", "))
			}
			for _, p := range res.Pages {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p.Title)
			}
			if res.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", res.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelIntegrationsCmd(flags))
	})
}
