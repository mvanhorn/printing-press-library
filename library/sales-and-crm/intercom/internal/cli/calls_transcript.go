// PATCH(intercom-calls-v2-14): see calls.go.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCallsTranscriptCmd(flags *rootFlags) *cobra.Command {
	var renderText bool
	cmd := &cobra.Command{
		Use:   "transcript <id>",
		Short: "Fetch the turn-by-turn transcript for a call",
		Long: `Returns the call's transcript as a JSON array of turns. Each turn carries
start_time, end_time, speaker_label, speaker, and content. Pass --text to render
as a human-readable transcript (speaker: content, one line per turn) instead.`,
		Example: `  # JSON array of turns
  intercom-pp-cli calls transcript 35553520 --json

  # Human-readable transcript
  intercom-pp-cli calls transcript 35553520 --text`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			callID := strings.TrimSpace(args[0])
			if callID == "" {
				return usageErr(fmt.Errorf("call id is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GetWithHeaders(cmd.Context(), "/calls/"+callID+"/transcript", nil, callsHeaderOverrides())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if renderText {
				return renderCallTranscriptText(cmd, data)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().BoolVar(&renderText, "text", false, "Render as human-readable speaker: content lines instead of JSON")
	return cmd
}

// transcriptTurn is the per-turn shape Intercom returns. Mirrored against the
// /calls/{id}/transcript probe; fields are documented in calls.go.
type transcriptTurn struct {
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	SpeakerLabel string `json:"speaker_label"`
	Speaker      string `json:"speaker"`
	Content      string `json:"content"`
}

func renderCallTranscriptText(cmd *cobra.Command, data []byte) error {
	var turns []transcriptTurn
	if err := json.Unmarshal(data, &turns); err != nil {
		// Empty transcript ("[]") or unexpected shape: passthrough the raw
		// body so the user sees something actionable rather than a parse
		// error.
		_, werr := cmd.OutOrStdout().Write(data)
		return werr
	}
	if len(turns) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(transcript is empty)")
		return nil
	}
	for _, t := range turns {
		speaker := t.SpeakerLabel
		if speaker == "" {
			speaker = t.Speaker
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%ss] %s: %s\n", t.StartTime, speaker, t.Content)
	}
	return nil
}
