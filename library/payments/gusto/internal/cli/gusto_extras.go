// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/gusto/internal/gusto"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if payrolls, _, err := root.Find([]string{"payrolls"}); err == nil && payrolls != nil && payrolls.Name() == "payrolls" {
			addNovelCommandIfAbsent(payrolls, newPayrollsNetPayPreviewCmd(flags))
		}
		group, _, err := root.Find([]string{"reimbursements"})
		if err != nil || group == nil || group.Name() != "reimbursements" {
			group = &cobra.Command{Use: "reimbursements", Short: "Reimbursement helpers for Gusto payrolls"}
			root.AddCommand(group)
		}
		addNovelCommandIfAbsent(group, newReimbursementsSuggestionsCmd(flags))
	})
}

type netPayPreview struct {
	EmployeeUUID   string  `json:"employee_uuid"`
	Gross          float64 `json:"gross"`
	Net            float64 `json:"net_pay_excluding_reimbursements"`
	Reimbursements float64 `json:"reimbursements"`
}

func newPayrollsNetPayPreviewCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	cmd := &cobra.Command{
		Use:         "net-pay-preview",
		Short:       "Preview each employee's gross and net pay on a draft payroll from its saved inputs",
		Example:     "  gusto-pp-cli payrolls net-pay-preview --period-end 2026-11-01",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls net-pay-preview")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			api, err := newGustoAPI(flags)
			if err != nil {
				return err
			}
			d, err := resolveDraft(ctx, api, t)
			if err != nil {
				return gustoErr(err)
			}
			ids := make([]any, 0, len(d.Employees))
			for _, e := range d.Employees {
				ids = append(ids, e.UUID)
			}
			rows := make([]netPayPreview, 0, len(ids))
			if len(ids) > 0 {
				data, err := api.Query(ctx, "BackgroundNetPayPreviewByIds", map[string]any{"draftPayrollId": d.ID, "ids": ids})
				if err != nil {
					return gustoErr(err)
				}
				for _, n := range gusto.List(data, "data", "company", "draftPayroll", "draftEmployeePayrollsByIds") {
					rows = append(rows, netPayPreview{
						EmployeeUUID:   gusto.Str(n, "employee", "id"),
						Gross:          gusto.Num(n, "previewPay", "grossPay", "amount"),
						Net:            gusto.Num(n, "previewPay", "netPayAmount"),
						Reimbursements: gusto.Num(n, "previewPay", "totalReimbursements"),
					})
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No employees on this draft.")
				return nil
			}
			names := map[string]string{}
			for _, e := range d.Employees {
				names[e.UUID] = e.Name
			}
			items := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				items = append(items, map[string]any{"employee": names[r.EmployeeUUID], "gross": money(r.Gross), "reimbursements": money(r.Reimbursements), "net_pay_excl_reimbursements": money(r.Net)})
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	return cmd
}

func newReimbursementsSuggestionsCmd(flags *rootFlags) *cobra.Command {
	var query string
	cmd := &cobra.Command{
		Use:         "suggestions",
		Short:       "List the reimbursement descriptions Gusto suggests for this company",
		Example:     "  gusto-pp-cli reimbursements suggestions --query mile",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reimbursements suggestions")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			api, err := newGustoAPI(flags)
			if err != nil {
				return err
			}
			data, err := api.Query(ctx, "ReimbursementDescriptionSuggestions", map[string]any{"query": query})
			if err != nil {
				return gustoErr(err)
			}
			out := make([]string, 0)
			if raw, ok := gusto.Get(data, "data", "company", "reimbursementDescriptionSuggestions").([]any); ok {
				for _, v := range raw {
					if s, ok := v.(string); ok {
						out = append(out, s)
					}
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Gusto has no saved reimbursement descriptions yet.")
				return nil
			}
			for _, s := range out {
				fmt.Fprintln(cmd.OutOrStdout(), s)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Only suggestions that match this text")
	return cmd
}
