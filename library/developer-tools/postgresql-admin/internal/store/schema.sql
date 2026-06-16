-- Copyright 2026 Kieran and contributors. Licensed under Apache-2.0. See LICENSE.
-- SQLite schema for local cache (KTD-3)

-- Metadata table for schema versioning
CREATE TABLE IF NOT EXISTS _schema_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Schema objects cache (from pg_namespace, pg_class, etc.)
CREATE TABLE IF NOT EXISTS schemas (
    oid INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    owner TEXT,
    description TEXT,
    cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tables cache
CREATE TABLE IF NOT EXISTS tables (
    oid INTEGER PRIMARY KEY,
    schema_oid INTEGER NOT NULL,
    name TEXT NOT NULL,
    owner TEXT,
    row_count INTEGER,
    total_size TEXT,
    description TEXT,
    cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (schema_oid) REFERENCES schemas(oid)
);

-- Columns cache
CREATE TABLE IF NOT EXISTS columns (
    oid INTEGER PRIMARY KEY,
    table_oid INTEGER NOT NULL,
    name TEXT NOT NULL,
    type_name TEXT,
    is_nullable BOOLEAN,
    default_value TEXT,
    description TEXT,
    cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (table_oid) REFERENCES tables(oid)
);

-- Indexes cache
CREATE TABLE IF NOT EXISTS indexes (
    oid INTEGER PRIMARY KEY,
    table_oid INTEGER NOT NULL,
    name TEXT NOT NULL,
    is_unique BOOLEAN,
    is_primary BOOLEAN,
    definition TEXT,
    cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (table_oid) REFERENCES tables(oid)
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_tables_schema_oid ON tables(schema_oid);
CREATE INDEX IF NOT EXISTS idx_columns_table_oid ON columns(table_oid);
CREATE INDEX IF NOT EXISTS idx_indexes_table_oid ON indexes(table_oid);
