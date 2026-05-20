package store

import "database/sql"

func Open(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}
