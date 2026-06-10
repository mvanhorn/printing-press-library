package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCampaignsCloneCmd(flags *rootFlags) *cobra.Command {
	var (
		campaignID int
		newSubject string
		sendNow    bool
	)

	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone an existing campaign into a new draft",
		Long: `clone fetches the HTML body, from_name, and from_email of an existing
campaign and creates a new draft from them. Useful for resending a campaign to
a different list or with an updated subject line.`,
		Example: strings.Trim(`
  # Clone campaign 18 and give it a new subject
  sendfox-pp-cli campaigns clone --id 18 --subject "Updated: My best tips for 2026"

  # Clone and assign to a different list
  sendfox-pp-cli campaigns clone --id 18 --list 99`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("id") && !flags.dryRun {
				return fmt.Errorf("required flag \"id\" not set")
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch the source campaign
			srcData, _, err := resolveRead(cmd.Context(), c, flags, "campaigns", false,
				fmt.Sprintf("/campaigns/%d", campaignID), nil, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var src struct {
				ID          int    `json:"id"`
				Title       string `json:"title"`
				Subject     string `json:"subject"`
				PreviewText string `json:"preview_text"`
				HTML        string `json:"html"`
				FromName    string `json:"from_name"`
				FromEmail   string `json:"from_email"`
			}
			if err := json.Unmarshal(srcData, &src); err != nil {
				return fmt.Errorf("parsing campaign: %w", err)
			}

			// Build the clone
			subject := src.Subject
			if newSubject != "" {
				subject = newSubject
			} else {
				subject = "Copy: " + src.Subject
			}

			payload := map[string]any{
				"title":        fmt.Sprintf("Clone of #%d — %s", src.ID, time.Now().Format("Jan 2")),
				"subject":      subject,
				"html":         src.HTML,
				"from_name":    src.FromName,
				"from_email":   src.FromEmail,
				"preview_text": src.PreviewText,
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Cloning campaign #%d: %q\n", src.ID, src.Subject)

			draftData, _, err := c.Post("/campaigns", payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var draft struct {
				ID      int    `json:"id"`
				Subject string `json:"subject"`
			}
			_ = json.Unmarshal(draftData, &draft)

			fmt.Fprintf(w, "Clone created (ID: %d) — subject: %s\n", draft.ID, draft.Subject)

			if sendNow {
				fmt.Fprint(w, "Sending... ")
				_, _, sendErr := c.Post(fmt.Sprintf("/campaigns/%d/send", draft.ID), nil)
				if sendErr != nil {
					return classifyAPIError(sendErr, flags)
				}
				fmt.Fprintln(w, green("Sent!"))
			} else {
				fmt.Fprintf(w, "To edit:   sendfox-pp-cli campaigns update --id %d\n", draft.ID)
				fmt.Fprintf(w, "To send:   sendfox-pp-cli campaigns send --id %d\n", draft.ID)
			}

			if flags.asJSON {
				return printJSONFiltered(w, map[string]any{
					"id":          draft.ID,
					"subject":     draft.Subject,
					"cloned_from": src.ID,
				}, flags)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&campaignID, "id", 0, "ID of the campaign to clone")
	cmd.Flags().StringVar(&newSubject, "subject", "", "New subject line for the clone (defaults to 'Copy: <original subject>')")
	cmd.Flags().BoolVar(&sendNow, "send", false, "Send the cloned campaign immediately")

	return cmd
}
