package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

// SearchCountContext uses the same token and short-token fallback rules as
// SearchContextOffset so MCP's total and continuation cursor agree with its rows.
func (s *Store) SearchCountContext(ctx context.Context, query string) (int, error) {
	return searchCountContext(ctx, s.db, query)
}

type searchCounter interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func searchCountContext(ctx context.Context, db searchCounter, query string) (int, error) {
	tokens := ftsQueryTokenRE.FindAllString(query, -1)
	if len(tokens) == 0 {
		return 0, nil
	}
	clause := "resources_fts MATCH ?"
	args := []any{FTSMatchQuery(query)}
	if ftsNeedsLikeFallback(tokens) {
		clause, args = ftsLikeClause("f", tokens)
	}
	var count int
	err := db.QueryRowContext(ctx, strings.Join([]string{
		"SELECT COUNT(*) FROM resources r",
		"JOIN resources_fts f ON r.id = f.id AND r.resource_type = f.resource_type",
		"WHERE " + clause,
	}, " "), args...).Scan(&count)
	return count, err
}

// SearchPageContext returns rows and total from one SQLite snapshot.
func (s *Store) SearchPageContext(ctx context.Context, query string, limit, offset int) ([]json.RawMessage, int, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	rows, err := searchContextOffset(ctx, tx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := searchCountContext(ctx, tx, query)
	if err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	return rows, count, nil
}
