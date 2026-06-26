package cli

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"profilepress-pp-cli/internal/packet"
	"profilepress-pp-cli/internal/store"
)

func newProposeForJobCmd() *cobra.Command {
	var dbPath, snapshotID, jobFile, sourceNote string
	var strict bool
	var changes []string
	cmd := &cobra.Command{
		Use:     "propose-for-job",
		Short:   "Create an opportunity-specific change packet from a snapshot, job brief, and proposed section edits.",
		Example: "profilepress propose-for-job --snapshot snap_123 --job-file job.txt --change 'headline=AI Evaluation Lead' --source-note resume",
		RunE: func(cmd *cobra.Command, args []string) error {
			if snapshotID == "" {
				return errors.New("--snapshot is required")
			}
			if strict && sourceNote == "" {
				return errors.New("strict mode requires --source-note")
			}
			if len(changes) == 0 {
				return errors.New("at least one --change section=value is required")
			}
			job := ""
			if jobFile != "" {
				b, err := os.ReadFile(jobFile)
				if err != nil {
					return err
				}
				job = string(b)
			}
			db, err := openStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			snap, err := db.GetSnapshot(snapshotID)
			if err != nil {
				return err
			}
			before := map[string]string{}
			for _, s := range snap.Sections {
				before[strings.ToLower(s.Name)] = s.RawText
			}
			var packetChanges []packet.Change
			for _, raw := range changes {
				k, v, ok := strings.Cut(raw, "=")
				if !ok {
					return errors.New("--change must use section=value")
				}
				section := strings.TrimSpace(k)
				packetChanges = append(packetChanges, packet.Change{Section: section, Before: before[strings.ToLower(section)], After: strings.TrimSpace(v), SourceNote: sourceNote})
			}
			p, err := db.CreatePacket(packet.Packet{SnapshotID: snap.ID, Opportunity: job, OpportunityID: store.NewID("opp"), RiskNotes: riskNotes(packetChanges), Changes: packetChanges})
			if err != nil {
				return err
			}
			return printJSON(map[string]any{"packet_id": p.ID, "snapshot_id": p.SnapshotID, "changes": len(p.Changes), "risk_notes": p.RiskNotes})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().StringVar(&snapshotID, "snapshot", "", "snapshot ID")
	cmd.Flags().StringVar(&jobFile, "job-file", "", "job description file")
	cmd.Flags().StringArrayVar(&changes, "change", nil, "section=value change; repeatable")
	cmd.Flags().StringVar(&sourceNote, "source-note", "", "truth/source note for changes")
	cmd.Flags().BoolVar(&strict, "strict", false, "require source note")
	return cmd
}

func riskNotes(changes []packet.Change) []string {
	seen := map[string]bool{}
	for _, ch := range packet.SensitiveChanges(changes) {
		seen["sensitive LinkedIn profile field changed: "+ch.Section] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}
