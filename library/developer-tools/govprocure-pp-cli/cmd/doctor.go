package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

type checkResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
	Latency string `json:"latency_ms,omitempty"`
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate all APIs and SQLite database health",
		Long:  `Checks connectivity to grants.gov, SAM.gov, and USASpending.gov, and verifies the local SQLite database.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			var checks []checkResult
			allOK := true

			// Check SQLite
			checks = append(checks, checkSQLite(cfg.DBPath))

			// Check grants.gov
			checks = append(checks, checkGrantsGov(cfg.GrantsURL))

			// Check SAM.gov
			checks = append(checks, checkSAMGov(cfg.SAMURL, cfg.SAMAPIKey))

			// Check USASpending.gov
			checks = append(checks, checkUSASpending(cfg.USASpendURL))

			for _, c := range checks {
				if c.Status != "OK" {
					allOK = false
				}
			}

			fmt := output.EffectiveFormat(GlobalOpts)
			switch fmt {
			case output.FormatJSON:
				result := map[string]interface{}{
					"checks": checks,
					"all_ok": allOK,
				}
				output.PrintJSON(result)
			default:
				printDoctorTable(checks, allOK)
			}

			if !allOK {
				return &exitError{code: output.ExitAPI, msg: "one or more checks failed"}
			}
			return nil
		},
	}
}

func checkSQLite(dbPath string) checkResult {
	start := time.Now()
	database, err := db.Open(dbPath)
	if err != nil {
		return checkResult{Name: "SQLite", Status: "FAIL", Detail: err.Error()}
	}
	defer database.Close()

	if err := database.HealthCheck(); err != nil {
		return checkResult{Name: "SQLite", Status: "FAIL", Detail: err.Error(), Latency: ms(start)}
	}
	return checkResult{Name: "SQLite", Status: "OK", Detail: dbPath, Latency: ms(start)}
}

func checkGrantsGov(baseURL string) checkResult {
	start := time.Now()
	reqBody := map[string]interface{}{
		"keyword":        "technology",
		"rows":           1,
		"startRecordNum": 0,
		"oppStatuses":    "posted",
	}
	body, _ := json.Marshal(reqBody)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(baseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return checkResult{Name: "grants.gov", Status: "FAIL", Detail: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return checkResult{Name: "grants.gov", Status: "FAIL", Detail: goFmt("HTTP %d", resp.StatusCode), Latency: ms(start)}
	}
	return checkResult{Name: "grants.gov", Status: "OK", Detail: baseURL, Latency: ms(start)}
}

func checkSAMGov(baseURL, apiKey string) checkResult {
	if apiKey == "" {
		return checkResult{Name: "SAM.gov", Status: "WARN", Detail: "no API key — run: govprocure-pp-cli auth set-key --sam KEY"}
	}
	start := time.Now()
	// SAM.gov requires postedFrom date; use a broad recent window for health check
	url := baseURL + "?api_key=" + apiKey + "&q=technology&limit=1&postedFrom=01/01/2026&postedTo=12/31/2026"
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return checkResult{Name: "SAM.gov", Status: "FAIL", Detail: err.Error()}
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return checkResult{Name: "SAM.gov", Status: "OK", Detail: "authenticated", Latency: ms(start)}
	case http.StatusUnauthorized, http.StatusForbidden:
		return checkResult{Name: "SAM.gov", Status: "FAIL", Detail: "invalid API key", Latency: ms(start)}
	default:
		return checkResult{Name: "SAM.gov", Status: "FAIL", Detail: goFmt("HTTP %d", resp.StatusCode), Latency: ms(start)}
	}
}

func checkUSASpending(baseURL string) checkResult {
	start := time.Now()
	url := baseURL + "/references/data_dictionary/"
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return checkResult{Name: "USASpending.gov", Status: "FAIL", Detail: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return checkResult{Name: "USASpending.gov", Status: "FAIL", Detail: goFmt("HTTP %d", resp.StatusCode), Latency: ms(start)}
	}
	return checkResult{Name: "USASpending.gov", Status: "OK", Detail: baseURL, Latency: ms(start)}
}

func printDoctorTable(checks []checkResult, allOK bool) {
	headers := []string{"CHECK", "STATUS", "DETAIL", "LATENCY"}
	var rows [][]string
	for _, c := range checks {
		status := c.Status
		if status == "OK" {
			status = "✓ OK"
		} else if status == "FAIL" {
			status = "✗ FAIL"
		}
		rows = append(rows, []string{c.Name, status, output.Truncate(c.Detail, 60), c.Latency})
	}
	output.PrintTable(headers, rows)
	if allOK {
		fmt.Println("\nAll checks passed.")
	} else {
		fmt.Println("\nSome checks failed. Run 'govprocure-pp-cli auth set-key' to add missing keys.")
	}
}

func ms(start time.Time) string {
	return goFmt("%dms", time.Since(start).Milliseconds())
}

func goFmt(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// exitError allows returning a typed exit code from cobra RunE.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string {
	return e.msg
}
