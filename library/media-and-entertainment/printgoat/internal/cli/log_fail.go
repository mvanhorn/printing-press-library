// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: `log fail` records a failed print outcome against a model
// and (best-effort) its designer, feeding `designer stats` and giving
// `similar` a designer to exclude on the next search. No parent `log`
// command existed in root.go, so it is added here and wired in root.go
// alongside the other novel command families.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

func newNovelLogCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "log",
		Short:       "log subcommands: fail",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelLogFailCmd(flags))
	return cmd
}

func newNovelLogFailCmd(flags *rootFlags) *cobra.Command {
	var flagReason string

	cmd := &cobra.Command{
		Use:         "fail <model-key>",
		Short:       "Record a failed print outcome for a model, so designer stats and future similar searches can learn from it.",
		Example:     `  printgoat-pp-cli log fail printables:3161 --reason "warping on first layer" --agent`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("missing required argument <model-key>\nUsage: %s <model-key> --reason <text>", cmd.CommandPath()))
			}
			source, id, perr := parseModelRef(args[0])
			if perr != nil {
				return usageErr(perr)
			}
			if strings.TrimSpace(flagReason) == "" {
				return usageErr(fmt.Errorf("--reason is required\nUsage: %s <model-key> --reason <text>", cmd.CommandPath()))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Best-effort designer lookup: a failed/unavailable API call must
			// not block recording the outcome the user is trying to log.
			designer := ""
			if detail, derr := fetchModelDetail(ctx, c, source, id); derr == nil && detail != nil && detail.Found {
				designer = detail.Designer
			}

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := store.EnsurePrintgoatNovelSchema(s.DB()); err != nil {
				return fmt.Errorf("preparing local schema: %w", err)
			}

			loggedAt := time.Now().UTC().Format(time.RFC3339)
			if _, err := s.DB().ExecContext(ctx,
				`INSERT INTO printgoat_print_outcomes (source, model_id, designer, outcome, reason, logged_at) VALUES (?, ?, ?, 'fail', ?, ?)`,
				source, id, designer, flagReason, loggedAt,
			); err != nil {
				return fmt.Errorf("recording outcome: %w", err)
			}

			out := map[string]any{
				"source":    source,
				"model_id":  id,
				"designer":  designer,
				"outcome":   "fail",
				"reason":    flagReason,
				"logged_at": loggedAt,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagReason, "reason", "", "Why the print failed (required)")
	return cmd
}
