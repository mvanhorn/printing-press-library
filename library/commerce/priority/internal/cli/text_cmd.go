// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written text subform surface: read and write Priority HTML text screens
// (TEXT / APPEND / SIGNATURE semantics, v20.0+).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/priorityx"
)

// textSubformFor derives the conventional text-subform name (FORM + "TEXT_SUBFORM").
func textSubformFor(form, override string) string {
	if override != "" {
		return override
	}
	return form + "TEXT_SUBFORM"
}

func newTextCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "text",
		Short:       "Text screens: read and write a record's HTML text subform",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTextGetCmd(flags))
	cmd.AddCommand(newTextSetCmd(flags))
	return cmd
}

func newTextGetCmd(flags *rootFlags) *cobra.Command {
	var subform string
	var raw bool
	cmd := &cobra.Command{
		Use:   "get <form> <keyspec>",
		Short: "Read a record's text screen (HTML converted to plain text by default)",
		Example: strings.Trim(`
  priority-pp-cli text get ORDERS "'SO18000002'"
  priority-pp-cli text get DOCUMENTS_D "DOCNO='SH17000001',TYPE='D'" --subform DOCUMENTSTEXT_SUBFORM
  priority-pp-cli text get ORDERS "'SO18000002'" --raw`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read the text subform")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form and keyspec are required"))
			}
			sub := textSubformFor(args[0], subform)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, fmt.Sprintf("/%s(%s)/%s", args[0], args[1], sub), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var rec map[string]json.RawMessage
			if err := json.Unmarshal(data, &rec); err != nil {
				return fmt.Errorf("parsing text subform (is %s the right subform name? override with --subform): %w", sub, err)
			}
			text := jsonStrField(rec, "TEXT")
			if !raw {
				text = priorityx.StripHTML(text)
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]string{"form": args[0], "subform": sub, "text": text}, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), text)
			return nil
		},
	}
	cmd.Flags().StringVar(&subform, "subform", "", "text subform name (default: <FORM>TEXT_SUBFORM)")
	cmd.Flags().BoolVar(&raw, "raw", false, "print the raw HTML instead of plain text")
	return cmd
}

func newTextSetCmd(flags *rootFlags) *cobra.Command {
	var subform string
	var text string
	var appendText bool
	var signature bool
	cmd := &cobra.Command{
		Use:   "set <form> <keyspec>",
		Short: "Write to a record's text screen (append or replace)",
		Example: strings.Trim(`
  priority-pp-cli text set ORDERS "'SO18000002'" --text "Called customer, confirmed delivery date" --append --dry-run
  priority-pp-cli text set ORDERS "'SO18000002'" --text "<b>rush order</b>" --append --signature`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would PATCH the text subform")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form and keyspec are required"))
			}
			if text == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--text is required"))
			}
			sub := textSubformFor(args[0], subform)
			body := map[string]any{
				"TEXT":      text,
				"APPEND":    appendText,
				"SIGNATURE": signature,
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, status, err := c.Patch(ctx, fmt.Sprintf("/%s(%s)/%s", args[0], args[1], sub), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printWriteResult(cmd, flags, resp, status)
		},
	}
	cmd.Flags().StringVar(&subform, "subform", "", "text subform name (default: <FORM>TEXT_SUBFORM)")
	cmd.Flags().StringVar(&text, "text", "", "text to write (HTML allowed; plain text for NOHTML forms)")
	cmd.Flags().BoolVar(&appendText, "append", false, "append instead of replacing existing text")
	cmd.Flags().BoolVar(&signature, "signature", false, "add a user+date signature line (HTML text forms only)")
	return cmd
}
