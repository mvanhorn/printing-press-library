package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// batchAppHistory is the combined history for a single application.
type batchAppHistory struct {
	ApplicationNumber string          `json:"applicationNumber"`
	Transactions      json.RawMessage `json:"transactions,omitempty"`
	Continuity        json.RawMessage `json:"continuity,omitempty"`
	Assignment        json.RawMessage `json:"assignment,omitempty"`
	Error             string          `json:"error,omitempty"`
}

func newBatchHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch-history <num1> [num2] [num3] ...",
		Short: "Fetch prosecution history for multiple applications",
		Long: `Fetches transactions, continuity, and assignment data for multiple
application numbers in one command. Reads from args or stdin (one per line).
Automatically throttles to stay under rate limits.`,
		Example: strings.Trim(`
  uspto-patents-pp-cli patent batch-history 14412875 15123456
  echo -e "14412875\n15123456" | uspto-patents-pp-cli patent batch-history
  uspto-patents-pp-cli patent batch-history 14412875 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read app numbers from args or stdin
			appNums := args
			if len(appNums) == 0 {
				// Check if stdin is piped
				stat, _ := os.Stdin.Stat()
				if (stat.Mode() & os.ModeCharDevice) == 0 {
					scanner := bufio.NewScanner(os.Stdin)
					for scanner.Scan() {
						line := strings.TrimSpace(scanner.Text())
						if line != "" {
							appNums = append(appNums, line)
						}
					}
				}
			}

			if len(appNums) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var results []batchAppHistory

			// Rate limit: 60 req/min = 1 req/sec. Each app makes 3 requests.
			// Process in batches of 20 apps (60 requests) with pauses.
			batchSize := 20
			requestCount := 0

			for i, appNum := range appNums {
				if i > 0 && requestCount >= batchSize*3 {
					fmt.Fprintf(cmd.ErrOrStderr(), "rate limit pause (processed %d/%d)...\n", i, len(appNums))
					time.Sleep(60 * time.Second)
					requestCount = 0
				}

				entry := batchAppHistory{ApplicationNumber: appNum}

				// Transactions
				txPath := replacePathParam("/api/v1/patent/applications/{num}/transactions", "num", appNum)
				txData, err := c.Get(txPath, nil)
				requestCount++
				if err != nil {
					entry.Error = fmt.Sprintf("transactions: %v", err)
				} else {
					entry.Transactions = txData
				}

				// Continuity
				contPath := replacePathParam("/api/v1/patent/applications/{num}/continuity", "num", appNum)
				contData, err := c.Get(contPath, nil)
				requestCount++
				if err != nil {
					if entry.Error != "" {
						entry.Error += "; "
					}
					entry.Error += fmt.Sprintf("continuity: %v", err)
				} else {
					entry.Continuity = contData
				}

				// Assignment
				assignPath := replacePathParam("/api/v1/patent/applications/{num}/assignment", "num", appNum)
				assignData, err := c.Get(assignPath, nil)
				requestCount++
				if err != nil {
					if entry.Error != "" {
						entry.Error += "; "
					}
					entry.Error += fmt.Sprintf("assignment: %v", err)
				} else {
					entry.Assignment = assignData
				}

				results = append(results, entry)

				fmt.Fprintf(cmd.ErrOrStderr(), "processed %d/%d: %s\n", i+1, len(appNums), appNum)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	return cmd
}
