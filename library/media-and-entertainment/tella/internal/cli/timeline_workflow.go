// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/cliutil"
	"github.com/spf13/cobra"
)

func printLessonTimeline(cmd *cobra.Command, t lessonTimeline) error {
	fmt.Fprintf(cmd.OutOrStdout(), "%s (%s), %.3fs\n", cliutil.EscapeTerminalControls(t.Video.Name), cliutil.EscapeTerminalControls(t.Video.ID), t.Video.DurationMs/1000)
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "CLIP\tSTART s\tEND s\tDURATION s\tNAME")
	for _, clip := range t.Clips {
		fmt.Fprintf(w, "%s\t%.3f\t%.3f\t%.3f\t%s\n", cliutil.EscapeTerminalControls(clip.ID), clip.TimelineStartMs/1000, (clip.TimelineStartMs+clip.DurationMs)/1000, clip.DurationMs/1000, cliutil.EscapeTerminalControls(clip.Name))
	}
	return w.Flush()
}
