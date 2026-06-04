// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
	"github.com/spf13/cobra"
)

// PATCH: hand-written quota-aware batch lookup command promised by README Highlights.
func newTypesBatchCmd(flags *rootFlags) *cobra.Command {
	var filePath string
	var resumable bool
	var checkpoint string
	var lang string

	cmd := &cobra.Command{
		Use:     "batch",
		Short:   "Look up many Numista type IDs in one quota-aware command. --dry-run forecasts cost; --resumable splits a list larger than the monthly budget across UTC months.",
		Example: "  numista-pp-cli types batch --file ./type-ids.csv --dry-run --json\n  numista-pp-cli types batch --file ./type-ids.txt --resumable --checkpoint ./batch.ckpt --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return cmd.Help()
			}
			if filePath == "" {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if err := validateLang(lang); err != nil {
				return err
			}
			// PRINTING_PRESS_VERIFY=1: never spend quota or read fixture files during verify probes.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"total_ids":0,"cached_fresh":0,"live_needed":0,"estimated_calls":0,"fits_remaining_quota":true,"verify_short_circuit":true}`)
				return nil
			}
			ids, err := parseTypeIDFile(filePath)
			if err != nil {
				return usageErr(err)
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("numista-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if flags.dryRun {
				forecast, err := forecastBatch(cmd.Context(), s, ids)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), forecast, flags)
			}
			if resumable && checkpoint == "" {
				return usageErr(fmt.Errorf("--checkpoint is required with --resumable"))
			}
			done := map[int64]bool{}
			if resumable {
				done, err = readCheckpoint(checkpoint)
				if err != nil {
					return err
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			summary := map[string]any{
				"processed":   0,
				"live_calls":  0,
				"cache_hits":  0,
				"errors":      0,
				"quota_after": cliutil.QuotaSnapshot{},
			}
			for i, id := range ids {
				if done[id] {
					continue
				}
				_, live, err := quotaTrackedGet(cmd.Context(), c, s, "/types/"+strconv.FormatInt(id, 10), map[string]string{"lang": lang})
				if err != nil {
					summary["errors"] = summary["errors"].(int) + 1
					var apiError *client.APIError
					if errors.As(err, &apiError) && apiError.StatusCode == http.StatusTooManyRequests {
						remaining := countUnprocessed(ids[i:], done)
						fmt.Fprintf(os.Stderr, "quota exhausted after type %d; %d IDs remain. Retry after %s.\n", id, remaining, cliutil.ResetForNextMonth(time.Now()).Format("2006-01-02 UTC"))
						return apiErr(fmt.Errorf("quota exhausted; %d IDs remain", remaining))
					}
					return classifyAPIError(err, flags)
				}
				summary["processed"] = summary["processed"].(int) + 1
				if live {
					summary["live_calls"] = summary["live_calls"].(int) + 1
				} else {
					summary["cache_hits"] = summary["cache_hits"].(int) + 1
				}
				if resumable {
					done[id] = true
					if err := writeCheckpoint(checkpoint, done); err != nil {
						return err
					}
				}
			}
			q, err := readQuota(cmd.Context(), s)
			if err != nil {
				return err
			}
			summary["quota_after"] = q
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Input file with type IDs (.csv, .jsonl, .ndjson, .txt)")
	cmd.Flags().BoolVar(&resumable, "resumable", false, "Skip completed IDs from a checkpoint and update it after each success")
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "Checkpoint file for --resumable")
	cmd.Flags().StringVar(&lang, "lang", "en", "Language (one of: en, es, fr)")
	return cmd
}

func parseTypeIDFile(path string) ([]int64, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		return parseCSVTypeIDs(path)
	case ".jsonl", ".ndjson":
		return parseJSONLTypeIDs(path)
	default:
		return parseTextTypeIDs(path)
	}
}

