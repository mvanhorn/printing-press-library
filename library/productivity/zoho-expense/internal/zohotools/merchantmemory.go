// Package zohotools provides hand-authored shared logic for the novel
// commands layered on top of the generated Zoho Expense CLI: merchant
// memory, autoscan polling, receipt-hash dedup, and GST math.
package zohotools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// TagMapping represents a reporting-tag selection for a merchant.
type TagMapping struct {
	TagID       string `json:"tag_id"`
	TagOptionID string `json:"tag_option_id"`
	TagName     string `json:"tag_name"`
	OptionName  string `json:"option_name"`
}

// MerchantMapping is the local memory of how a merchant should be tagged
// when a fresh receipt arrives with that merchant name.
type MerchantMapping struct {
	MerchantName       string       `json:"merchant_name"`
	CategoryID         string       `json:"category_id,omitempty"`
	CategoryName       string       `json:"category_name,omitempty"`
	ProjectID          string       `json:"project_id,omitempty"`
	ProjectName        string       `json:"project_name,omitempty"`
	Tags               []TagMapping `json:"tags,omitempty"`
	LearnedFromHistory bool         `json:"learned_from_history,omitempty"`
	SeenCount          int          `json:"seen_count,omitempty"`
}

// EnsureSchema creates the merchant_memory and receipt_hashes tables.
// Idempotent — safe to call before every operation.
func EnsureSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS merchant_memory (
			merchant_name TEXT PRIMARY KEY,
			category_id TEXT,
			category_name TEXT,
			project_id TEXT,
			project_name TEXT,
			tags_json TEXT,
			learned_from_history INTEGER DEFAULT 0,
			seen_count INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS receipt_hashes (
			hash TEXT PRIMARY KEY,
			expense_id TEXT NOT NULL,
			original_filename TEXT,
			uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_receipt_hashes_expense ON receipt_hashes(expense_id)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("merchant memory schema: %w", err)
		}
	}
	return nil
}

// GetMerchant returns the saved mapping for the given merchant name, or
// (nil, nil) if no mapping exists. Matching is case-insensitive on the
// stored key.
func GetMerchant(db *sql.DB, merchantName string) (*MerchantMapping, error) {
	if strings.TrimSpace(merchantName) == "" {
		return nil, nil
	}
	row := db.QueryRow(
		`SELECT merchant_name, COALESCE(category_id,''), COALESCE(category_name,''),
		        COALESCE(project_id,''), COALESCE(project_name,''),
		        COALESCE(tags_json,''), COALESCE(learned_from_history,0), COALESCE(seen_count,0)
		 FROM merchant_memory WHERE LOWER(merchant_name) = LOWER(?)`,
		merchantName,
	)
	var m MerchantMapping
	var tagsJSON string
	var learned int
	if err := row.Scan(&m.MerchantName, &m.CategoryID, &m.CategoryName,
		&m.ProjectID, &m.ProjectName, &tagsJSON, &learned, &m.SeenCount); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	m.LearnedFromHistory = learned != 0
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
	}
	return &m, nil
}

// PutMerchant upserts a merchant mapping.
func PutMerchant(db *sql.DB, m *MerchantMapping) error {
	if m == nil || strings.TrimSpace(m.MerchantName) == "" {
		return fmt.Errorf("merchant_name required")
	}
	var tagsJSON string
	if len(m.Tags) > 0 {
		b, err := json.Marshal(m.Tags)
		if err != nil {
			return fmt.Errorf("marshal tags: %w", err)
		}
		tagsJSON = string(b)
	}
	learned := 0
	if m.LearnedFromHistory {
		learned = 1
	}
	_, err := db.Exec(
		`INSERT INTO merchant_memory (merchant_name, category_id, category_name, project_id, project_name, tags_json, learned_from_history, seen_count, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(merchant_name) DO UPDATE SET
		   category_id = excluded.category_id,
		   category_name = excluded.category_name,
		   project_id = excluded.project_id,
		   project_name = excluded.project_name,
		   tags_json = excluded.tags_json,
		   learned_from_history = excluded.learned_from_history,
		   seen_count = excluded.seen_count,
		   updated_at = CURRENT_TIMESTAMP`,
		m.MerchantName, m.CategoryID, m.CategoryName, m.ProjectID, m.ProjectName,
		tagsJSON, learned, m.SeenCount,
	)
	if err != nil {
		return fmt.Errorf("upsert merchant_memory: %w", err)
	}
	return nil
}

