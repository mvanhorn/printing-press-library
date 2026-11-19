package cli

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// This driver simulates a writer commit between every table read. A transaction
// pins all reads to the first version; standalone queries observe mixed versions.
type reviewSnapshotDriver struct{}
type reviewSnapshotConn struct {
	inTx    bool
	version int
}
type reviewSnapshotTx struct{ c *reviewSnapshotConn }
type reviewSnapshotRows struct {
	version int
	done    bool
}

func (reviewSnapshotDriver) Open(string) (driver.Conn, error) { return &reviewSnapshotConn{}, nil }
func (c *reviewSnapshotConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected prepare")
}
func (c *reviewSnapshotConn) Close() error { return nil }
func (c *reviewSnapshotConn) Begin() (driver.Tx, error) {
	c.inTx = true
	return &reviewSnapshotTx{c}, nil
}
func (c *reviewSnapshotConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if !opts.ReadOnly {
		return nil, fmt.Errorf("snapshot transaction must be read-only")
	}
	return c.Begin()
}
func (c *reviewSnapshotConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	version := 1
	if !c.inTx {
		c.version++
		version = c.version
	}
	return &reviewSnapshotRows{version: version}, nil
}
func (tx *reviewSnapshotTx) Commit() error    { tx.c.inTx = false; return nil }
func (tx *reviewSnapshotTx) Rollback() error  { tx.c.inTx = false; return nil }
func (*reviewSnapshotRows) Columns() []string { return []string{"data"} }
func (*reviewSnapshotRows) Close() error      { return nil }
func (r *reviewSnapshotRows) Next(dst []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dst[0] = fmt.Sprintf(`{"id":"snapshot-%d"}`, r.version)
	return nil
}
func init() { sql.Register("review-snapshot", reviewSnapshotDriver{}) }
func TestReviewMirrorSingleSnapshot(t *testing.T) {
	db, err := sql.Open("review-snapshot", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	payloads, files, empty, err := novelMirrorSnapshot(ctx, db)
	if err != nil || empty || len(files) != 4 {
		t.Fatalf("snapshot: %v %v %v", files, empty, err)
	}
	for name, data := range payloads {
		if !strings.Contains(string(data), "snapshot-1") {
			t.Fatalf("mixed snapshot in %s: %s", name, data)
		}
	}
}
