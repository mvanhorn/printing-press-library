// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/client"
	"io"
	"time"
)

var EntityTypes = []struct {
	Name      string
	TableName string
}{
	{"Customer", "customers"},
	{"Vendor", "vendors"},
	{"Account", "accounts"},
	{"Invoice", "invoices"},
	{"Payment", "payments"},
	{"Bill", "bills"},
	{"Purchase", "purchases"},
	{"JournalEntry", "journal_entries"},
}

type CDCResponseEnvelope struct {
	CDCResponse []struct {
		QueryResponse []map[string]json.RawMessage `json:"QueryResponse"`
	} `json:"CDCResponse"`
}

type QueryResponseEnvelope struct {
	QueryResponse map[string]json.RawMessage `json:"QueryResponse"`
}

func Sync(ctx context.Context, c *client.Client, s *Store, out io.Writer) error {
	lastSync, err := s.GetLastSyncTime()
	if err != nil {
		return fmt.Errorf("getting last sync time: %w", err)
	}

	start := time.Now()

	if lastSync.IsZero() {
		fmt.Fprintln(out, "No previous sync found. Performing initial bulk sync of all entities...")
		for _, et := range EntityTypes {
			fmt.Fprintf(out, "Syncing %s...\n", et.Name)
			pos := 1
			count := 0
			for {
				qboSQL := fmt.Sprintf("SELECT * FROM %s STARTPOSITION %d MAXRESULTS 100", et.Name, pos)
				data, err := c.Get(ctx, "/query", map[string]string{"query": qboSQL})
				if err != nil {
					return fmt.Errorf("querying %s (pos %d): %w", et.Name, pos, err)
				}

				var env QueryResponseEnvelope
				if err := json.Unmarshal(data, &env); err != nil {
					return fmt.Errorf("unmarshaling query response: %w", err)
				}

				entityData, exists := env.QueryResponse[et.Name]
				if !exists || len(entityData) == 0 {
					break
				}

				var list []json.RawMessage
				if err := json.Unmarshal(entityData, &list); err != nil {
					return fmt.Errorf("unmarshaling entity list: %w", err)
				}

				if len(list) == 0 {
					break
				}

				tx, err := s.db.Begin()
				if err != nil {
					return fmt.Errorf("beginning transaction: %w", err)
				}
				for _, item := range list {
					id, name, docNum, lastUpdated, rawJSON := parseRawEntity(item)
					if id == "" {
						continue
					}
					query := fmt.Sprintf(`
						INSERT INTO %s (id, name, doc_number, last_updated, raw_json)
						VALUES (?, ?, ?, ?, ?)
						ON CONFLICT(id) DO UPDATE SET
							name = excluded.name,
							doc_number = excluded.doc_number,
							last_updated = excluded.last_updated,
							raw_json = excluded.raw_json
					`, et.TableName)
					if _, err := tx.ExecContext(ctx, query, id, name, docNum, lastUpdated, rawJSON); err != nil {
						tx.Rollback()
						return fmt.Errorf("saving %s (id %s): %w", et.Name, id, err)
					}
					count++
				}
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("committing transaction: %w", err)
				}

				if len(list) < 100 {
					break
				}
				pos += 100
			}
			fmt.Fprintf(out, "  ✓ Synced %d %s records\n", count, et.Name)
		}
	} else {
		// Incremental Sync via CDC
		// QBO CDC changedSince must be ISO-8601 formatted
		sinceStr := lastSync.UTC().Format("2006-01-02T15:04:05Z")
		fmt.Fprintf(out, "Performing incremental sync since %s...\n", sinceStr)

		// Build comma-separated entity names
		entityNames := ""
		for i, et := range EntityTypes {
			if i > 0 {
				entityNames += ","
			}
			entityNames += et.Name
		}

		data, err := c.Get(ctx, "/cdc", map[string]string{
			"entities":     entityNames,
			"changedSince": sinceStr,
		})
		if err != nil {
			// If CDC returns error, fallback to full sync to keep data consistent
			fmt.Fprintf(out, "CDC endpoint returned error: %v. Falling back to full query sync...\n", err)
			return triggerFallbackFullSync(ctx, c, s, out)
		}

		var env CDCResponseEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("unmarshaling CDC response: %w", err)
		}

		counts := make(map[string]int)
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("beginning CDC transaction: %w", err)
		}
		for _, responseObj := range env.CDCResponse {
			for _, qr := range responseObj.QueryResponse {
				for qKey, qVal := range qr {
					// Find the matching EntityType
					var tableName string
					for _, et := range EntityTypes {
						if et.Name == qKey {
							tableName = et.TableName
							break
						}
					}
					if tableName == "" {
						continue
					}

					var list []json.RawMessage
					if err := json.Unmarshal(qVal, &list); err != nil {
						// Single entity updates might not be formatted as an array under CDC depending on API version quirks,
						// let's handle single object fallback.
						list = []json.RawMessage{qVal}
					}

					for _, item := range list {
						// Check status if deleted
						var parser struct {
							qboEntityParser
							Status string `json:"status"`
						}
						if err := json.Unmarshal(item, &parser); err != nil {
							continue
						}

						id := parser.ID
						if id == "" {
							continue
						}

						if parser.Status == "Deleted" {
							_, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName), id)
							if err != nil {
								tx.Rollback()
								return fmt.Errorf("deleting CDC %s (id %s): %w", qKey, id, err)
							}
							counts[qKey+" (Deleted)"]++
						} else {
							id, name, docNum, lastUpdated, rawJSON := parseRawEntity(item)
							query := fmt.Sprintf(`
								INSERT INTO %s (id, name, doc_number, last_updated, raw_json)
								VALUES (?, ?, ?, ?, ?)
								ON CONFLICT(id) DO UPDATE SET
									name = excluded.name,
									doc_number = excluded.doc_number,
									last_updated = excluded.last_updated,
									raw_json = excluded.raw_json
							`, tableName)
							if _, err := tx.ExecContext(ctx, query, id, name, docNum, lastUpdated, rawJSON); err != nil {
								tx.Rollback()
								return fmt.Errorf("saving CDC %s (id %s): %w", qKey, id, err)
							}
							counts[qKey]++
						}
					}
				}
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing CDC transaction: %w", err)
		}

		hasChanges := false
		for k, v := range counts {
			fmt.Fprintf(out, "  ✓ Updated %d %s records\n", v, k)
			hasChanges = true
		}
		if !hasChanges {
			fmt.Fprintln(out, "No changes detected.")
		}
	}

	// Update sync state to start time so we don't miss anything that changed during the run
	if err := s.SetLastSyncTime(start); err != nil {
		return fmt.Errorf("saving last sync time: %w", err)
	}

	fmt.Fprintln(out, "Sync completed successfully.")
	return nil
}

