package cli

import (
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/linkedin"
	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/packet"
	"github.com/spf13/cobra"
)

func newApplyPacketCmd() *cobra.Command {
	var dbPath, packetID, privacyRaw, confirmSensitive, confirmApply, confirmNotify, profileURL string
	var override, dryRun, simulateLive, liveLinkedIn, notifyNetwork bool
	cmd := &cobra.Command{
		Use:     "apply-packet",
		Short:   "Apply an approved packet only after privacy preflight, sensitive-change confirmation, and final user approval.",
		Example: "profilepress apply-packet --packet pkt_123 --privacy-status disabled --dry-run --confirm-sensitive APPLY-SENSITIVE",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			p, err := packetByIDOrLatest(db, packetID)
			if err != nil {
				return err
			}
			privacyStatus, err := linkedin.RequirePrivacyPassed(privacyRaw, override)
			if err != nil {
				return err
			}
			sensitive := packet.SensitiveChanges(p.Changes)
			sensitiveStatus := "none"
			if len(sensitive) > 0 {
				sensitiveStatus = "requires-confirmation"
				if confirmSensitive != "APPLY-SENSITIVE" {
					return fmt.Errorf("packet has %d sensitive change(s); pass --confirm-sensitive APPLY-SENSITIVE", len(sensitive))
				}
				sensitiveStatus = "confirmed"
			}
			if !dryRun && confirmApply != "APPLY" {
				return errors.New("non-dry-run apply requires --confirm-apply APPLY")
			}
			notifyStatus := "network-notify-disabled-default"
			if notifyNetwork {
				notifyStatus = "network-notify-requested"
				if confirmNotify != "NOTIFY-NETWORK" {
					return errors.New("--notify-network requires --confirm-notify NOTIFY-NETWORK")
				}
				notifyStatus = "network-notify-confirmed"
			}
			adapter := linkedin.ApplyAdapter(linkedin.NotImplementedAdapter{})
			result := "applied"
			if liveLinkedIn {
				if simulateLive {
					return errors.New("choose either --live-linkedin or --simulate-live, not both")
				}
				changes := make([]linkedin.SectionChange, 0, len(p.Changes))
				for _, ch := range p.Changes {
					changes = append(changes, linkedin.SectionChange{Section: ch.Section, After: ch.After})
				}
				if err := linkedin.RequireLiveApplySupported(changes); err != nil {
					return err
				}
				if !dryRun {
					if profileURL == "" {
						return errors.New("--live-linkedin requires --profile-url")
					}
					adapter = linkedin.ChromeSessionApplyAdapter{ProfileURL: profileURL}
					result = "linkedin-apply-passed"
				} else {
					adapter = linkedin.DryRunAdapter{}
				}
			} else if dryRun || simulateLive {
				adapter = linkedin.DryRunAdapter{}
			}
			appliedCount := 0
			for _, ch := range p.Changes {
				if err := adapter.Apply(ch.Section, ch.After); err != nil {
					if liveLinkedIn && !dryRun && appliedCount > 0 {
						_, _ = db.AddApplyLog(packet.ApplyLog{PacketID: p.ID, PrivacyStatus: string(privacyStatus), SensitiveStatus: sensitiveStatus, Result: fmt.Sprintf("partial-linkedin-apply-failed-after-%d-change(s): %s", appliedCount, ch.Section), DryRun: false, ConfirmationSource: "cli"})
					}
					return err
				}
				appliedCount++
			}
			if dryRun {
				result = "dry-run-passed"
			} else if simulateLive {
				result = "simulated-apply-passed"
			}
			auditDryRun := dryRun || simulateLive
			log, err := db.AddApplyLog(packet.ApplyLog{PacketID: p.ID, PrivacyStatus: string(privacyStatus), SensitiveStatus: sensitiveStatus, Result: result, DryRun: auditDryRun, ConfirmationSource: "cli"})
			if err != nil {
				return err
			}
			return printJSON(map[string]any{"apply_log_id": log.ID, "packet_id": p.ID, "result": result, "privacy_status": privacyStatus, "sensitive_status": sensitiveStatus, "notify_network": notifyNetwork, "notify_status": notifyStatus})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().StringVar(&packetID, "packet", "", "packet ID")
	cmd.Flags().StringVar(&privacyRaw, "privacy-status", "unknown", "disabled|enabled|unknown")
	cmd.Flags().BoolVar(&override, "override-privacy-risk", false, "override failed/unknown privacy status")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "run checks without writing")
	cmd.Flags().BoolVar(&simulateLive, "simulate-live", false, "test-only live adapter")
	cmd.Flags().BoolVar(&liveLinkedIn, "live-linkedin", false, "apply supported sections to LinkedIn using the existing local Chrome session")
	cmd.Flags().StringVar(&profileURL, "profile-url", "", "LinkedIn /in/<slug> profile URL for --live-linkedin")
	cmd.Flags().StringVar(&confirmSensitive, "confirm-sensitive", "", "must equal APPLY-SENSITIVE for sensitive changes")
	cmd.Flags().StringVar(&confirmApply, "confirm-apply", "", "must equal APPLY for non-dry-run apply")
	cmd.Flags().BoolVar(&notifyNetwork, "notify-network", false, "explicitly allow LinkedIn to notify the user's network if the browser exposes that option; default is false")
	cmd.Flags().StringVar(&confirmNotify, "confirm-notify", "", "must equal NOTIFY-NETWORK when --notify-network is set")
	return cmd
}
