// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/shield"
	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source auto
func newShieldCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shield",
		Short: "Privacy firewall for masking data before frontier-model calls",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newShieldScanCmd(flags))
	cmd.AddCommand(newShieldRedactCmd(flags))
	cmd.AddCommand(newShieldRestructureCmd(flags))
	cmd.AddCommand(newShieldIngestCmd(flags))
	cmd.AddCommand(newShieldAskCmd(flags))
	cmd.AddCommand(newShieldAuditCmd(flags))
	return cmd
}

func newShieldScanCmd(flags *rootFlags) *cobra.Command {
	var maxRisk int
	cmd := &cobra.Command{
		Use:         "scan <file>",
		Short:       "Detect PII locally without an upstream call",
		Example:     `  mixlayer-pp-cli shield scan customer-export.csv --max-risk 0 --json`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,6"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			text, err := readRequiredFile(args[0])
			if err != nil {
				return err
			}
			entities := shield.Detect(text)
			result := map[string]any{"risk": shield.RiskScore(entities), "entities": entities}
			if err := outputJSON(cmd, result); err != nil {
				return err
			}
			if shield.RiskScore(entities) > maxRisk {
				return partialFailureErr(fmt.Errorf("PII risk %d exceeds --max-risk %d", shield.RiskScore(entities), maxRisk))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxRisk, "max-risk", 0, "Exit non-zero when the per-entity max risk exceeds this value (not volume-weighted)")
	return cmd
}

func newShieldRedactCmd(flags *rootFlags) *cobra.Command {
	var dbPath, output string
	var diff bool
	cmd := &cobra.Command{
		Use:         "redact <file>",
		Short:       "Mask a file and populate the local vault",
		Example:     `  mixlayer-pp-cli shield redact customer-export.csv --diff -o masked.csv`,
		Annotations: map[string]string{"mcp:hidden": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			text, err := readRequiredFile(args[0])
			if err != nil {
				return err
			}
			res, err := shield.Redact(cmd.Context(), s, text, diff)
			if err != nil {
				return err
			}
			if output != "" {
				return os.WriteFile(output, []byte(res.Text), 0o600)
			}
			if diff || flags.asJSON || flags.agent {
				return outputJSON(cmd, res)
			}
			fmt.Fprint(cmd.OutOrStdout(), res.Text)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write masked copy to file")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include tokenization details")
	return cmd
}

func newShieldRestructureCmd(flags *rootFlags) *cobra.Command {
	var output, coarsenDates string
	var bucket bool
	var drop []string
	cmd := &cobra.Command{
		Use:         "restructure <file>",
		Short:       "Coarsen values to reduce re-identification risk",
		Example:     `  mixlayer-pp-cli shield restructure customers.csv --bucket --coarsen-dates month --drop email -o safe.csv`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			text, err := readRequiredFile(args[0])
			if err != nil {
				return err
			}
			out, err := shield.Restructure(text, shield.RestructureOptions{BucketNumerics: bucket, CoarsenDates: coarsenDates, DropColumns: drop})
			if err != nil {
				return err
			}
			if output != "" {
				return os.WriteFile(output, []byte(out), 0o600)
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&bucket, "bucket", false, "Bucket numeric values into ranges")
	cmd.Flags().StringVar(&coarsenDates, "coarsen-dates", "", "Coarsen ISO dates to month or quarter")
	cmd.Flags().StringSliceVar(&drop, "drop", nil, "CSV column to drop (repeatable)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write restructured copy to file")
	return cmd
}

func newShieldIngestCmd(flags *rootFlags) *cobra.Command {
	var dbPath, output, manifestPath string
	var maxBytes int
	cmd := &cobra.Command{
		Use:         "ingest <bigfile>",
		Short:       "Split, redact, and reassemble a large corpus with one shared vault",
		Example:     `  mixlayer-pp-cli shield ingest big-export.csv -o masked-corpus.csv --manifest tranches.json`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			text, err := readRequiredFile(args[0])
			if err != nil {
				return err
			}
			tranches, err := shield.SplitRecords(text, maxBytes)
			if err != nil {
				return err
			}
			var b strings.Builder
			manifest := shield.TrancheManifest{}
			for _, tr := range tranches {
				res, err := shield.Redact(cmd.Context(), s, tr.Text, false)
				if err != nil {
					return err
				}
				b.WriteString(res.Text)
				kinds := map[string]int{}
				for _, e := range res.Entities {
					kinds[e.Kind]++
				}
				manifest.Tranches = append(manifest.Tranches, shield.TrancheSummary{
					Index: tr.Index, FromLine: tr.FromLine, ToLine: tr.ToLine,
					EntityCount: len(res.Entities), EntityKinds: kinds,
				})
			}
			if manifestPath != "" {
				if err := os.WriteFile(manifestPath, shield.ManifestJSON(manifest), 0o600); err != nil {
					return err
				}
			}
			return writeShieldIngestResult(cmd, flags, output, b.String(), map[string]any{"tranches": len(tranches), "manifest": manifest, "output": output})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write masked corpus to file")
	cmd.Flags().StringVar(&manifestPath, "manifest", "", "Write tranche manifest to file")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", 64000, "Maximum bytes per tranche")
	return cmd
}

func writeShieldIngestResult(cmd *cobra.Command, flags *rootFlags, output, corpus string, summary map[string]any) error {
	if output != "" {
		if err := os.WriteFile(output, []byte(corpus), 0o600); err != nil {
			return err
		}
		return outputJSON(cmd, summary)
	}
	if flags.asJSON || flags.agent {
		summary["masked_corpus"] = corpus
		return outputJSON(cmd, summary)
	}
	fmt.Fprint(cmd.OutOrStdout(), corpus)
	return nil
}

func newShieldAskCmd(flags *rootFlags) *cobra.Command {
	var dbPath, dataFile, model, guard string
	var showThinking, force bool
	cmd := &cobra.Command{
		Use:         "ask <question>",
		Short:       "Ask a frontier model using only masked local data",
		Example:     `  mixlayer-pp-cli shield ask "What segments are at risk?" --data customer-export.csv --json`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dataFile == "" {
				return usageErr(fmt.Errorf("--data is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			data, err := readRequiredFile(dataFile)
			if err != nil {
				return err
			}
			prepared, err := prepareShieldAskPayload(cmd.Context(), s, data, args[0])
			if err != nil {
				return err
			}
			if len(prepared.Leaks) > 0 && !force {
				return partialFailureErr(fmt.Errorf("masked payload still contains %d detector hits; use --force to send anyway", len(prepared.Leaks)))
			}
			run, err := chatAndSave(cmd.Context(), flags, s, "shield ask", prepared.Payload, model, 0, showThinking)
			if err != nil {
				return err
			}
			answer, err := shield.Rehydrate(cmd.Context(), s, run.Answer)
			if err != nil {
				return err
			}
			rehydratedRun, err := rehydrateRunForOutput(cmd.Context(), s, run, answer)
			if err != nil {
				return err
			}
			audit := store.AuditRecord{
				ID: store.NewID("audit"), RunID: run.ID, Command: "shield ask",
				PayloadSHA256: sha256Hex([]byte(prepared.Payload)), ByteCount: len(prepared.Payload), Model: model, GuardModel: guard,
				MaskedEntities: prepared.MaskedEntities, LeakedPIICount: len(prepared.Leaks), CostUSD: run.CostUSD,
			}
			if err := s.SaveAudit(cmd.Context(), audit); err != nil {
				return err
			}
			if flags.asJSON || flags.agent {
				return outputJSON(cmd, map[string]any{"answer": answer, "run": rehydratedRun, "audit": audit})
			}
			fmt.Fprintln(cmd.OutOrStdout(), answer)
			fmt.Fprintf(cmd.ErrOrStderr(), "privacy receipt %s\n", audit.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&dataFile, "data", "", "File containing local data")
	cmd.Flags().StringVar(&model, "model", defaultFrontierModel, "Frontier model")
	cmd.Flags().StringVar(&guard, "guard", defaultGuardModel, "Guard model label recorded in receipts; local detectors enforce the tripwire")
	cmd.Flags().BoolVar(&showThinking, "show-thinking", false, "Request reasoning_content")
	cmd.Flags().BoolVar(&force, "force", false, "Send even if the outbound tripwire detects residual PII")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

type shieldAskPayload struct {
	Payload        string
	MaskedEntities int
	Leaks          []shield.Entity
}

func prepareShieldAskPayload(ctx context.Context, s *store.Store, data, question string) (shieldAskPayload, error) {
	redactedData, err := shield.Redact(ctx, s, data, false)
	if err != nil {
		return shieldAskPayload{}, err
	}
	redactedQuestion, err := shield.Redact(ctx, s, question, false)
	if err != nil {
		return shieldAskPayload{}, err
	}
	payload := fmt.Sprintf("Use this masked corpus:\n\n%s\n\nQuestion: %s", redactedData.Text, redactedQuestion.Text)
	return shieldAskPayload{
		Payload:        payload,
		MaskedEntities: len(redactedData.Entities) + len(redactedQuestion.Entities),
		Leaks:          shield.Detect(payload),
	}, nil
}

func rehydrateRunForOutput(ctx context.Context, s *store.Store, run store.RunRecord, answer string) (store.RunRecord, error) {
	rehydrated := run
	rehydrated.Answer = answer
	if rehydrated.Reasoning != "" {
		reasoning, err := shield.Rehydrate(ctx, s, rehydrated.Reasoning)
		if err != nil {
			return store.RunRecord{}, err
		}
		rehydrated.Reasoning = reasoning
	}
	return rehydrated, nil
}

func newShieldAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "audit [id]",
		Short:       "Show privacy receipts",
		Example:     `  mixlayer-pp-cli shield audit --limit 25 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			id := ""
			if len(args) > 0 {
				id = args[0]
			}
			rows, err := s.AuditRecords(cmd.Context(), id, limit)
			if err != nil {
				return err
			}
			return outputJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum receipts")
	return cmd
}
