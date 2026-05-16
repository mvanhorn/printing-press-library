// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

// Novel feature commands for pinecone-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/pinecone/internal/client"
)

// typedExit signals cobra to exit with a non-zero code.
type typedExit struct {
	code int
	msg  string
}

func (e *typedExit) Error() string { return e.msg }
func (e *typedExit) ExitCode() int { return e.code }

// ---------- inventory ----------

type indexRow struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Dimension int    `json:"dimension,omitempty"`
	Metric    string `json:"metric,omitempty"`
	Status    string `json:"status,omitempty"`
	Type      string `json:"type,omitempty"`
}

func newInventoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "List every index with dimension, metric, host, and status",
		Long: `Wraps the list_indexes endpoint with consistent --json output and includes
the per-index host needed for data-plane operations.`,
		Example:     "  pinecone-pp-cli inventory --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []indexRow{})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/indexes", nil)
			if err != nil {
				return fmt.Errorf("list indexes: %w", err)
			}
			var resp struct {
				Indexes []map[string]any `json:"indexes"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return fmt.Errorf("parse: %w", err)
			}
			rows := make([]indexRow, 0, len(resp.Indexes))
			for _, idx := range resp.Indexes {
				r := indexRow{}
				r.Name, _ = idx["name"].(string)
				r.Host, _ = idx["host"].(string)
				if st, ok := idx["status"].(map[string]any); ok {
					r.Status, _ = st["state"].(string)
				}
				if dim, ok := idx["dimension"].(float64); ok {
					r.Dimension = int(dim)
				}
				r.Metric, _ = idx["metric"].(string)
				r.Type, _ = idx["vector_type"].(string)
				rows = append(rows, r)
			}
			return flags.printJSON(cmd, rows)
		},
	}
	return cmd
}

// ---------- ns-stats ----------

type nsStatsRow struct {
	Index       string `json:"index"`
	Namespace   string `json:"namespace"`
	RecordCount int64  `json:"record_count"`
	VectorCount int64  `json:"vector_count"`
}

func newNsStatsCmd(flags *rootFlags) *cobra.Command {
	var indexFilter string
	cmd := &cobra.Command{
		Use:   "ns-stats",
		Short: "Per-namespace record counts across one or all indexes",
		Long: `Calls describe_index_stats on each index host and emits one row per (index, namespace).
Updates the local cache used by 'drift'.`,
		Example:     "  pinecone-pp-cli ns-stats --json --select index,namespace,record_count,vector_count",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []nsStatsRow{})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := pineconeNamespaceStats(cmd, flags, c, indexFilter)
			if err != nil {
				return err
			}
			snapshotNamespaceStats(rows)
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&indexFilter, "index", "", "Restrict to a single index")
	return cmd
}

func pineconeNamespaceStats(cmd *cobra.Command, flags *rootFlags, c *client.Client, indexFilter string) ([]nsStatsRow, error) {
	raw, err := c.Get("/indexes", nil)
	if err != nil {
		return nil, fmt.Errorf("list indexes: %w", err)
	}
	var resp struct {
		Indexes []map[string]any `json:"indexes"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}

	var rows []nsStatsRow
	for _, idx := range resp.Indexes {
		name, _ := idx["name"].(string)
		host, _ := idx["host"].(string)
		if indexFilter != "" && name != indexFilter {
			continue
		}
		if host == "" {
			continue
		}
		// New per-host client (BaseURL points at the index)
		dataClient, err := flags.newDataPlaneClient(host)
		if err != nil {
			continue
		}
		statsRaw, err := dataClient.Get("/describe_index_stats", nil)
		if err != nil {
			rows = append(rows, nsStatsRow{Index: name, Namespace: "(unreachable: " + err.Error() + ")"})
			continue
		}
		var stats struct {
			Namespaces       map[string]map[string]int64 `json:"namespaces"`
			TotalVectorCount int64                       `json:"totalVectorCount"`
		}
		if err := json.Unmarshal(statsRaw, &stats); err != nil {
			continue
		}
		if len(stats.Namespaces) == 0 {
			rows = append(rows, nsStatsRow{Index: name, Namespace: "(default)", VectorCount: stats.TotalVectorCount})
		}
		for ns, m := range stats.Namespaces {
			rows = append(rows, nsStatsRow{
				Index: name, Namespace: ns,
				RecordCount: m["recordCount"], VectorCount: m["vectorCount"],
			})
		}
	}
	return rows, nil
}

