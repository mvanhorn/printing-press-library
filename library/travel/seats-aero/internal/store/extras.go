// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
)

const extrasVersion = 2

const availabilityAllViewSQL = `CREATE VIEW availability_all AS
	SELECT "id", "data", "synced_at", "created_at", "date",
		"f_airlines", "f_available", "f_available_raw", "f_direct", "f_direct_airlines", "f_direct_mileage_cost", "f_direct_mileage_cost_raw", "f_direct_remaining_seats", "f_direct_remaining_seats_raw", "f_direct_total_taxes", "f_direct_total_taxes_raw", "f_mileage_cost", "f_mileage_cost_raw", "f_remaining_seats", "f_remaining_seats_raw", "f_total_taxes", "f_total_taxes_raw",
		"j_airlines", "j_available", "j_available_raw", "j_direct", "j_direct_airlines", "j_direct_mileage_cost", "j_direct_mileage_cost_raw", "j_direct_remaining_seats", "j_direct_remaining_seats_raw", "j_direct_total_taxes", "j_direct_total_taxes_raw", "j_mileage_cost", "j_mileage_cost_raw", "j_remaining_seats", "j_remaining_seats_raw", "j_total_taxes", "j_total_taxes_raw",
		"parsed_date", "route_id", "source", "taxes_currency", "updated_at",
		"w_airlines", "w_available", "w_available_raw", "w_direct", "w_direct_airlines", "w_direct_mileage_cost", "w_direct_mileage_cost_raw", "w_direct_remaining_seats", "w_direct_remaining_seats_raw", "w_direct_total_taxes", "w_direct_total_taxes_raw", "w_mileage_cost", "w_mileage_cost_raw", "w_remaining_seats", "w_remaining_seats_raw", "w_total_taxes", "w_total_taxes_raw",
		"y_airlines", "y_available", "y_available_raw", "y_direct", "y_direct_airlines", "y_direct_mileage_cost", "y_direct_mileage_cost_raw", "y_direct_remaining_seats", "y_direct_remaining_seats_raw", "y_direct_total_taxes", "y_direct_total_taxes_raw", "y_mileage_cost", "y_mileage_cost_raw", "y_remaining_seats", "y_remaining_seats_raw", "y_total_taxes", "y_total_taxes_raw"
	FROM "availability" a
	WHERE NOT EXISTS (SELECT 1 FROM "awards" w WHERE w.id = a.id AND COALESCE(datetime(w.synced_at),'0000-01-01') > COALESCE(datetime(a.synced_at),'0000-01-01'))
	UNION ALL
	SELECT "id", "data", "synced_at", "created_at", "date",
		"f_airlines", "f_available", "f_available_raw", "f_direct", "f_direct_airlines", "f_direct_mileage_cost", "f_direct_mileage_cost_raw", "f_direct_remaining_seats", "f_direct_remaining_seats_raw", "f_direct_total_taxes", "f_direct_total_taxes_raw", "f_mileage_cost", "f_mileage_cost_raw", "f_remaining_seats", "f_remaining_seats_raw", "f_total_taxes", "f_total_taxes_raw",
		"j_airlines", "j_available", "j_available_raw", "j_direct", "j_direct_airlines", "j_direct_mileage_cost", "j_direct_mileage_cost_raw", "j_direct_remaining_seats", "j_direct_remaining_seats_raw", "j_direct_total_taxes", "j_direct_total_taxes_raw", "j_mileage_cost", "j_mileage_cost_raw", "j_remaining_seats", "j_remaining_seats_raw", "j_total_taxes", "j_total_taxes_raw",
		"parsed_date", "route_id", "source", "taxes_currency", "updated_at",
		"w_airlines", "w_available", "w_available_raw", "w_direct", "w_direct_airlines", "w_direct_mileage_cost", "w_direct_mileage_cost_raw", "w_direct_remaining_seats", "w_direct_remaining_seats_raw", "w_direct_total_taxes", "w_direct_total_taxes_raw", "w_mileage_cost", "w_mileage_cost_raw", "w_remaining_seats", "w_remaining_seats_raw", "w_total_taxes", "w_total_taxes_raw",
		"y_airlines", "y_available", "y_available_raw", "y_direct", "y_direct_airlines", "y_direct_mileage_cost", "y_direct_mileage_cost_raw", "y_direct_remaining_seats", "y_direct_remaining_seats_raw", "y_direct_total_taxes", "y_direct_total_taxes_raw", "y_mileage_cost", "y_mileage_cost_raw", "y_remaining_seats", "y_remaining_seats_raw", "y_total_taxes", "y_total_taxes_raw"
	FROM "awards" w
	WHERE NOT EXISTS (SELECT 1 FROM "availability" a WHERE a.id = w.id AND COALESCE(datetime(a.synced_at),'0000-01-01') >= COALESCE(datetime(w.synced_at),'0000-01-01'))`

