// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelWorkflowCampaignReviewCmd(flags *rootFlags) *cobra.Command {
	o := &sendfoxEvidenceOptions{}
	cmd := &cobra.Command{Use: "campaign-review", Short: "Benchmark campaign counters and assemble an auditable resend plan.", Example: "  sendfox-pp-cli workflow campaign-review --input examples/snapshot.json --agent"}
	name := "campaign-review"

	cmd.Flags().StringVar(&o.input, "input", "", "Version 1 local JSON/YAML evidence snapshot, or Markdown YAML frontmatter")
	cmd.Flags().StringVar(&o.dbPath, "db", "", "Local SQLite mirror created by sync; no network refresh")
	cmd.Flags().StringVar(&o.id, "id", "", "Contact or campaign ID to select from the supplied evidence")
	cmd.Flags().StringVar(&o.email, "email", "", "Exact normalized email for a single contact dossier")
	cmd.Flags().IntVar(&o.limit, "limit", 100, "Maximum findings, activity, links and recipient samples printed; counts remain complete")
	if name == "migration-readiness" {
		cmd.Flags().StringVar(&o.kit, "kit", "", "Normalized Kit JSON/YAML snapshot, including explicit suppression evidence")
		cmd.Flags().StringVar(&o.previous, "previous", "", "Previous Kit snapshot used to compute the late-change delta")
	}
	if name == "campaign-review" || name == "campaign-digest" {
		cmd.Flags().StringVar(&o.audience, "audience", "non_openers", "Documented engagement cohort: non_openers, openers or clickers")
	}
	return configureSendfoxEvidenceCmd(cmd, flags, name, o)
}
