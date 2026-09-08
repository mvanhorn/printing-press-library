// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/keywordapi"
	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/portfolio"
	"github.com/spf13/cobra"
)

const plannerEnvFileEnv = "KEYWORD_PLANNER_ENV_FILE"

type plannerDoctorFlags struct {
	dbPath     string
	envFile    string
	customerID string
	live       bool
}

func newPlannerDoctorCmd(flags *rootFlags) *cobra.Command {
	var doctor plannerDoctorFlags
	cmd := &cobra.Command{
		Use:         "doctor",
		Short:       "Check local portfolio health; use --live for an explicit Google Ads read",
		Example:     "  keyword-planner doctor --dry-run --agent\n  keyword-planner doctor --agent\n  keyword-planner doctor --live --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2,3,4,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePlannerRuntimeFlags(flags); err != nil {
				return usageErr(err)
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("unexpected argument %q", args[0]))
			}
			// Preserve the generated platform doctor contract when a provider
			// registration is present. The curated offline doctor is used for
			// this binary's ordinary Google Ads workflow; this branch keeps the
			// framework conformance report and gate error reachable in tests and
			// future provider-bound builds.
			if registeredPlatformSource != nil {
				if flags.platformSession == nil {
					return fmt.Errorf("verified client profile session is required")
				}
				report, err := platformDoctorV2Report(cmd.Context(), flags.platformSession)
				if err != nil {
					return err
				}
				if err := flags.printJSON(cmd, report); err != nil {
					return err
				}
				return flags.platformGateError
			}
			if dryRunOK(flags) {
				return writePlannerDoctorDryRun(cmd, flags, doctor)
			}
			if doctor.live {
				return runPlannerDoctorLive(cmd.Context(), cmd, flags, doctor)
			}
			return runPlannerDoctorOffline(cmd.Context(), cmd, flags, doctor)
		},
	}
	cmd.Flags().StringVar(&doctor.dbPath, "db", "", "Portfolio SQLite path (env KEYWORD_PLANNER_DB is used when omitted)")
	cmd.Flags().StringVar(&doctor.envFile, "env-file", "", "Google Ads env file (env KEYWORD_PLANNER_ENV_FILE is used when omitted)")
	cmd.Flags().StringVar(&doctor.customerID, "customer-id", "", "Operating customer ID for --live (otherwise use the approved binding)")
	cmd.Flags().BoolVar(&doctor.live, "live", false, "Perform an explicit read-only Google Ads account check")
	return cmd
}

func writePlannerDoctorDryRun(cmd *cobra.Command, flags *rootFlags, doctor plannerDoctorFlags) error {
	report := map[string]any{
		"dry_run":  true,
		"action":   "doctor",
		"mode":     map[bool]string{true: "live", false: "offline"}[doctor.live],
		"database": plannerPortfolioDBPath(doctor.dbPath, plannerRootHome(flags)),
	}
	if doctor.live {
		report["env_file"] = plannerEnvFilePath(doctor.envFile, plannerRootHome(flags))
		report["customer_target"] = plannerCustomerTarget(strings.TrimSpace(doctor.customerID))
		if strings.TrimSpace(doctor.customerID) == "" {
			report["customer_target_note"] = "customer target is unresolved until the approved nonsecret binding is loaded"
		}
	}
	return writeKeywordDoctorOutput(cmd, flags, report, "dry-run")
}

