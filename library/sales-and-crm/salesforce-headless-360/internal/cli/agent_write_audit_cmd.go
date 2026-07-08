package cli

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"

	"github.com/spf13/cobra"
)

func newAgentWriteAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "write-audit",
		Short: "Inspect local signed write audit rows",
	}
	cmd.AddCommand(newAgentWriteAuditListCmd(flags))
	cmd.AddCommand(newAgentWriteAuditInspectCmd(flags))
	cmd.AddCommand(newAgentWriteAuditVerifyCmd(flags))
	return cmd
}

func newAgentWriteAuditListCmd(flags *rootFlags) *cobra.Command {
	var sinceRaw, sobject, status, kid string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local write audit rows",
		RunE: func(cmd *cobra.Command, args []string) error {
			var since time.Time
			var err error
			if sinceRaw != "" {
				since, err = time.Parse(time.RFC3339, sinceRaw)
				if err != nil {
					return fmt.Errorf("--since must be RFC3339: %w", err)
				}
			}
			rows, err := listWriteAuditRows(store.WriteAuditFilter{
				TargetSObject:   strings.TrimSpace(sobject),
				ExecutionStatus: strings.TrimSpace(status),
				Limit:           writeAuditFetchLimit(limit, sinceRaw, kid),
			}, since, strings.TrimSpace(kid), limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			return printWriteAuditList(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&sinceRaw, "since", "", "Only include rows generated at or after this RFC3339 timestamp")
	cmd.Flags().StringVar(&sobject, "sobject", "", "Filter by Salesforce object API name")
	cmd.Flags().StringVar(&status, "status", "", "Filter by execution status: executed, rejected, conflict, or pending")
	cmd.Flags().StringVar(&kid, "kid", "", "Filter by signing key id")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows to return")
	return cmd
}

func newAgentWriteAuditInspectCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <jti>",
		Short: "Inspect one local write audit row",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			row, err := getWriteAuditRow(args[0])
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, row)
			}
			return printWriteAuditInspect(cmd, row)
		},
	}
	return cmd
}

func newAgentWriteAuditVerifyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <jti>",
		Short: "Verify one local write audit row's signed write intent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			row, err := getWriteAuditRow(args[0])
			if err != nil {
				return err
			}
			result := verifyWriteAuditRow(row, time.Now().UTC())
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, result)
			}
			printWriteAuditVerify(cmd, result)
			return nil
		},
	}
	return cmd
}

type writeAuditVerifyResult struct {
	JTI                string `json:"jti"`
	KID                string `json:"kid,omitempty"`
	SignatureValid     bool   `json:"signature_valid"`
	Audience           string `json:"audience,omitempty"`
	AudienceValid      bool   `json:"audience_valid"`
	Expired            bool   `json:"expired"`
	NotExpired         bool   `json:"not_expired"`
	PlanJTI            string `json:"plan_jti,omitempty"`
	PlanSignatureValid *bool  `json:"plan_signature_valid,omitempty"`
	Error              string `json:"error,omitempty"`
}

func openWriteAuditStore() (*store.Store, error) {
	return store.Open(defaultDBPath("salesforce-headless-360-pp-cli"))
}

func listWriteAuditRows(filter store.WriteAuditFilter, since time.Time, kid string, limit int) ([]store.WriteAuditRow, error) {
	if limit <= 0 {
		limit = 50
	}
	db, err := openWriteAuditStore()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.ListWriteAudit(filter)
	if err != nil {
		return nil, err
	}
	out := make([]store.WriteAuditRow, 0, len(rows))
	for _, row := range rows {
		if kid != "" && row.ActingKID != kid {
			continue
		}
		if !since.IsZero() {
			generatedAt, err := time.Parse(time.RFC3339, row.GeneratedAt)
			if err != nil || generatedAt.Before(since) {
				continue
			}
		}
		out = append(out, row)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func writeAuditFetchLimit(limit int, sinceRaw string, kid string) int {
	if limit <= 0 {
		limit = 50
	}
	if sinceRaw != "" || strings.TrimSpace(kid) != "" {
		return max(limit*10, 1000)
	}
	return limit
}

func getWriteAuditRow(jti string) (store.WriteAuditRow, error) {
	db, err := openWriteAuditStore()
	if err != nil {
		return store.WriteAuditRow{}, err
	}
	defer db.Close()
	row, err := db.GetWriteAudit(jti)
	if errors.Is(err, sql.ErrNoRows) {
		return store.WriteAuditRow{}, fmt.Errorf("write audit row %q not found", jti)
	}
	return row, err
}

func printWriteAuditList(cmd *cobra.Command, rows []store.WriteAuditRow) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "jti\tgenerated_at\toperation\tsobject\ttarget_record_id\texecution_status\tacting_kid")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			shortID(row.JTI),
			row.GeneratedAt,
			row.Operation,
			row.TargetSObject,
			row.TargetRecordID,
			row.ExecutionStatus,
			shortID(row.ActingKID),
		)
	}
	return tw.Flush()
}