func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	exec := func(statement string) error {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
		return nil
	}
	if err := exec(`CREATE TABLE IF NOT EXISTS store_extras_meta(key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	var current int
	if err := conn.QueryRowContext(ctx, `SELECT COALESCE((SELECT CAST(value AS INTEGER) FROM store_extras_meta WHERE key = 'extras_version'), 0)`).Scan(&current); err != nil {
		return fmt.Errorf("read extras version: %w", err)
	}
	if err := exec(`CREATE TABLE IF NOT EXISTS availability_first_seen (id TEXT PRIMARY KEY, first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, source_table TEXT NOT NULL)`); err != nil {
		return err
	}
	if current < extrasVersion {
		for _, q := range []string{`DROP TRIGGER IF EXISTS trg_availability_first_seen`, `DROP TRIGGER IF EXISTS trg_awards_first_seen`} {
			if err := exec(q); err != nil {
				return err
			}
		}
	}
	for _, q := range []string{
		`CREATE TRIGGER IF NOT EXISTS trg_availability_first_seen AFTER INSERT ON "availability" WHEN NOT EXISTS (SELECT 1 FROM availability_first_seen WHERE id = NEW.id) BEGIN INSERT INTO availability_first_seen(id, first_seen_at, source_table) VALUES (NEW.id, strftime('%Y-%m-%dT%H:%M:%SZ','now'), 'availability'); END`,
		`CREATE TRIGGER IF NOT EXISTS trg_awards_first_seen AFTER INSERT ON "awards" WHEN NOT EXISTS (SELECT 1 FROM availability_first_seen WHERE id = NEW.id) BEGIN INSERT INTO availability_first_seen(id, first_seen_at, source_table) VALUES (NEW.id, strftime('%Y-%m-%dT%H:%M:%SZ','now'), 'awards'); END`,
	} {
		if err := exec(q); err != nil {
			return err
		}
	}
	resourcesExist, err := tableExists(ctx, conn, "resources")
	if err != nil {
		return fmt.Errorf("inspect resources table: %w", err)
	}
	if current < extrasVersion {
		if resourcesExist {
			if err := exec(`INSERT OR IGNORE INTO availability_first_seen(id, first_seen_at, source_table) SELECT id, strftime('%Y-%m-%dT%H:%M:%SZ', datetime(COALESCE(synced_at,'now'))), resource_type FROM resources WHERE resource_type IN ('availability','awards') AND id IS NOT NULL`); err != nil {
				return err
			}
		}
		for _, q := range []string{
			`INSERT OR IGNORE INTO availability_first_seen(id, first_seen_at, source_table) SELECT id, strftime('%Y-%m-%dT%H:%M:%SZ', datetime(COALESCE(synced_at,'now'))), 'availability' FROM "availability"`,
			`INSERT OR IGNORE INTO availability_first_seen(id, first_seen_at, source_table) SELECT id, strftime('%Y-%m-%dT%H:%M:%SZ', datetime(COALESCE(synced_at,'now'))), 'awards' FROM "awards"`,
			`UPDATE availability_first_seen SET first_seen_at = strftime('%Y-%m-%dT%H:%M:%SZ', datetime(first_seen_at)) WHERE first_seen_at NOT LIKE '____-__-__T__:__:__Z' AND datetime(first_seen_at) IS NOT NULL`,
		} {
			if err := exec(q); err != nil {
				return err
			}
		}
	}
	for _, q := range []string{
		`DROP INDEX IF EXISTS idx_availability_route_date`, `DROP INDEX IF EXISTS idx_awards_route_date`,
		`CREATE INDEX IF NOT EXISTS idx_availability_od_date ON "availability"(json_extract(data,'$.Route.OriginAirport'), json_extract(data,'$.Route.DestinationAirport'), date)`,
		`CREATE INDEX IF NOT EXISTS idx_awards_od_date ON "awards"(json_extract(data,'$.Route.OriginAirport'), json_extract(data,'$.Route.DestinationAirport'), date)`,
		`CREATE INDEX IF NOT EXISTS idx_availability_first_seen_at ON availability_first_seen(first_seen_at)`,
		`CREATE INDEX IF NOT EXISTS idx_availability_source ON "availability"(source)`, `CREATE INDEX IF NOT EXISTS idx_awards_source ON "awards"(source)`,
	} {
		if err := exec(q); err != nil {
			return err
		}
	}
	if resourcesExist {
		var hasUpdatedAt int
		if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pragma_table_info('resources') WHERE name = 'updated_at')`).Scan(&hasUpdatedAt); err != nil {
			return fmt.Errorf("inspect resources columns: %w", err)
		}
		q := `CREATE INDEX IF NOT EXISTS idx_resources_type_synced ON resources(resource_type, synced_at)`
		if hasUpdatedAt != 0 {
			q = `CREATE INDEX IF NOT EXISTS idx_resources_type_updated ON resources(resource_type, updated_at)`
		}
		if err := exec(q); err != nil {
			return err
		}
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(availabilityAllViewSQL))
	desiredHash := fmt.Sprintf("%016x", h.Sum64())
	var storedHash string
	_ = conn.QueryRowContext(ctx, `SELECT value FROM store_extras_meta WHERE key = 'view_hash'`).Scan(&storedHash)
	var viewExists int
	if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'view' AND name = 'availability_all')`).Scan(&viewExists); err != nil {
		return fmt.Errorf("inspect availability_all view: %w", err)
	}
	if storedHash != desiredHash || viewExists == 0 {
		if err := exec(`DROP VIEW IF EXISTS availability_all`); err != nil {
			return err
		}
		if err := exec(availabilityAllViewSQL); err != nil {
			return err
		}
		if err := exec(fmt.Sprintf(`INSERT OR REPLACE INTO store_extras_meta(key,value) VALUES('view_hash','%s')`, desiredHash)); err != nil {
			return err
		}
	}
	return exec(`INSERT OR REPLACE INTO store_extras_meta(key,value) VALUES('extras_version','2')`)
}
