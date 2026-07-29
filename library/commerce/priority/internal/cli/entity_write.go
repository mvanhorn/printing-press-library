// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written generic write surface: create/update any form record with a raw
// JSON body (arbitrary fields, deep insert), plus subform line add/update/delete.
// Spec-generated commands cannot carry arbitrary JSON bodies, so these are
// hand-coded against the generated client (Post/Patch/Delete).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"priority-pp-cli/internal/store"
)

// readBodyInput resolves the JSON body from --data, @file syntax, or stdin.
func readBodyInput(cmd *cobra.Command, data string, useStdin bool) (json.RawMessage, error) {
	var raw []byte
	switch {
	case useStdin:
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		raw = b
	case strings.HasPrefix(data, "@"):
		b, err := os.ReadFile(strings.TrimPrefix(data, "@"))
		if err != nil {
			return nil, fmt.Errorf("reading body file: %w", err)
		}
		raw = b
	case data != "":
		raw = []byte(data)
	default:
		return nil, fmt.Errorf("record body is required: pass --data '<json>', --data @file.json, or --stdin")
	}
	trimmed := json.RawMessage(strings.TrimSpace(string(raw)))
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("body is not valid JSON")
	}
	return trimmed, nil
}

// validateMandatoryFields warns (stderr) about cached-mandatory fields missing
// from a create body. Advisory only — server-side business logic is the truth.
func validateMandatoryFields(ctx context.Context, flags *rootFlags, cmd *cobra.Command, form string, body json.RawMessage) {
	dbPath := defaultDBPath("priority-pp-cli")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	c, err := flags.newClient()
	if err != nil {
		return
	}
	tenant := tenantKeyFromClient(c)
	rows, err := db.DB().QueryContext(ctx,
		`SELECT field FROM pp_meta_fields WHERE tenant = ? AND form = ? AND mandatory = 1`, tenant, form)
	if err != nil {
		return
	}
	var mandatory []string
	for rows.Next() {
		var f string
		if rows.Scan(&f) == nil {
			mandatory = append(mandatory, f)
		}
	}
	_ = rows.Err()
	_ = rows.Close()
	if len(mandatory) == 0 {
		return
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(body, &obj) != nil {
		return
	}
	var missing []string
	for _, f := range mandatory {
		if _, ok := obj[f]; !ok {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: cached schema marks these %s fields mandatory and they are absent from the body: %s\n", form, strings.Join(missing, ", "))
	}
}

func printWriteResult(cmd *cobra.Command, flags *rootFlags, data json.RawMessage, status int) error {
	if len(data) == 0 || string(data) == "null" {
		data = json.RawMessage(fmt.Sprintf(`{"status": %d, "ok": true}`, status))
	}
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	} else if flags.compact {
		data = compactFields(data)
	}
	return printOutput(cmd.OutOrStdout(), data, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
}

func newEntityCreateCmd(flags *rootFlags) *cobra.Command {
	var data string
	var useStdin bool
	cmd := &cobra.Command{
		Use:   "create <form>",
		Short: "Create a record on any form with a raw JSON body (deep insert supported)",
		Long: strings.Trim(`
POSTs the JSON body to the form. Subform arrays in the body perform a deep
insert (parent + children in one call). Field names are case-sensitive
UPPERCASE; check them with 'forms describe <FORM>' or 'forms search <term>'.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli entity create FAMILY_LOG --data '{"FAMILYNAME":"765","FAMILYDESC":"My OData Family"}' --dry-run
  priority-pp-cli entity create ORDERS --data '{"CUSTNAME":"007","ORDERITEMS_SUBFORM":[{"PARTNAME":"TR0001","TQUANT":5}]}'
  cat order.json | priority-pp-cli entity create ORDERS --stdin`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				form := "<form>"
				if len(args) > 0 {
					form = args[0]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would POST a new %s record\n", form)
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form name is required"))
			}
			body, err := readBodyInput(cmd, data, useStdin)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			validateMandatoryFields(ctx, flags, cmd, args[0], body)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, status, err := c.Post(ctx, "/"+args[0], body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printWriteResult(cmd, flags, resp, status)
		},
	}
	cmd.Flags().StringVar(&data, "data", "", "JSON body ('{...}' or @file.json)")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read the JSON body from stdin")
	return cmd
}

func newEntityUpdateCmd(flags *rootFlags) *cobra.Command {
	var data string
	var useStdin bool
	cmd := &cobra.Command{
		Use:   "update <form> <keyspec>",
		Short: "Update a record on any form (PATCH) with a raw JSON body",
		Long: strings.Trim(`
PATCHes the JSON body onto the record. Address records by unique key —
auto-unique keys are read-only in Priority 21.1+. Single key: "'SO17000003'"
(quotes included). Composite: "IVNUM='T9696',IVTYPE='A',DEBIT='D'".`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli entity update FAMILY_LOG "'765'" --data '{"FAMILYDESC":"Updated"}' --dry-run
  priority-pp-cli entity update ORDERS "'SO18000002'" --data '{"DETAILS":"rush"}'`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would PATCH the record")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form and keyspec are required"))
			}
			body, err := readBodyInput(cmd, data, useStdin)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, status, err := c.Patch(ctx, fmt.Sprintf("/%s(%s)", args[0], args[1]), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printWriteResult(cmd, flags, resp, status)
		},
	}
	cmd.Flags().StringVar(&data, "data", "", "JSON body ('{...}' or @file.json)")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read the JSON body from stdin")
	return cmd
}

func newEntitySubformAddCmd(flags *rootFlags) *cobra.Command {
	var data string
	var useStdin bool
	cmd := &cobra.Command{
		Use:   "subform-add <form> <keyspec> <subform>",
		Short: "Add a related line to a record's subform",
		Example: strings.Trim(`
  priority-pp-cli entity subform-add ORDERS "'SO18000002'" ORDERITEMS_SUBFORM --data '{"PARTNAME":"TR0001","TQUANT":5}' --dry-run`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would POST a new subform line")
				return nil
			}
			if len(args) < 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form, keyspec, and subform are required"))
			}
			body, err := readBodyInput(cmd, data, useStdin)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, status, err := c.Post(ctx, fmt.Sprintf("/%s(%s)/%s", args[0], args[1], args[2]), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printWriteResult(cmd, flags, resp, status)
		},
	}
	cmd.Flags().StringVar(&data, "data", "", "JSON body ('{...}' or @file.json)")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read the JSON body from stdin")
	return cmd
}

func newEntitySubformUpdateCmd(flags *rootFlags) *cobra.Command {
	var data string
	var useStdin bool
	cmd := &cobra.Command{
		Use:   "subform-update <form> <keyspec> <subform> <line>",
		Short: "Update one subform line (PATCH) by line key",
		Example: strings.Trim(`
  priority-pp-cli entity subform-update ORDERS "'SO18000002'" ORDERITEMS_SUBFORM 1 --data '{"TQUANT":10}' --dry-run`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would PATCH the subform line")
				return nil
			}
			if len(args) < 4 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form, keyspec, subform, and line are required"))
			}
			body, err := readBodyInput(cmd, data, useStdin)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, status, err := c.Patch(ctx, fmt.Sprintf("/%s(%s)/%s(%s)", args[0], args[1], args[2], args[3]), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printWriteResult(cmd, flags, resp, status)
		},
	}
	cmd.Flags().StringVar(&data, "data", "", "JSON body ('{...}' or @file.json)")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read the JSON body from stdin")
	return cmd
}

func newEntitySubformDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subform-delete <form> <keyspec> <subform> <line>",
		Short: "Delete one subform line by line key",
		Example: strings.Trim(`
  priority-pp-cli entity subform-delete ORDERS "'SO18000002'" ORDERITEMS_SUBFORM 1 --dry-run`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would DELETE the subform line")
				return nil
			}
			if len(args) < 4 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form, keyspec, subform, and line are required"))
			}
			if !flags.yes && !flags.dryRun {
				return usageErr(fmt.Errorf("deleting subform line %s(%s)/%s(%s) is irreversible; re-run with --yes to confirm, or --dry-run to preview", args[0], args[1], args[2], args[3]))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, status, err := c.Delete(ctx, fmt.Sprintf("/%s(%s)/%s(%s)", args[0], args[1], args[2], args[3]))
			if err != nil {
				return classifyDeleteError(err, flags)
			}
			return printWriteResult(cmd, flags, resp, status)
		},
	}
	return cmd
}
