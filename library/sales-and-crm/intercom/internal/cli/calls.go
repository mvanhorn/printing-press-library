// PATCH(intercom-calls-v2-14): the /calls/* surface was not present in
// Intercom OpenAPI 2.13 (which this CLI pins to), so the generator did not
// emit these commands. The endpoints exist in 2.14+; this hand-built subtree
// reaches them with a per-request Intercom-Version: 2.14 override and leaves
// the rest of the CLI on 2.13.

package cli

import "github.com/spf13/cobra"

// callsVersion is the Intercom-Version override applied per request to every
// /calls/* command. Bumping the global pin to 2.14 risks shifting other
// endpoint behavior (e.g. internal-articles, response shapes); the per-request
// override keeps the blast radius scoped.
const callsVersion = "2.14"

func callsHeaderOverrides() map[string]string {
	return map[string]string{"Intercom-Version": callsVersion}
}

func newCallsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "calls",
		Hidden: true,
		Short:  "Phone calls and transcripts (Intercom 2.14+)",
		Long: `Inbound and outbound phone calls. These endpoints exist in
Intercom API 2.14+; this CLI pins to 2.13 by default but per-request bumps to
2.14 for every /calls command. Use 'calls list' to enumerate, 'calls get <id>'
for one call, 'calls transcript <id>' for the turn-by-turn transcript, and
'calls recording <id> --out <path>' to download the audio recording.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCallsListCmd(flags))
	cmd.AddCommand(newCallsGetCmd(flags))
	cmd.AddCommand(newCallsTranscriptCmd(flags))
	cmd.AddCommand(newCallsRecordingCmd(flags))
	return cmd
}