func parseCSVTypeIDs(path string) ([]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	var ids []int64
	for i, row := range rows {
		line := i + 1
		if len(row) == 0 {
			continue
		}
		id, err := parsePositiveInt64(strings.TrimSpace(row[0]))
		if err != nil {
			if line == 1 {
				continue
			}
			return nil, fmt.Errorf("line %d: first CSV column must be a positive integer", line)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseJSONLTypeIDs(path string) ([]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ids []int64
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		if id, err := parsePositiveInt64(text); err == nil {
			ids = append(ids, id)
			continue
		}
		var obj struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal([]byte(text), &obj); err != nil || obj.ID <= 0 {
			return nil, fmt.Errorf("line %d: JSONL row must be {\"id\": <positive integer>} or a bare positive integer", line)
		}
		ids = append(ids, obj.ID)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func parseTextTypeIDs(path string) ([]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ids []int64
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		id, err := parsePositiveInt64(text)
		if err != nil {
			return nil, fmt.Errorf("line %d: expected positive integer type ID", line)
		}
		ids = append(ids, id)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func parsePositiveInt64(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("not a positive integer")
	}
	return id, nil
}

func forecastBatch(ctx context.Context, s *store.Store, ids []int64) (map[string]any, error) {
	q, err := readQuota(ctx, s)
	if err != nil {
		return nil, err
	}
	cached := 0
	for _, id := range ids {
		ok, err := recentFreshLookup(ctx, s, "/types/"+strconv.FormatInt(id, 10))
		if err != nil {
			return nil, err
		}
		if ok {
			cached++
		}
	}
	liveNeeded := len(ids) - cached
	return map[string]any{
		"total_ids":            len(ids),
		"cached_fresh":         cached,
		"live_needed":          liveNeeded,
		"quota_used_now":       q.Used,
		"quota_remaining":      q.Remaining,
		"quota_after_batch":    q.Used + liveNeeded,
		"fits_remaining_quota": liveNeeded <= q.Remaining,
		"estimated_calls":      liveNeeded,
	}, nil
}

// readQuota reads this month's used/remaining quota from the lookup_log table
// on the caller-supplied store handle. Callers MUST open the store once per
// command invocation and reuse the handle across every readQuota call to
// avoid per-call open+migrate+close churn under the SQLite migration lock.
func readQuota(ctx context.Context, s *store.Store) (cliutil.QuotaSnapshot, error) {
	return cliutil.ReadQuotaFromDB(ctx, s.DB())
}

func recentFreshLookup(ctx context.Context, s *store.Store, endpoint string) (bool, error) {
	var n int
	err := s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lookup_log
		 WHERE endpoint = ?
		   AND is_valid_request = 1
		   AND called_at >= datetime('now', '-5 minutes')`, endpoint).Scan(&n)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	return n > 0, nil
}

// quotaTrackedGet wraps a client GET and reports whether the call was live
// (vs cache-served) by comparing the lookup_log row count before/after. The
// store handle threads through from the caller's RunE — see readQuota.
func quotaTrackedGet(ctx context.Context, c *client.Client, s *store.Store, path string, params map[string]string) (json.RawMessage, bool, error) {
	before, err := readQuota(ctx, s)
	if err != nil {
		return nil, false, err
	}
	data, err := c.GetWithHeaders(path, params, nil)
	after, qerr := readQuota(ctx, s)
	if qerr != nil {
		return nil, false, qerr
	}
	return data, after.Used > before.Used, err
}

func readCheckpoint(path string) (map[int64]bool, error) {
	done := map[int64]bool{}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return done, nil
	}
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		id, err := parsePositiveInt64(text)
		if err != nil {
			return nil, fmt.Errorf("checkpoint line %d: expected positive integer type ID", line)
		}
		done[id] = true
	}
	return done, sc.Err()
}

func writeCheckpoint(path string, done map[int64]bool) error {
	var b strings.Builder
	for id := range done {
		b.WriteString(strconv.FormatInt(id, 10))
		b.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func countUnprocessed(ids []int64, done map[int64]bool) int {
	n := 0
	for _, id := range ids {
		if !done[id] {
			n++
		}
	}
	return n
}

func validateLang(lang string) error {
	switch lang {
	case "en", "es", "fr":
		return nil
	default:
		return usageErr(fmt.Errorf("--lang must be one of en, es, fr; got %q", lang))
	}
}
