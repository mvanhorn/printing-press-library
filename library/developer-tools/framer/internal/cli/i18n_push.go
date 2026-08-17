// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newI18nPushCmd(flags *rootFlags) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "i18n-push <translations_file>",
		Short: "Push translations from CSV/PO/XLIFF to Framer's localization system",
		Long: strings.Trim(`
Push translations from standard i18n formats (CSV, PO, XLIFF) into Framer's
localization system.

Localization push is planned for a future version.`, "\n"),
		Example: strings.Trim(`
  # Push translations from a CSV file
  framer-pp-cli i18n-push translations.csv --format csv

  # Push from a PO file
  framer-pp-cli i18n-push messages.po --format po

  # Dry-run mode
  framer-pp-cli i18n-push translations.csv --format csv --dry-run`, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Localization push is planned for a future version.")
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "csv", "Translation file format: csv, po, or xliff")

	return cmd
}
