// Copyright 2026 Eric Rash and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored helpers shared by the novel commands (ground, items recent,
// cite, collections tree, cache status/reindex). Kept out of generated files
// so regen-merge preserves them.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/zotero-research-library/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/zotero-research-library/internal/store"
)

// apiErrStatus extracts the HTTP status from a client error, or 0.
func apiErrStatus(err error) int {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}

// zoteroItemData is the subset of a Zotero item's nested `data` object the
// novel commands read. The API envelope is {key, version, data:{...}}.
type zoteroItemData struct {
	Key              string          `json:"key"`
	ItemType         string          `json:"itemType"`
	Title            string          `json:"title"`
	AbstractNote     string          `json:"abstractNote"`
	Date             string          `json:"date"`
	DateAdded        string          `json:"dateAdded"`
	DOI              string          `json:"DOI"`
	PublicationTitle string          `json:"publicationTitle"`
	Extra            string          `json:"extra"`
	ParentItem       string          `json:"parentItem"`
	Collections      []string        `json:"collections"`
	Creators         []zoteroCreator `json:"creators"`
}

type zoteroCreator struct {
	CreatorType string `json:"creatorType"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Name        string `json:"name"`
}

// decodeItemData unwraps the nested `data` object from a stored item envelope.
func decodeItemData(raw []byte) (zoteroItemData, error) {
	var envelope struct {
		Key  string         `json:"key"`
		Data zoteroItemData `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return zoteroItemData{}, err
	}
	d := envelope.Data
	if d.Key == "" {
		d.Key = envelope.Key
	}
	return d, nil
}

// creatorSummary renders "Last, Last & Last" (capped at three surnames) for
// human and JSON output. Institutional creators use their single Name field.
func creatorSummary(creators []zoteroCreator) string {
	var names []string
	for _, c := range creators {
		switch {
		case c.LastName != "":
			names = append(names, c.LastName)
		case c.Name != "":
			names = append(names, c.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	if len(names) > 3 {
		return strings.Join(names[:3], ", ") + " et al."
	}
	if len(names) > 1 {
		return strings.Join(names[:len(names)-1], ", ") + " & " + names[len(names)-1]
	}
	return names[0]
}

// ftsQuote wraps every user-supplied term as a quoted FTS5 string so
// punctuation (hyphens, apostrophes, colons) can't be parsed as FTS syntax.
func ftsQuote(query string) string {
	terms := strings.Fields(query)
	quoted := make([]string, 0, len(terms))
	for _, t := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(t, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " ")
}

// resolveCollectionKey resolves a user-supplied collection reference — exact
// key first, then case-insensitive name match — against the local cache.
func resolveCollectionKey(ctx context.Context, db *store.Store, ref string) (string, error) {
	// Dedicated columns are empty on this envelope-nested API — the id
	// column holds the collection key and the name lives in the JSON.
	row := db.DB().QueryRowContext(ctx,
		`SELECT "id" FROM "collections" WHERE "id" = ?`, ref)
	var key string
	if err := row.Scan(&key); err == nil {
		return key, nil
	}
	rows, err := db.DB().QueryContext(ctx,
		`SELECT "id", COALESCE(json_extract("data", '$.data.name'), '')
		 FROM "collections"
		 WHERE LOWER(json_extract("data", '$.data.name')) LIKE LOWER(?)
		 ORDER BY 2 LIMIT 5`,
		"%"+ref+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var matches [][2]string
	for rows.Next() {
		var k, n string
		if err := rows.Scan(&k, &n); err != nil {
			return "", err
		}
		matches = append(matches, [2]string{k, n})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no collection matches %q (run 'collections tree' to browse)", ref)
	case 1:
		return matches[0][0], nil
	default:
		var names []string
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%s (%s)", m[1], m[0]))
		}
		return "", fmt.Errorf("collection %q is ambiguous: %s", ref, strings.Join(names, ", "))
	}
}

// itemInCollection reports whether the item (by key) belongs to the
// collection, per the collections array in its cached JSON.
func itemInCollection(ctx context.Context, db *store.Store, itemKey, collectionKey string) (bool, error) {
	var n int
	err := db.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM "items" i
		 WHERE i."id" = ? AND EXISTS (
		   SELECT 1 FROM json_each(COALESCE(json_extract(i."data", '$.data.collections'), '[]')) je
		   WHERE je.value = ?)`, itemKey, collectionKey).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("checking collection membership: %w", err)
	}
	return n > 0, nil
}

// requireLocalCache opens the store read-only and converts the two empty
// states (no DB file, open error) into actionable messages.
func requireLocalCache(ctx context.Context, cliName string) (*store.Store, error) {
	db, err := openStoreForRead(ctx, cliName)
	if err != nil {
		return nil, fmt.Errorf("opening local cache: %w", err)
	}
	if db == nil {
		return nil, fmt.Errorf("local cache is empty — run '%s sync --full' first", cliName)
	}
	return db, nil
}
