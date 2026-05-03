// `dupe-cluster` finds listings whose body fingerprint and image hash match
// across cities. We compute a 64-bit SimHash over the body text and group
// rows whose hashes are within hamming distance 8. Cross-city scams and
// aggregator reposts share body text verbatim, so this surface is high signal
// for the triage commands that consume cluster sizes (e.g. scam-score adds
// 15 points if the listing belongs to a 3+-member cluster).

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math/bits"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// dupeCluster is one cluster — a group of pids whose bodies hash close enough
// that we treat them as the same listing.
type dupeCluster struct {
	ClusterID           int     `json:"clusterId"`
	Members             []int64 `json:"memberPids"`
	RepresentativeTitle string  `json:"representativeTitle"`
	Size                int     `json:"size"`
}

func newDupeClusterCmd(flags *rootFlags) *cobra.Command {
	var category, site string
	var minClusterSize int
	var pid int64
	cmd := &cobra.Command{
		Use:         "dupe-cluster",
		Short:       "Cluster listings by body-text similarity (cross-city dup detection)",
		Long:        "Compute a SimHash over every listing's body text and group rows within hamming distance 8. Surfaces aggregator reposts and likely scams.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := loadListingsForCluster(ctx, db.DB(), site, category)
			if err != nil {
				return err
			}
			clusters := buildDupeClusters(rows, minClusterSize)
			if pid > 0 {
				clusters = filterClustersForPID(clusters, pid)
			}
			return printJSONFiltered(cmd.OutOrStdout(), clusters, flags)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Filter by category abbreviation")
	cmd.Flags().StringVar(&site, "site", "", "Filter by site (omit for cross-city)")
	cmd.Flags().IntVar(&minClusterSize, "min-cluster-size", 2, "Drop clusters smaller than this")
	cmd.Flags().Int64Var(&pid, "pid", 0, "Show only the cluster containing this PID")
	return cmd
}

// listingForCluster is the minimal projection we need for clustering.
type listingForCluster struct {
	PID      int64
	Title    string
	BodyText string
}

func loadListingsForCluster(ctx context.Context, db *sql.DB, site, category string) ([]listingForCluster, error) {
	q := `SELECT pid, COALESCE(title,''), COALESCE(body_text,'') FROM listings WHERE 1=1`
	args := []any{}
	if site != "" {
		q += " AND site = ?"
		args = append(args, site)
	}
	if category != "" {
		q += " AND category_abbr = ?"
		args = append(args, category)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("load listings: %w", err)
	}
	defer rows.Close()
	var out []listingForCluster
	for rows.Next() {
		var l listingForCluster
		if err := rows.Scan(&l.PID, &l.Title, &l.BodyText); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// buildDupeClusters assigns each row a 64-bit SimHash, then merges rows whose
// hashes are within hamming distance 8 using a union-find pass.
func buildDupeClusters(rows []listingForCluster, minSize int) []dupeCluster {
	if minSize < 2 {
		minSize = 2
	}
	type hashed struct {
		idx int
		sig uint64
		row listingForCluster
	}
	all := make([]hashed, 0, len(rows))
	for i, r := range rows {
		// Fall back to the title when body_text is empty — search-only sync
		// stores title without rapi-hydrated body, and simhash of "" is the
		// same value for every row, which would falsely cluster everything.
		// Require at least 12 non-space characters of signal so very short
		// titles ("ipad", "couch") don't dominate clustering either.
		text := r.BodyText
		if len(strings.Fields(text)) < 4 {
			text = r.Title
		}
		if len(strings.ReplaceAll(text, " ", "")) < 12 {
			continue
		}
		all = append(all, hashed{idx: i, sig: simhash64(text), row: r})
	}
	parent := make([]int, len(all))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	const maxHamming = 8
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if hamming64(all[i].sig, all[j].sig) <= maxHamming {
				union(i, j)
			}
		}
	}
	groups := map[int][]hashed{}
	for i := range all {
		root := find(i)
		groups[root] = append(groups[root], all[i])
	}
	out := make([]dupeCluster, 0, len(groups))
	cid := 0
	for _, members := range groups {
		if len(members) < minSize {
			continue
		}
		cid++
		pids := make([]int64, 0, len(members))
		var title string
		for _, m := range members {
			pids = append(pids, m.row.PID)
			if title == "" {
				title = m.row.Title
			}
		}
		sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
		out = append(out, dupeCluster{ClusterID: cid, Members: pids, RepresentativeTitle: title, Size: len(pids)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].ClusterID < out[j].ClusterID
	})
	return out
}

func filterClustersForPID(clusters []dupeCluster, pid int64) []dupeCluster {
	out := clusters[:0]
	for _, c := range clusters {
		for _, m := range c.Members {
			if m == pid {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// simhash64 computes a 64-bit SimHash of the input. Tokens are whitespace-split
// lowercased words. This is deliberately small + dependency-free; the property
// we need is "near-duplicate strings hash close in hamming distance," which
// 64-bit SimHash satisfies for ~kilobyte body texts.
func simhash64(s string) uint64 {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	tokens := strings.Fields(strings.ToLower(s))
	var v [64]int
	for _, tok := range tokens {
		h := fnv64(tok)
		for i := 0; i < 64; i++ {
			if (h>>i)&1 == 1 {
				v[i]++
			} else {
				v[i]--
			}
		}
	}
	var sig uint64
	for i := 0; i < 64; i++ {
		if v[i] > 0 {
			sig |= 1 << uint(i)
		}
	}
	return sig
}

// fnv64 is FNV-1a 64-bit. Stdlib has hash/fnv but we want this loop tight and
// allocation-free; the 8-line implementation is faster than the io.Writer
// surface and easier to reason about for tests.
func fnv64(s string) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime
	}
	return h
}

// hamming64 returns the popcount of the XOR of two 64-bit values.
func hamming64(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}