func triggerFallbackFullSync(ctx context.Context, c *client.Client, s *Store, out io.Writer) error {
	// To perform a fallback, we clear the sync state last sync time and call Sync recursively
	if err := s.SetLastSyncTime(time.Time{}); err != nil {
		return err
	}
	return Sync(ctx, c, s, out)
}

type qboMetadata struct {
	LastUpdatedTime string `json:"LastUpdatedTime"`
}

type qboEntityParser struct {
	ID            string       `json:"Id"`
	DocNumber     string       `json:"DocNumber"`
	PaymentRefNum string       `json:"PaymentRefNum"`
	DisplayName   string       `json:"DisplayName"`
	Name          string       `json:"Name"`
	MetaData      *qboMetadata `json:"MetaData"`
}

func parseRawEntity(data json.RawMessage) (id, name, docNum, lastUpdated, rawJSON string) {
	var item qboEntityParser
	if err := json.Unmarshal(data, &item); err != nil {
		return
	}
	id = item.ID
	if item.DocNumber != "" {
		docNum = item.DocNumber
	} else {
		docNum = item.PaymentRefNum
	}
	if item.DisplayName != "" {
		name = item.DisplayName
	} else {
		name = item.Name
	}
	if item.MetaData != nil {
		lastUpdated = normalizeToUTC(item.MetaData.LastUpdatedTime)
	}
	rawJSON = string(data)
	return
}

func normalizeToUTC(s string) string {
	if s == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return s
}