func plannerEnvFilePath(explicit string, homeOverrides ...string) string {
	if path := strings.TrimSpace(explicit); path != "" {
		return path
	}
	if path := strings.TrimSpace(os.Getenv(plannerEnvFileEnv)); path != "" {
		return path
	}
	if len(homeOverrides) > 0 {
		if home := strings.TrimSpace(homeOverrides[0]); home != "" {
			return filepath.Join(home, ".env")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ".env"
	}
	return filepath.Join(home, ".env")
}

func runPlannerDoctorOffline(ctx context.Context, cmd *cobra.Command, flags *rootFlags, doctor plannerDoctorFlags) error {
	path := plannerPortfolioDBPath(doctor.dbPath, plannerRootHome(flags))
	report := map[string]any{
		"mode":     "offline",
		"database": path,
		"network":  "not contacted",
		"auth":     "not checked",
	}
	store, missing, err := openPlannerPortfolioReadOnly(ctx, path)
	if err != nil {
		report["database_status"] = "error"
		report["database_error"] = err.Error()
		if outputErr := writeKeywordDoctorOutput(cmd, flags, report, "local"); outputErr != nil {
			return configErr(outputErr)
		}
		return configErr(err)
	}
	if missing {
		report["database_status"] = "missing"
		report["database_hint"] = "run ideas or historical to create the immutable portfolio"
		return writeKeywordDoctorOutput(cmd, flags, report, "local")
	}
	defer store.Close()
	version, err := store.SchemaVersion(ctx)
	if err != nil {
		report["database_status"] = "error"
		report["database_error"] = err.Error()
		if outputErr := writeKeywordDoctorOutput(cmd, flags, report, "local"); outputErr != nil {
			return configErr(outputErr)
		}
		return configErr(err)
	}
	report["database_status"] = "ok"
	report["schema_version"] = version
	return writeKeywordDoctorOutput(cmd, flags, report, "local")
}

func runPlannerDoctorLive(ctx context.Context, cmd *cobra.Command, flags *rootFlags, doctor plannerDoctorFlags) error {
	path := plannerPortfolioDBPath(doctor.dbPath, plannerRootHome(flags))
	envPath := plannerEnvFilePath(doctor.envFile, plannerRootHome(flags))
	report := map[string]any{
		"mode":     "live",
		"database": path,
		"network":  "pending",
		"env_file": envPath,
	}
	config, err := plannerLiveConfig(doctor.envFile, doctor.customerID, flags)
	if err != nil {
		if strings.Contains(err.Error(), "customer target is unresolved") {
			report["auth"] = "configured"
			report["network"] = "not contacted"
			report["customer_target"] = "unresolved"
			report["customer_target_note"] = "supply --customer-id or GOOGLE_ADS_CUSTOMER_ID before --live"
			if outputErr := writeKeywordDoctorOutput(cmd, flags, report, "live"); outputErr != nil {
				return plannerErrorForCLI(outputErr)
			}
			return authErr(err)
		}
		if strings.Contains(err.Error(), "customer-id") || strings.Contains(err.Error(), "rate-limit") {
			return usageErr(err)
		}
		report["auth"] = "config_error"
		report["error_code"] = string(keywordapi.CodeOf(err))
		if outputErr := writeKeywordDoctorOutput(cmd, flags, report, "live"); outputErr != nil {
			return plannerErrorForCLI(outputErr)
		}
		return plannerErrorForCLI(err)
	}
	report["customer_target"] = "resolved"
	report["auth"] = "configured"

	// A live doctor uses the same bounded, receipt-preserving orchestration as
	// the collection commands. Each probe has one account lookup and one
	// Planner request, so both allowlisted methods are exercised without an
	// unbounded pagination or batch walk. The explicit --live flag authorises
	// these read receipts in the configured local portfolio; the default doctor
	// remains entirely offline.
	ctx, cancel := boundCtx(ctx, flags)
	defer cancel()
	store, err := portfolio.Open(ctx, plannerPortfolioDBPath(doctor.dbPath, plannerRootHome(flags)))
	if err != nil {
		report["database_status"] = "error"
		report["database_error"] = err.Error()
		if outputErr := writeKeywordDoctorOutput(cmd, flags, report, "live"); outputErr != nil {
			return configErr(outputErr)
		}
		return configErr(err)
	}
	defer store.Close()

	now := time.Now().UTC()
	window, windowErr := resolvePlannerWindow("", "", now)
	if windowErr != nil {
		if outputErr := writeKeywordDoctorOutput(cmd, flags, report, "live"); outputErr != nil {
			return usageErr(outputErr)
		}
		return usageErr(windowErr)
	}
	target := plannerTarget{
		CustomerID: config.CustomerID,
		Language:   "languageConstants/1000",
		GeoTargets: []string{"geoTargetConstants/2840"},
		Network:    plannerNetworkGoogleSearch,
		Window:     window,
	}
	ideasInput := plannerIdeasInput{
		plannerTarget: target,
		Seeds:         []string{"beef steak", "ribeye steak", "beef brisket", "pork chops", "meat cutting", "butchery"},
		PageSize:      defaultPlannerPageSize,
		MaxPages:      1,
		OutputLimit:   3,
	}
	historicalInput := plannerHistoricalInput{
		plannerTarget: target,
		Keywords:      []string{"beef steak", "beef brisket"},
		BatchSize:     2,
		MaxBatches:    1,
		OutputLimit:   3,
	}
	client := keywordapi.NewClient(config)
	ideas, ideasErr := collectPlannerIdeasWithLogin(ctx, client, store, ideasInput, config.LoginCustomerID, now)
	historical, historicalErr := collectPlannerHistoricalWithLogin(ctx, client, store, historicalInput, config.LoginCustomerID, now)

	report["database_status"] = "ok"
	report["account"] = plannerDoctorAccountReport(ideas, historical)
	report["planner_ideas"] = plannerDoctorCollectionReport(ideas, ideasErr)
	report["planner_historical"] = plannerDoctorCollectionReport(historical, historicalErr)
	report["checked_at"] = time.Now().UTC()
	if ideasErr != nil || historicalErr != nil {
		report["network"] = "error"
		firstErr := ideasErr
		if firstErr == nil {
			firstErr = historicalErr
		}
		report["error_code"] = string(keywordapi.CodeOf(firstErr))
		report["exit_class"] = keywordapi.ExitCodeOf(firstErr)
	} else {
		report["network"] = "ok"
	}
	if outputErr := writeKeywordDoctorOutput(cmd, flags, report, "live"); outputErr != nil {
		return plannerErrorForCLI(outputErr)
	}
	if ideasErr != nil {
		return plannerErrorForCLI(ideasErr)
	}
	if historicalErr != nil {
		return plannerErrorForCLI(historicalErr)
	}
	return nil
}

func plannerDoctorAccountReport(ideas, historical plannerCollectionOutput) map[string]any {
	for _, output := range []plannerCollectionOutput{ideas, historical} {
		if output.Snapshot.CurrencyCode != "" {
			return map[string]any{
				"status":          "ok",
				"customer_id":     output.Snapshot.CustomerID,
				"currency":        output.Snapshot.CurrencyCode,
				"currency_source": output.Snapshot.CurrencySource,
			}
		}
	}
	return map[string]any{"status": "error", "note": "account lookup did not establish verified currency"}
}

func plannerDoctorCollectionReport(output plannerCollectionOutput, err error) map[string]any {
	report := map[string]any{
		"status":               "ok",
		"snapshot_id":          output.Snapshot.ID,
		"stored_keyword_count": output.StoredKeywordCount,
		"stored_monthly_count": output.StoredMonthlyCount,
		"receipt_count":        output.ReceiptCount,
	}
	if err != nil {
		report["status"] = "error"
		report["error"] = err.Error()
		if code := keywordapi.CodeOf(err); code != "" && code != "unknown" {
			report["error_code"] = string(code)
		}
	}
	return report
}

func writeKeywordDoctorOutput(cmd *cobra.Command, flags *rootFlags, value any, source string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode planner doctor output: %w", err)
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": source})
}
