// Copyright 2026 cleverai-zh. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/cliutil"
	"github.com/spf13/cobra"
)

type tenantCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pass, fail, skip
	Detail string `json:"detail,omitempty"`
}

type tenantReport struct {
	Workspace   string        `json:"workspace,omitempty"`
	ChannelID   string        `json:"channelId,omitempty"`
	OverallPass bool          `json:"overallPass"`
	Checks      []tenantCheck `json:"checks"`
}

func newTenantDoctorCmd(flags *rootFlags) *cobra.Command {
	var (
		channelID   string
		testContact string
	)
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run an SMS-tenant readiness checklist with a single exit code.",
		Long: `Sequences the five Bird endpoints that bot-vendor onboarding depends on
into one report:
  1. SMS channel exists in the workspace
  2. Conversations configuration on that channel
  3. Workspace anti-spam setting
  4. Compliance keywords (HELP, STOP, START)
  5. Messageability probe against --test-contact (optional)

Exits non-zero if any check fails.`,
		Example: `  bird-pp-cli tenant doctor --json
  bird-pp-cli tenant doctor --channel-id ch_sms_1 --test-contact contact_42 --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,1",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return nil
			}
			report := tenantReport{Checks: []tenantCheck{}}
			if cliutil.IsVerifyEnv() {
				report.OverallPass = true
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1. SMS channel
			if channelID == "" {
				channelID = defaultChannelID()
			}
			if channelID == "" {
				// Try to auto-discover an SMS channel.
				if data, err := c.Get("/channels", map[string]string{"kind": "sms", "limit": "1"}); err == nil {
					if rows := parseMessagesEnvelope(data); len(rows) > 0 {
						if id, ok := rows[0]["id"].(string); ok {
							channelID = id
						}
					}
				}
			}
			if channelID == "" {
				report.Checks = append(report.Checks, tenantCheck{Name: "sms-channel", Status: "fail", Detail: "no SMS channel found in workspace"})
			} else {
				report.ChannelID = channelID
				report.Checks = append(report.Checks, tenantCheck{Name: "sms-channel", Status: "pass", Detail: channelID})
			}

			// 2. Conversations configuration
			if channelID != "" {
				if data, err := c.Get(fmt.Sprintf("/channels/%s/conversations-configuration", channelID), nil); err == nil {
					var cfg map[string]any
					_ = json.Unmarshal(data, &cfg)
					enabled, _ := cfg["enabled"].(bool)
					if enabled {
						report.Checks = append(report.Checks, tenantCheck{Name: "conversations-config", Status: "pass"})
					} else {
						report.Checks = append(report.Checks, tenantCheck{Name: "conversations-config", Status: "fail", Detail: "Conversations not enabled on channel"})
					}
				} else {
					report.Checks = append(report.Checks, tenantCheck{Name: "conversations-config", Status: "fail", Detail: err.Error()})
				}
			}

			// 3. Workspace anti-spam
			if data, err := c.Get("/conversation-settings/antispam", nil); err == nil {
				var as map[string]any
				_ = json.Unmarshal(data, &as)
				if enabled, _ := as["enabled"].(bool); enabled {
					report.Checks = append(report.Checks, tenantCheck{Name: "antispam", Status: "pass"})
				} else {
					report.Checks = append(report.Checks, tenantCheck{Name: "antispam", Status: "fail", Detail: "anti-spam disabled"})
				}
			} else {
				report.Checks = append(report.Checks, tenantCheck{Name: "antispam", Status: "fail", Detail: err.Error()})
			}

			// 4. Compliance keywords
			if channelID != "" {
				if data, err := c.Get(fmt.Sprintf("/channels/%s/compliance-keywords", channelID), nil); err == nil {
					rows := parseMessagesEnvelope(data)
					if len(rows) >= 1 {
						report.Checks = append(report.Checks, tenantCheck{Name: "compliance-keywords", Status: "pass", Detail: fmt.Sprintf("%d configured", len(rows))})
					} else {
						report.Checks = append(report.Checks, tenantCheck{Name: "compliance-keywords", Status: "fail", Detail: "no compliance keywords configured"})
					}
				} else {
					report.Checks = append(report.Checks, tenantCheck{Name: "compliance-keywords", Status: "fail", Detail: err.Error()})
				}
			}

			// 5. Messageability probe (optional)
			if channelID != "" && testContact != "" {
				if data, err := c.Get(fmt.Sprintf("/channels/%s/messageability/%s", channelID, testContact), nil); err == nil {
					var m map[string]any
					_ = json.Unmarshal(data, &m)
					if ok, _ := m["messageable"].(bool); ok {
						report.Checks = append(report.Checks, tenantCheck{Name: "messageability", Status: "pass"})
					} else {
						reason, _ := m["reason"].(string)
						report.Checks = append(report.Checks, tenantCheck{Name: "messageability", Status: "fail", Detail: reason})
					}
				} else {
					report.Checks = append(report.Checks, tenantCheck{Name: "messageability", Status: "fail", Detail: err.Error()})
				}
			} else {
				report.Checks = append(report.Checks, tenantCheck{Name: "messageability", Status: "skip", Detail: "set --test-contact to enable"})
			}

			report.OverallPass = true
			for _, ck := range report.Checks {
				if ck.Status == "fail" {
					report.OverallPass = false
					break
				}
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
				return err
			}
			if !report.OverallPass {
				return fmt.Errorf("tenant readiness check failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&channelID, "channel-id", "", "SMS channel ID (auto-discovered when omitted)")
	cmd.Flags().StringVar(&testContact, "test-contact", "", "Optional contact ID for the messageability probe")
	return cmd
}
