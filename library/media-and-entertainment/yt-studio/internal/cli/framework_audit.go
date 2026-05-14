package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

// FrameworkAudit is the per-video result returned by the framework-audit command.
type FrameworkAudit struct {
	VideoID         string           `json:"video_id"`
	ScriptPath      string           `json:"script_path,omitempty"`
	Title           string           `json:"title,omitempty"`
	Verdict         string           `json:"verdict"` // pass / partial / fail / no-binding
	Signal          string           `json:"signal_line,omitempty"`
	BeliefShift     string           `json:"belief_shift_line,omitempty"`
	CTA             string           `json:"cta_line,omitempty"`
	Missing         []string         `json:"missing,omitempty"`
	Drops           []DropAnnotation `json:"drops,omitempty"`
	RetentionPoints []float64        `json:"retention_points,omitempty"`
	BindingSource   string           `json:"binding_source"` // store / registry / none
	Recommendation  string           `json:"recommendation,omitempty"`
}

func newFrameworkAuditCmd(flags *rootFlags) *cobra.Command {
	var (
		scriptDir    string
		scriptPath   string
		registryPath string
		includeCurve bool
		dbPath       string
	)

	cmd := &cobra.Command{
		Use:   "framework-audit [video_id]",
		Short: "Audit Signal / Belief-Shift / CTA structure of a video by joining retention to script",
		Long: strings.TrimSpace(`
The killer command. Looks up the script bound to a video (via the local
binding table or ~/.openclaw/workspace/data/content-registry.md), extracts
its Signal / Belief-Shift / CTA lines, pulls the latest retention curve from
the local store, and returns a structured verdict:

  pass        all three framework lines found in the script
  partial     1 or 2 of the three lines found
  fail        none of the three lines found
  no-binding  no script could be located for this video_id

When a curve is available the response includes the 3 sharpest drops so the
caller can correlate them with the structural beats.`),
		Example: strings.Trim(`
  # Default: looks for content-registry.md under ~/.openclaw/workspace/data
  yt-studio-pp-cli framework-audit dQw4w9WgXcQ --json

  # Explicit script directory
  yt-studio-pp-cli framework-audit dQw4w9WgXcQ --script-dir ~/.openclaw/workspace/data

  # Pass the script path directly (skip registry lookup)
  yt-studio-pp-cli framework-audit dQw4w9WgXcQ --script ~/my-script.md
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			videoID := args[0]
			ctx := cmd.Context()

			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			audit := FrameworkAudit{VideoID: videoID}

			// 1. Resolve script binding: --script wins; then local binding table; then registry parse.
			source := "none"
			resolvedPath := scriptPath
			if resolvedPath == "" {
				if b, err := ytstore.GetScriptBinding(ctx, db, videoID); err == nil && b != nil && b.ScriptPath != "" {
					resolvedPath = b.ScriptPath
					source = "store"
					audit.Signal = b.SignalLine
					audit.BeliefShift = b.BeliefShiftLine
					audit.CTA = b.CTALine
				}
			}
			if resolvedPath == "" {
				rpath := registryPath
				if rpath == "" {
					rpath = FindRegistryFile(scriptDir)
				}
				if rpath != "" {
					entry, err := LookupRegistryByVideoID(rpath, videoID)
					if err == nil && entry != nil {
						audit.Title = entry.Title
						resolvedPath = ResolveScriptPath(scriptDir, entry)
						if resolvedPath == "" && entry.Title != "" {
							source = "registry-title-only"
						} else if resolvedPath != "" {
							source = "registry"
						}
					}
				}
			}
			audit.ScriptPath = resolvedPath
			audit.BindingSource = source

			// 2. Parse script for framework lines if we have a path and the binding didn't preload them.
			if resolvedPath != "" && audit.Signal == "" && audit.BeliefShift == "" && audit.CTA == "" {
				signal, belief, cta, perr := ExtractFrameworkLines(resolvedPath)
				if perr != nil {
					return fmt.Errorf("parsing script: %w", perr)
				}
				audit.Signal = signal
				audit.BeliefShift = belief
				audit.CTA = cta
			}

			// 3. Verdict
			missing := []string{}
			if audit.Signal == "" {
				missing = append(missing, "signal")
			}
			if audit.BeliefShift == "" {
				missing = append(missing, "belief_shift")
			}
			if audit.CTA == "" {
				missing = append(missing, "cta")
			}
			audit.Missing = missing
			switch {
			case resolvedPath == "" && audit.Title == "":
				audit.Verdict = "no-binding"
				audit.Recommendation = fmt.Sprintf("Run `yt-studio-pp-cli script-link %s <script_path>` to bind a script manually, or add a `**Video ID:** %s` entry under a heading in your content-registry.md.", videoID, videoID)
			case resolvedPath == "" && audit.Title != "":
				audit.Verdict = "no-script-file"
				audit.Recommendation = fmt.Sprintf("Registry has title %q but no script file was found. Run `yt-studio-pp-cli script-link %s <path>` to bind one.", audit.Title, videoID)
			case len(missing) == 0:
				audit.Verdict = "pass"
			case len(missing) == 3:
				audit.Verdict = "fail"
				audit.Recommendation = "Script is present but none of Signal / Belief-Shift / CTA sections were detected. Make sure the script has `## Signal`, `## Belief Shift`, and `## CTA` (or `## Call to Action`) headings."
			default:
				audit.Verdict = "partial"
				audit.Recommendation = fmt.Sprintf("Script is missing: %s. Add the matching `## %s` heading(s) and re-run.", strings.Join(missing, ", "), strings.ToUpper(missing[0]))
			}

			// 4. Latest retention curve (best effort).
			if cur, err := ytstore.LatestRetentionCurve(ctx, db, videoID); err == nil && cur != nil {
				if includeCurve {
					audit.RetentionPoints = cur.Points
				}
				audit.Drops = findSharpestDrops(cur.Points, 3)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, audit)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "framework-audit: %s\n", videoID)
			if audit.Title != "" {
				fmt.Fprintf(w, "  title:    %s\n", audit.Title)
			}
			fmt.Fprintf(w, "  verdict:  %s\n", audit.Verdict)
			fmt.Fprintf(w, "  binding:  %s\n", audit.BindingSource)
			if audit.ScriptPath != "" {
				fmt.Fprintf(w, "  script:   %s\n", audit.ScriptPath)
			}
			fmt.Fprintf(w, "  signal:        %s\n", oneLine(audit.Signal))
			fmt.Fprintf(w, "  belief-shift:  %s\n", oneLine(audit.BeliefShift))
			fmt.Fprintf(w, "  cta:           %s\n", oneLine(audit.CTA))
			if len(audit.Drops) > 0 {
				fmt.Fprintln(w, "  drops:")
				for _, d := range audit.Drops {
					fmt.Fprintf(w, "    %.0f%%: %.3f → %.3f (drop %.3f)\n",
						d.VideoTimeRatio*100, d.BeforeRatio, d.AfterRatio, d.DropMagnitude)
				}
			}
			if audit.Recommendation != "" {
				fmt.Fprintf(w, "  recommendation: %s\n", audit.Recommendation)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&scriptDir, "script-dir", "", "Directory containing content-registry.md and scripts (defaults to ~/.openclaw/workspace/data)")
	cmd.Flags().StringVar(&scriptPath, "script", "", "Path to a specific script file (skips registry lookup)")
	cmd.Flags().StringVar(&registryPath, "registry", "", "Path to content-registry.md (overrides --script-dir resolution)")
	cmd.Flags().BoolVar(&includeCurve, "include-curve", false, "Include the full retention curve in JSON output")
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

func oneLine(s string) string {
	if s == "" {
		return "(missing)"
	}
	if len(s) > 100 {
		return s[:97] + "…"
	}
	return s
}