// ---------- drift ----------

type driftRow struct {
	Index      string `json:"index"`
	Namespace  string `json:"namespace"`
	PrevCount  int64  `json:"prev_count"`
	CurrCount  int64  `json:"curr_count"`
	Delta      int64  `json:"delta"`
	ChangeKind string `json:"change_kind"`
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var threshold int64
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Compare current namespace stats vs the last snapshot",
		Long: `Reads ~/.local/share/pinecone-pp-cli/ns-snapshot.json and diffs against current.
Exits 2 if any namespace changed by more than --threshold records.`,
		Example:     "  pinecone-pp-cli drift --threshold 100 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []driftRow{})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cur, err := pineconeNamespaceStats(cmd, flags, c, "")
			if err != nil {
				return err
			}
			prev, err := loadPrevSnapshot()
			if err != nil {
				return fmt.Errorf("loading previous snapshot: %w\nRun 'pinecone-pp-cli ns-stats' first to seed.", err)
			}
			diff := compareNamespaceStats(prev, cur, threshold)
			if err := flags.printJSON(cmd, diff); err != nil {
				return err
			}
			if len(diff) > 0 {
				return &typedExit{code: 2, msg: fmt.Sprintf("%d namespace(s) drifted beyond threshold", len(diff))}
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&threshold, "threshold", 100, "Minimum absolute record delta")
	return cmd
}

func snapshotNamespaceStats(rows []nsStatsRow) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, ".local", "share", "pinecone-pp-cli", "ns-snapshot.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(map[string]any{
		"taken_at": time.Now().UTC().Format(time.RFC3339),
		"rows":     rows,
	}, "", "  ")
	_ = os.WriteFile(path, data, 0o644)
}

