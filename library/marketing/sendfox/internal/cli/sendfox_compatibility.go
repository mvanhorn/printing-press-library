// pp:novel-static-reference: unsupported webhook operations are contract gaps, not simulated API responses.
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/config"
	"github.com/spf13/cobra"
	"io"
	"strings"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		auth, _, _ := root.Find([]string{"auth"})
		auth.AddCommand(newAuthSetTokenCmd(f))
		lists, _, _ := root.Find([]string{"lists"})
		lists.AddCommand(newListsContactsCmd(f))
		root.AddCommand(newWebhooksCmd(f))
	})
}

func newAuthSetTokenCmd(f *rootFlags) *cobra.Command {
	var stdin bool
	cmd := &cobra.Command{Use: "set-token", Short: "Save a personal access token from stdin without echoing it", Example: "  sendfox-pp-cli auth set-token --stdin", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) || cliutil.IsAnyHarness() {
			return writeDryRun(cmd.OutOrStdout(), f, "auth set-token")
		}
		if len(args) != 0 || !stdin {
			return usageErr(fmt.Errorf("use --stdin; tokens in command arguments are not accepted"))
		}
		b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 16385))
		if err != nil {
			return err
		}
		token := strings.TrimSpace(string(b))
		if token == "" || len(b) > 16384 || strings.ContainsAny(token, "\r\n\t ") {
			return usageErr(fmt.Errorf("stdin must contain one token of at most 16384 bytes"))
		}
		cfg, err := config.Load(f.configPath)
		if err != nil {
			return configErr(err)
		}
		if err := cfg.SaveSendfoxToken(token); err != nil {
			return configErr(err)
		}
		return f.printJSON(cmd, map[string]any{"saved": true, "verified": false, "note": "Stored locally; environment token overrides still take precedence."})
	}}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read one personal access token from stdin; never place secrets in command arguments")
	return cmd
}

func newListsContactsCmd(f *rootFlags) *cobra.Command {
	cmd := newListsContactsInCmd(f)
	cmd.Use = "contacts <list_id>"
	cmd.Short = "List members of one list; compatibility alias for lists contacts-in"
	delete(cmd.Annotations, "pp:endpoint")
	cmd.Annotations["mcp:hidden"] = "true"
	return cmd
}

func newWebhooksCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "webhooks", Short: "Explain the public API gap for webhook management and event delivery", Example: "  sendfox-pp-cli webhooks --agent", Annotations: map[string]string{"mcp:hidden": "true", "mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		return f.printJSON(cmd, map[string]any{"supported": false, "api_gap": "No webhook management or event-delivery operation exists in the public SendFox v1.4.0 contract.", "alternative": "Use explicit REST polling and local export comparisons; these do not guarantee event parity."})
	}}
}

func newWorkflowAccountSnapshotCmd(f *rootFlags) *cobra.Command {
	return newSendfoxCompatibilityCmd(f, "account-snapshot")
}
func newWorkflowAudienceMapCmd(f *rootFlags) *cobra.Command {
	return newSendfoxCompatibilityCmd(f, "audience-map")
}
func newWorkflowCampaignDigestCmd(f *rootFlags) *cobra.Command {
	return newSendfoxCompatibilityCmd(f, "campaign-digest")
}
func newWorkflowHygieneReportCmd(f *rootFlags) *cobra.Command {
	return newSendfoxCompatibilityCmd(f, "hygiene-report")
}
func newWorkflowLaunchPlanCmd(f *rootFlags) *cobra.Command {
	return newSendfoxCompatibilityCmd(f, "launch-plan")
}
