package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

func newScriptLinkCmd(flags *rootFlags) *cobra.Command {
	var (
		signal      string
		beliefShift string
		cta         string
		autoParse   bool
		dbPath      string
	)

	cmd := &cobra.Command{
		Use:   "script-link [video_id] [script_path]",
		Short: "Manually bind a script file to a video for framework-audit",
		Long: strings.TrimSpace(`
Inserts a row in the local script_videos table linking a video to a script
file. Used when content-registry.md does not already cover the video, or
when framework-audit needs an override.

By default the command also extracts the Signal / Belief-Shift / CTA lines
from the script and caches them. Pass --no-auto-parse to skip extraction
(useful when the script structure is non-standard).`),
		Example: strings.Trim(`
  yt-studio-pp-cli script-link dQw4w9WgXcQ ~/scripts/my-video.md
  yt-studio-pp-cli script-link dQw4w9WgXcQ ~/scripts/my-video.md --signal "..." --belief-shift "..." --cta "..."
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			videoID := args[0]
			rawPath := args[1]
			expanded, err := expandHome(rawPath)
			if err != nil {
				return err
			}
			abs, err := filepath.Abs(expanded)
			if err != nil {
				return err
			}
			info, err := os.Stat(abs)
			if err != nil {
				return notFoundErr(fmt.Errorf("script path %q: %w", abs, err))
			}
			if info.IsDir() {
				return usageErr(fmt.Errorf("%q is a directory; script-link expects a file", abs))
			}

			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			if autoParse && (signal == "" || beliefShift == "" || cta == "") {
				autoSig, autoBs, autoCta, perr := ExtractFrameworkLines(abs)
				if perr == nil {
					if signal == "" {
						signal = autoSig
					}
					if beliefShift == "" {
						beliefShift = autoBs
					}
					if cta == "" {
						cta = autoCta
					}
				}
			}

			if err := ytstore.LinkScript(ctx, db, videoID, abs, signal, beliefShift, cta); err != nil {
				return fmt.Errorf("linking script: %w", err)
			}

			result := map[string]any{
				"video_id":          videoID,
				"script_path":       abs,
				"signal_line":       signal,
				"belief_shift_line": beliefShift,
				"cta_line":          cta,
				"linked":            true,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Linked %s → %s\n", videoID, abs)
			if signal != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  signal:        %s\n", oneLine(signal))
			}
			if beliefShift != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  belief-shift:  %s\n", oneLine(beliefShift))
			}
			if cta != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  cta:           %s\n", oneLine(cta))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&signal, "signal", "", "Signal line (override auto-parse)")
	cmd.Flags().StringVar(&beliefShift, "belief-shift", "", "Belief-shift line (override auto-parse)")
	cmd.Flags().StringVar(&cta, "cta", "", "CTA line (override auto-parse)")
	cmd.Flags().BoolVar(&autoParse, "auto-parse", true, "Auto-extract Signal/Belief-Shift/CTA lines from the script")
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

func expandHome(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}
