// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, parent := range root.Commands() {
			if parent.Name() == "campaigns" {
				addNovelCommandIfAbsent(parent, newCampaignsTestSendCmd(flags))
			}
		}
	})
}

type campaignTestSendVariation struct {
	VariationID string   `json:"variation_id"`
	TemplateID  string   `json:"template_id"`
	Subject     string   `json:"subject"`
	Recipients  []string `json:"recipients"`
	Result      string   `json:"result"`
	Reason      string   `json:"reason,omitempty"`
}

type campaignTestSendResult struct {
	CampaignID string                      `json:"campaign_id"`
	QAVerdict  string                      `json:"qa_verdict"`
	Findings   []map[string]any            `json:"findings"`
	Variations []campaignTestSendVariation `json:"variations"`
	Brand      *brandQAKit                 `json:"brand,omitempty"`
}

func newCampaignsTestSendCmd(flags *rootFlags) *cobra.Command {
	var campaignID, variationID string
	var to []string
	var brandOptions brandQAOptions
	cmd := &cobra.Command{
		Use: "test-send", Short: "Run campaign QA and send template previews to up to five recipients",
		Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(campaignID) == "" {
				return usageErr(fmt.Errorf("--campaign-id is required (campaign or message ID)"))
			}
			recipients, err := campaignTestSendRecipients(to)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// A dry run still needs current campaign and template evidence for QA.
			c.DryRun, c.NoCache = false, true
			qa, err := runCampaignQA(c, campaignID, "", "America/Chicago", nil, brandOptions)
			if err != nil {
				return err
			}
			result := campaignTestSendResult{CampaignID: qa.CampaignID, QAVerdict: qa.Verdict, Findings: qa.Findings, Variations: []campaignTestSendVariation{}, Brand: qa.Brand}
			for _, v := range qa.Variations {
				if variationID == "" || variationID == v.ID {
					result.Variations = append(result.Variations, campaignTestSendVariation{
						VariationID: v.ID, TemplateID: v.TemplateID, Subject: v.Subject, Recipients: recipients,
						Result: "failed", Reason: "not sent",
					})
				}
			}
			if len(result.Variations) == 0 {
				if variationID != "" {
					return notFoundErr(fmt.Errorf("variation %s is not an email variation of campaign %s", variationID, qa.CampaignID))
				}
				return notFoundErr(fmt.Errorf("campaign %s has no email variations", qa.CampaignID))
			}
			var sendErr error
			if qa.Verdict == "fail" {
				sendErr = fmt.Errorf("campaign QA failed; see findings above")
			} else {
				for _, v := range result.Variations {
					if v.TemplateID == "" {
						sendErr = fmt.Errorf("email variation %s has no assigned template", v.VariationID)
						break
					}
				}
			}
			for i := range result.Variations {
				v := &result.Variations[i]
				if sendErr != nil {
					v.Reason = "not sent: " + sendErr.Error()
					continue
				}
				if dryRunOK(flags) {
					v.Result, v.Reason = "dry-run", ""
					continue
				}
				// Four seconds between jobs respects both 1/s burst and 15/m steady limits.
				if i > 0 {
					timer := time.NewTimer(time.Minute / 15)
					select {
					case <-cmd.Context().Done():
						timer.Stop()
						sendErr = cmd.Context().Err()
						v.Reason = "not sent: " + sendErr.Error()
						continue
					case <-timer.C:
					}
				}
				body := jsonAPIBody("template-preview-send-job", map[string]any{"recipients": recipients}, map[string]any{
					"template": map[string]any{"data": map[string]any{"type": "template", "id": v.TemplateID}},
				})
				_, code, err := c.PostCampaignTestSend(cmd.Context(), body)
				switch {
				case code == http.StatusTooManyRequests:
					sendErr = rateLimitErr(fmt.Errorf("Klaviyo daily or rate limit reached; Klaviyo allows 100 test sends a day (burst 1/s, steady 15/m); %w", err))
				case err != nil && (code == 0 || code >= 500):
					sendErr = apiErr(fmt.Errorf("unknown, check the inbox: %w", err))
				case err != nil:
					sendErr = classifyAPIErrorOnly(err)
				case code != http.StatusAccepted:
					sendErr = apiErr(fmt.Errorf("unknown, check the inbox: expected HTTP 202, received HTTP %d", code))
				}
				if sendErr != nil {
					v.Reason = sendErr.Error()
				} else {
					v.Result, v.Reason = "sent", ""
				}
			}
			if err := printCampaignTestSend(cmd, flags, result); err != nil {
				return err
			}
			return sendErr
		},
	}
	cmd.Flags().StringVar(&campaignID, "campaign-id", "", "Campaign or message ID to test")
	cmd.Flags().StringVar(&variationID, "variation-id", "", "Test only this email variation; QA still checks the campaign")
	cmd.Flags().StringArrayVar(&to, "to", nil, "Recipient email addresses (repeatable, comma-separated; defaults to KLAVIYO_TEST_SEND_TO)")
	addBrandQAFlags(cmd, &brandOptions)
	return cmd
}

func campaignTestSendRecipients(to []string) ([]string, error) {
	if len(to) == 0 {
		to = []string{os.Getenv("KLAVIYO_TEST_SEND_TO")}
	}
	recipients := []string{}
	seen := map[string]bool{}
	for _, value := range to {
		for _, email := range strings.Split(value, ",") {
			email = strings.TrimSpace(email)
			if email == "" {
				return nil, fmt.Errorf("provide recipient email addresses with --to or KLAVIYO_TEST_SEND_TO")
			}
			parsed, err := mail.ParseAddress(email)
			_, domain, _ := strings.Cut(email, "@")
			if err != nil || parsed.Address != email || !strings.Contains(domain, ".") || strings.ContainsAny(email, "\r\n\t ") {
				return nil, fmt.Errorf("invalid recipient email address %q", email)
			}
			key := strings.ToLower(email)
			if !seen[key] {
				recipients = append(recipients, email)
				seen[key] = true
			}
		}
	}
	if len(recipients) > 5 {
		return nil, fmt.Errorf("Klaviyo allows at most 5 recipients per test send")
	}
	return recipients, nil
}

func printCampaignTestSend(cmd *cobra.Command, flags *rootFlags, result campaignTestSendResult) error {
	if flags.asJSON || flags.agent || flags.csv || flags.compact || flags.selectFields != "" {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	if flags.quiet {
		return nil
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Campaign %s — QA: %s\n", result.CampaignID, result.QAVerdict)
	for _, f := range result.Findings {
		if f["status"] == "warn" || f["status"] == "fail" {
			fmt.Fprintf(&out, "%s: %v [%v] %v\n", f["status"], f["variation_id"], f["check"], f["detail"])
		}
	}
	for _, v := range result.Variations {
		fmt.Fprintf(&out, "%s: %s; template %s; subject %q; to %s", v.VariationID, v.Result, v.TemplateID, v.Subject, strings.Join(v.Recipients, ", "))
		if v.Reason != "" {
			fmt.Fprintf(&out, "; %s", v.Reason)
		}
		out.WriteByte('\n')
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), terminalText(cmd.OutOrStdout(), out.String()))
	return err
}