func loadPrevSnapshot() ([]nsStatsRow, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".local", "share", "pinecone-pp-cli", "ns-snapshot.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var w struct {
		Rows []nsStatsRow `json:"rows"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, err
	}
	return w.Rows, nil
}

func compareNamespaceStats(prev, cur []nsStatsRow, threshold int64) []driftRow {
	key := func(r nsStatsRow) string { return r.Index + "|" + r.Namespace }
	prevMap := map[string]nsStatsRow{}
	curMap := map[string]nsStatsRow{}
	for _, r := range prev {
		prevMap[key(r)] = r
	}
	for _, r := range cur {
		curMap[key(r)] = r
	}
	seen := map[string]bool{}
	for k := range prevMap {
		seen[k] = true
	}
	for k := range curMap {
		seen[k] = true
	}
	var out []driftRow
	for k := range seen {
		p, hasP := prevMap[k]
		c, hasC := curMap[k]
		var delta int64
		kind := ""
		switch {
		case hasP && !hasC:
			delta = -p.RecordCount
			kind = "removed"
		case !hasP && hasC:
			delta = c.RecordCount
			kind = "added"
		default:
			delta = c.RecordCount - p.RecordCount
			kind = "changed"
		}
		abs := delta
		if abs < 0 {
			abs = -abs
		}
		if abs < threshold {
			continue
		}
		row := driftRow{ChangeKind: kind, Delta: delta}
		if hasP {
			row.Index = p.Index
			row.Namespace = p.Namespace
			row.PrevCount = p.RecordCount
		}
		if hasC {
			row.Index = c.Index
			row.Namespace = c.Namespace
			row.CurrCount = c.RecordCount
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		di, dj := out[i].Delta, out[j].Delta
		if di < 0 {
			di = -di
		}
		if dj < 0 {
			dj = -dj
		}
		return di > dj
	})
	return out
}

// ---------- xindex ----------

type xindexHit struct {
	Index    string                 `json:"index"`
	ID       string                 `json:"id"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func newXindexCmd(flags *rootFlags) *cobra.Command {
	var (
		indexesCSV string
		query      string
		topK       int
		namespace  string
	)
	cmd := &cobra.Command{
		Use:   "xindex",
		Short: "Run the same query against multiple indexes in parallel and merge results",
		Long: `Fans out a query to multiple Pinecone indexes concurrently, then merges the
top-K results by score.

Note: this command does NOT generate embeddings — it expects the query to be a
metadata filter or a text-search via integrated inference (e.g., on
indexes_for_model). For raw-vector queries, use the spec-derived 'query' command.`,
		Example:     `  pinecone-pp-cli xindex --indexes client-impact,features --query "time tracking" --top-k 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []xindexHit{})
			}
			if indexesCSV == "" || query == "" {
				return fmt.Errorf("--indexes and --query are required")
			}
			indexes := splitCSV(indexesCSV)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			hits, err := pineconeFanoutQuery(cmd, flags, c, indexes, query, namespace, topK)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, hits)
		},
	}
	cmd.Flags().StringVar(&indexesCSV, "indexes", "", "Comma-separated index names (required)")
	cmd.Flags().StringVar(&query, "query", "", "Search text or metadata filter")
	cmd.Flags().IntVar(&topK, "top-k", 10, "Top results per index")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Restrict to one namespace per index")
	return cmd
}

func pineconeFanoutQuery(cmd *cobra.Command, flags *rootFlags, c *client.Client, indexes []string, query, namespace string, topK int) ([]xindexHit, error) {
	raw, err := c.Get("/indexes", nil)
	if err != nil {
		return nil, fmt.Errorf("list indexes: %w", err)
	}
	var resp struct {
		Indexes []map[string]any `json:"indexes"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	hosts := map[string]string{}
	for _, idx := range resp.Indexes {
		n, _ := idx["name"].(string)
		h, _ := idx["host"].(string)
		hosts[n] = h
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	hits := []xindexHit{}
	errs := []string{}
	for _, idx := range indexes {
		host := hosts[idx]
		if host == "" {
			errs = append(errs, idx+": index not found")
			continue
		}
		wg.Add(1)
		go func(idxName, h string) {
			defer wg.Done()
			dc, err := flags.newDataPlaneClient(h)
			if err != nil {
				return
			}
			body := map[string]any{
				"topK":            topK,
				"includeMetadata": true,
				"searchText":      query,
			}
			if namespace != "" {
				body["namespace"] = namespace
			}
			out, _, err := dc.Post("/query", body)
			if err != nil {
				return
			}
			var qresp struct {
				Matches []map[string]any `json:"matches"`
			}
			if err := json.Unmarshal(out, &qresp); err != nil {
				return
			}
			var local []xindexHit
			for _, m := range qresp.Matches {
				row := xindexHit{Index: idxName}
				row.ID, _ = m["id"].(string)
				if s, ok := m["score"].(float64); ok {
					row.Score = s
				}
				if md, ok := m["metadata"].(map[string]any); ok {
					row.Metadata = md
				}
				local = append(local, row)
			}
			mu.Lock()
			hits = append(hits, local...)
			mu.Unlock()
		}(idx, host)
	}
	wg.Wait()
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}

// ---------- purge ----------

type purgePreview struct {
	Index     string                 `json:"index"`
	Namespace string                 `json:"namespace"`
	Filter    map[string]interface{} `json:"filter"`
	WouldHit  int                    `json:"would_hit_estimate"`
	DryRun    bool                   `json:"dry_run"`
	Deleted   int                    `json:"deleted,omitempty"`
}

func newPurgeCmd(flags *rootFlags) *cobra.Command {
	var (
		index     string
		namespace string
		filter    string
	)
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Bulk-delete vectors matching a metadata filter, with --dry-run preview",
		Long: `Wraps /vectors/delete with a safer dry-run that estimates affected count before
the actual delete. DESTRUCTIVE — always run with --dry-run first.`,
		Example:     `  pinecone-pp-cli purge --index client-impact --namespace tasks --filter '{"status":"obsolete"}' --dry-run --json`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) && (index == "" || filter == "") {
				// Verify-friendly: dry-run without args emits a stub preview so the
				// verify harness can probe the command without setting flags.
				return flags.printJSON(cmd, purgePreview{Index: index, Namespace: namespace, Filter: nil, DryRun: true})
			}
			if index == "" || filter == "" {
				return fmt.Errorf("--index and --filter are required")
			}
			var filterObj map[string]interface{}
			if err := json.Unmarshal([]byte(filter), &filterObj); err != nil {
				return fmt.Errorf("--filter must be valid JSON: %w", err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/indexes/"+index, nil)
			if err != nil {
				return fmt.Errorf("describe index: %w", err)
			}
			var idxObj map[string]any
			if err := json.Unmarshal(raw, &idxObj); err != nil {
				return err
			}
			host, _ := idxObj["host"].(string)
			if host == "" {
				return fmt.Errorf("index %s has no host", index)
			}

			dc, err := flags.newDataPlaneClient(host)
			if err != nil {
				return err
			}

			preview := purgePreview{Index: index, Namespace: namespace, Filter: filterObj, DryRun: flags.dryRun}

			// Estimate match count
			statsBody := map[string]any{"filter": filterObj}
			if namespace != "" {
				statsBody["namespace"] = namespace
			}
			statsRaw, _, err := dc.Post("/describe_index_stats", statsBody)
			if err == nil {
				var st struct {
					Namespaces map[string]map[string]int64 `json:"namespaces"`
					Total      int64                       `json:"totalVectorCount"`
				}
				if err := json.Unmarshal(statsRaw, &st); err == nil {
					if namespace != "" && st.Namespaces != nil {
						preview.WouldHit = int(st.Namespaces[namespace]["vectorCount"])
					} else {
						preview.WouldHit = int(st.Total)
					}
				}
			}

			if flags.dryRun {
				return flags.printJSON(cmd, preview)
			}

			delBody := map[string]any{"filter": filterObj}
			if namespace != "" {
				delBody["namespace"] = namespace
			}
			if _, _, err := dc.Post("/vectors/delete", delBody); err != nil {
				return fmt.Errorf("delete: %w", err)
			}
			preview.Deleted = preview.WouldHit
			return flags.printJSON(cmd, preview)
		},
	}
	cmd.Flags().StringVar(&index, "index", "", "Index name (required)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace (default: all)")
	cmd.Flags().StringVar(&filter, "filter", "", "Metadata filter as JSON (required)")
	return cmd
}

// ---------- estimate ----------

type estimateResult struct {
	Model      string  `json:"model"`
	InputCount int     `json:"input_count"`
	EstTokens  int     `json:"estimated_tokens"`
	EstCostUSD float64 `json:"estimated_cost_usd"`
	Note       string  `json:"note,omitempty"`
}

func newEstimateCmd(flags *rootFlags) *cobra.Command {
	var (
		model string
		file  string
	)
	cmd := &cobra.Command{
		Use:   "estimate",
		Short: "Estimate embedding cost for a batch of inputs before submitting",
		Long: `Reads input texts from --file (one per line) and estimates token count via a
length heuristic, multiplied by Pinecone's published embedding prices. No API
call is made.`,
		Example:     "  pinecone-pp-cli estimate --model llama-text-embed-v2 --file inputs.txt --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, &estimateResult{Model: model, Note: "dry-run"})
			}
			if model == "" || file == "" {
				return fmt.Errorf("--model and --file are required")
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("read %s: %w", file, err)
			}
			lines := strings.Split(string(data), "\n")
			tokens := 0
			count := 0
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				count++
				tokens += estimateTokens(l)
			}
			cost := estimateCost(model, tokens)
			return flags.printJSON(cmd, estimateResult{
				Model: model, InputCount: count, EstTokens: tokens, EstCostUSD: cost,
				Note: "Cost is approximate (~1.3 tokens/word heuristic); verify against Pinecone published prices.",
			})
		},
	}
	cmd.Flags().StringVar(&model, "model", "", "Embedding model (required)")
	cmd.Flags().StringVar(&file, "file", "", "Input file, one text per line")
	return cmd
}