func printWriteAuditInspect(cmd *cobra.Command, row store.WriteAuditRow) error {
	header, claims, _ := decodeCompactJWS(row.IntentJWS)
	fieldDiff := decodeAuditJSON(row.FieldDiff)
	fmt.Fprintf(cmd.OutOrStdout(), "jti: %s\n", row.JTI)
	fmt.Fprintf(cmd.OutOrStdout(), "generated_at: %s\n", row.GeneratedAt)
	fmt.Fprintf(cmd.OutOrStdout(), "operation: %s\n", row.Operation)
	fmt.Fprintf(cmd.OutOrStdout(), "sobject: %s\n", row.TargetSObject)
	fmt.Fprintf(cmd.OutOrStdout(), "target_record_id: %s\n", row.TargetRecordID)
	fmt.Fprintf(cmd.OutOrStdout(), "execution_status: %s\n", row.ExecutionStatus)
	if row.ExecutionError != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "execution_error: %s\n", row.ExecutionError)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "intent_jws:")
	fmt.Fprintln(cmd.OutOrStdout(), "  header:")
	printIndentedJSON(cmd, header, "    ")
	fmt.Fprintln(cmd.OutOrStdout(), "  claims:")
	printIndentedJSON(cmd, claims, "    ")
	fmt.Fprintln(cmd.OutOrStdout(), "field_diff:")
	printIndentedJSON(cmd, fieldDiff, "  ")
	if planJTI := planJTIFromFieldDiff(fieldDiff); planJTI != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "plan_jti: %s\n", planJTI)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "after_state: unavailable in local write_audit_local mirror")
	return nil
}

func verifyWriteAuditRow(row store.WriteAuditRow, now time.Time) writeAuditVerifyResult {
	result := writeAuditVerifyResult{JTI: row.JTI, KID: row.ActingKID}
	claims, err := trust.VerifyWriteIntent([]byte(row.IntentJWS))
	if err == nil || errors.Is(err, trust.ErrIntentExpired) || errors.Is(err, trust.ErrWrongAudience) {
		result.SignatureValid = true
		result.Audience = claims.Aud
		result.AudienceValid = claims.Aud == trust.WriteIntentAudience
		result.Expired = claims.Exp <= now.Unix()
		result.NotExpired = !result.Expired
	} else {
		_, decodedClaims, _ := decodeCompactJWS(row.IntentJWS)
		result.Audience, _ = decodedClaims["aud"].(string)
		result.AudienceValid = result.Audience == trust.WriteIntentAudience
		result.Error = err.Error()
	}
	if result.KID == "" {
		if kid, err := trust.ExtractKIDUnsafe(row.IntentJWS); err == nil {
			result.KID = kid
		}
	}
	fieldDiff := decodeAuditJSON(row.FieldDiff)
	result.PlanJTI = planJTIFromFieldDiff(fieldDiff)
	if result.PlanJTI != "" {
		valid := result.SignatureValid && result.PlanJTI == row.JTI
		result.PlanSignatureValid = &valid
	}
	return result
}

func printWriteAuditVerify(cmd *cobra.Command, result writeAuditVerifyResult) {
	fmt.Fprintf(cmd.OutOrStdout(), "jti: %s\n", result.JTI)
	if result.KID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "kid: %s\n", result.KID)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "signature_valid: %t\n", result.SignatureValid)
	fmt.Fprintf(cmd.OutOrStdout(), "audience: %s\n", result.Audience)
	fmt.Fprintf(cmd.OutOrStdout(), "audience_valid: %t\n", result.AudienceValid)
	fmt.Fprintf(cmd.OutOrStdout(), "not_expired: %t\n", result.NotExpired)
	if result.PlanJTI != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "plan_jti: %s\n", result.PlanJTI)
		fmt.Fprintf(cmd.OutOrStdout(), "plan_signature_valid: %t\n", *result.PlanSignatureValid)
	}
	if result.Error != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "error: %s\n", result.Error)
	}
}

func decodeCompactJWS(jws string) (map[string]any, map[string]any, error) {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return map[string]any{}, map[string]any{}, fmt.Errorf("invalid JWS")
	}
	header, err := decodeJWSJSONSegment(parts[0])
	if err != nil {
		return map[string]any{}, map[string]any{}, err
	}
	claims, err := decodeJWSJSONSegment(parts[1])
	if err != nil {
		return header, map[string]any{}, err
	}
	return header, claims, nil
}

func decodeJWSJSONSegment(segment string) (map[string]any, error) {
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeAuditJSON(raw string) map[string]any {
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func printIndentedJSON(cmd *cobra.Command, value any, prefix string) {
	data, err := json.MarshalIndent(value, prefix, "  ")
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "%s{}\n", prefix)
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func planJTIFromFieldDiff(fieldDiff map[string]any) string {
	value, _ := fieldDiff["plan_jti"].(string)
	return value
}