// ListMerchants returns all saved merchant mappings ordered by merchant_name.
func ListMerchants(db *sql.DB) ([]MerchantMapping, error) {
	rows, err := db.Query(
		`SELECT merchant_name, COALESCE(category_id,''), COALESCE(category_name,''),
		        COALESCE(project_id,''), COALESCE(project_name,''),
		        COALESCE(tags_json,''), COALESCE(learned_from_history,0), COALESCE(seen_count,0)
		 FROM merchant_memory ORDER BY merchant_name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MerchantMapping
	for rows.Next() {
		var m MerchantMapping
		var tagsJSON string
		var learned int
		if err := rows.Scan(&m.MerchantName, &m.CategoryID, &m.CategoryName,
			&m.ProjectID, &m.ProjectName, &tagsJSON, &learned, &m.SeenCount); err != nil {
			return nil, err
		}
		m.LearnedFromHistory = learned != 0
		if tagsJSON != "" {
			_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MerchantSummary is a synthesized row joining the expenses table with
// any explicit merchant_memory mapping for the same merchant.
type MerchantSummary struct {
	MerchantName   string `json:"merchant_name"`
	ExpenseCount   int    `json:"expense_count"`
	LastCategory   string `json:"last_category,omitempty"`
	MappedCategory string `json:"mapped_category,omitempty"`
	MappedProject  string `json:"mapped_project,omitempty"`
	HasExplicitMap bool   `json:"has_explicit_map"`
}

// SynthesizeMerchants walks the local expenses table, groups by merchant
// name, and joins to merchant_memory. Useful for `merchant list`.
func SynthesizeMerchants(db *sql.DB) ([]MerchantSummary, error) {
	rows, err := db.Query(
		`SELECT COALESCE(merchant_name,'') AS m,
		        COUNT(*) AS c,
		        COALESCE(MAX(category_name),'') AS last_cat
		 FROM expenses
		 WHERE merchant_name IS NOT NULL AND merchant_name <> ''
		 GROUP BY merchant_name
		 ORDER BY c DESC, m ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MerchantSummary, 0)
	for rows.Next() {
		var s MerchantSummary
		if err := rows.Scan(&s.MerchantName, &s.ExpenseCount, &s.LastCategory); err != nil {
			return nil, err
		}
		m, _ := GetMerchant(db, s.MerchantName)
		if m != nil {
			s.HasExplicitMap = true
			s.MappedCategory = m.CategoryName
			if s.MappedCategory == "" {
				s.MappedCategory = m.CategoryID
			}
			s.MappedProject = m.ProjectName
			if s.MappedProject == "" {
				s.MappedProject = m.ProjectID
			}
		}
		out = append(out, s)
	}
	// Stable secondary sort by merchant name for ties.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ExpenseCount != out[j].ExpenseCount {
			return out[i].ExpenseCount > out[j].ExpenseCount
		}
		return strings.ToLower(out[i].MerchantName) < strings.ToLower(out[j].MerchantName)
	})
	return out, rows.Err()
}

// DeduceFromHistory passes over the expenses table and populates
// merchant_memory with the most-common category seen per merchant. Does
// not overwrite mappings that already exist with a non-empty category.
func DeduceFromHistory(db *sql.DB) error {
	rows, err := db.Query(
		`SELECT merchant_name, category_id, category_name, project_id, project_name, COUNT(*) AS c
		 FROM expenses
		 WHERE merchant_name IS NOT NULL AND merchant_name <> ''
		   AND category_id IS NOT NULL AND category_id <> ''
		 GROUP BY merchant_name, category_id, category_name, project_id, project_name`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	type key struct {
		merchant, catID, catName, projID, projName string
	}
	counts := map[string]map[key]int{}
	for rows.Next() {
		var k key
		var c int
		if err := rows.Scan(&k.merchant, &k.catID, &k.catName, &k.projID, &k.projName, &c); err != nil {
			return err
		}
		if counts[k.merchant] == nil {
			counts[k.merchant] = map[key]int{}
		}
		counts[k.merchant][k] = c
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for merchant, kc := range counts {
		var best key
		bestN := -1
		for k, n := range kc {
			if n > bestN {
				bestN = n
				best = k
			}
		}
		if bestN <= 0 {
			continue
		}
		existing, _ := GetMerchant(db, merchant)
		if existing != nil && existing.CategoryID != "" && !existing.LearnedFromHistory {
			// User-set mapping; preserve it.
			continue
		}
		m := &MerchantMapping{
			MerchantName:       merchant,
			CategoryID:         best.catID,
			CategoryName:       best.catName,
			ProjectID:          best.projID,
			ProjectName:        best.projName,
			LearnedFromHistory: true,
			SeenCount:          bestN,
		}
		if err := PutMerchant(db, m); err != nil {
			return err
		}
	}
	return nil
}