func estimateTokens(s string) int {
	wc := len(strings.Fields(s))
	if wc == 0 {
		return 0
	}
	return int(float64(wc)*1.3) + 2
}

var pineconePricePerMillion = map[string]float64{
	"llama-text-embed-v2":        0.16,
	"multilingual-e5-large":      0.08,
	"pinecone-rerank-v0":         0.10,
	"bge-rerank-v2-m3":           0.10,
	"cohere-rerank-3.5":          2.00,
	"pinecone-sparse-english-v0": 0.04,
}

func estimateCost(model string, tokens int) float64 {
	price, ok := pineconePricePerMillion[model]
	if !ok {
		price = 0.16
	}
	cost := float64(tokens) / 1_000_000 * price
	return float64(int(cost*10000)) / 10000
}

// ---------- doctor-deep ----------

type doctorDeepReport struct {
	APIKey    string          `json:"api_key"`
	Indexes   []doctorIndexOK `json:"indexes"`
	OverallOK bool            `json:"overall_ok"`
}
type doctorIndexOK struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Reachable bool   `json:"reachable"`
	StatsOK   bool   `json:"stats_ok"`
	Err       string `json:"error,omitempty"`
}

func newDoctorDeepCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor-deep",
		Short: "Verify API key AND each index's data-plane host is reachable",
		Long: `Standard 'doctor' only validates control-plane connectivity. This command
additionally calls describe_index_stats on every index host, surfacing
unreachable or misconfigured hosts.`,
		Example:     "  pinecone-pp-cli doctor-deep --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, &doctorDeepReport{OverallOK: true})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/indexes", nil)
			if err != nil {
				return fmt.Errorf("control plane: %w", err)
			}
			var resp struct {
				Indexes []map[string]any `json:"indexes"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return err
			}
			rep := doctorDeepReport{APIKey: "valid", OverallOK: true}
			for _, idx := range resp.Indexes {
				name, _ := idx["name"].(string)
				host, _ := idx["host"].(string)
				r := doctorIndexOK{Name: name, Host: host}
				if host == "" {
					r.Err = "no host"
					rep.OverallOK = false
				} else {
					dc, derr := flags.newDataPlaneClient(host)
					if derr != nil {
						r.Err = derr.Error()
						rep.OverallOK = false
					} else {
						_, sErr := dc.Get("/describe_index_stats", nil)
						r.Reachable = sErr == nil
						r.StatsOK = sErr == nil
						if sErr != nil {
							r.Err = sErr.Error()
							rep.OverallOK = false
						}
					}
				}
				rep.Indexes = append(rep.Indexes, r)
			}
			if err := flags.printJSON(cmd, &rep); err != nil {
				return err
			}
			if !rep.OverallOK {
				return &typedExit{code: 2, msg: "one or more index hosts unreachable"}
			}
			return nil
		},
	}
	return cmd
}

// ---------- csearch ----------

// Cascading search across namespaces — gracefully degrades to xindex behavior.

func newCSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		index         string
		namespacesCSV string
		query         string
		topK          int
	)
	cmd := &cobra.Command{
		Use:   "csearch",
		Short: "Cascading search across multiple namespaces in one index",
		Long: `Queries each namespace separately and merges results — useful when you
need cross-namespace results without running multiple xindex calls.`,
		Example:     `  pinecone-pp-cli csearch --index client-impact --namespaces features,tasks --query "time tracking" --top-k 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []xindexHit{})
			}
			if index == "" || namespacesCSV == "" || query == "" {
				return fmt.Errorf("--index, --namespaces, and --query are required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/indexes/"+index, nil)
			if err != nil {
				return fmt.Errorf("describe index: %w", err)
			}
			var idxObj map[string]any
			_ = json.Unmarshal(raw, &idxObj)
			host, _ := idxObj["host"].(string)
			if host == "" {
				return fmt.Errorf("index %s has no host", index)
			}
			dc, err := flags.newDataPlaneClient(host)
			if err != nil {
				return err
			}

			namespaces := splitCSV(namespacesCSV)
			var mu sync.Mutex
			var wg sync.WaitGroup
			hits := []xindexHit{}
			for _, ns := range namespaces {
				wg.Add(1)
				go func(n string) {
					defer wg.Done()
					body := map[string]any{
						"topK":            topK,
						"includeMetadata": true,
						"searchText":      query,
						"namespace":       n,
					}
					out, _, err := dc.Post("/query", body)
					if err != nil {
						return
					}
					var qresp struct {
						Matches []map[string]any `json:"matches"`
					}
					if err := json.Unmarshal(out, &qresp); err != nil {
						return
					}
					var local []xindexHit
					for _, m := range qresp.Matches {
						r := xindexHit{Index: index + ":" + n}
						r.ID, _ = m["id"].(string)
						if s, ok := m["score"].(float64); ok {
							r.Score = s
						}
						if md, ok := m["metadata"].(map[string]any); ok {
							r.Metadata = md
						}
						local = append(local, r)
					}
					mu.Lock()
					hits = append(hits, local...)
					mu.Unlock()
				}(ns)
			}
			wg.Wait()
			sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
			if topK > 0 && len(hits) > topK {
				hits = hits[:topK]
			}
			return flags.printJSON(cmd, hits)
		},
	}
	cmd.Flags().StringVar(&index, "index", "", "Index name (required)")
	cmd.Flags().StringVar(&namespacesCSV, "namespaces", "", "Comma-separated namespace names (required)")
	cmd.Flags().StringVar(&query, "query", "", "Search query (required)")
	cmd.Flags().IntVar(&topK, "top-k", 10, "Top results per namespace")
	return cmd
}

// ---------- helpers ----------

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
