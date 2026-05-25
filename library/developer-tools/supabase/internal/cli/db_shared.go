// PATCH: shared helpers for the novel db safe-mutation lifecycle (SQL loading, receipt storage paths).
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/dbchange"
)

// loadSQL resolves the SQL to operate on from either a positional file arg or
// the --sql flag. Exactly one must be provided.
func loadSQL(args []string, inlineSQL string) (sql string, sourceLabel string, err error) {
	hasFile := len(args) > 0 && args[0] != ""
	hasInline := strings.TrimSpace(inlineSQL) != ""
	switch {
	case hasFile && hasInline:
		return "", "", usageErr(fmt.Errorf("provide either a SQL file argument or --sql, not both"))
	case hasFile:
		b, rerr := os.ReadFile(args[0])
		if rerr != nil {
			return "", "", notFoundErr(fmt.Errorf("reading SQL file %q: %w", args[0], rerr))
		}
		return string(b), args[0], nil
	case hasInline:
		return inlineSQL, "--sql", nil
	default:
		return "", "", usageErr(fmt.Errorf("no SQL provided: pass a .sql file path or --sql \"...\""))
	}
}

// receiptDir is the on-disk home for receipt artifacts. Co-located with the
// local SQLite store under the user's data dir.
func receiptDir() string {
	base := filepath.Dir(defaultDBPath("supabase-pp-cli"))
	return filepath.Join(base, "receipts")
}

// receipt is the persisted attestation + reversal artifact written by
// `db apply` (and standalone by `db receipt`). `db revert` consumes it.
type receipt struct {
	Receipt   string             `json:"receipt_id"`
	CreatedAt string             `json:"created_at"`
	Ref       string             `json:"project_ref"`
	PlanHash  string             `json:"plan_hash"`
	Source    string             `json:"source,omitempty"`
	Applied   bool               `json:"applied"`
	// PATCH(amend-2026-05-25: record the approver in the audit receipt) —
	// Authorization records how the apply was authorized:
	// "human:approved-plan-hash" or "policy:<name>". When a policy
	// self-authorizes an apply, the receipt is the attestation of which envelope
	// cleared it.
	Authorization string           `json:"authorization,omitempty"`
	Manifest  *dbchange.Manifest `json:"manifest"`
	// Snapshots holds prior object definitions captured before mutating, keyed
	// by object identifier. Populated best-effort for RevSnapshot statements.
	Snapshots map[string]string `json:"snapshots,omitempty"`
	// InverseSQL is the assembled undo script (mechanical inverses in reverse
	// statement order). Empty when the change has irreversible statements.
	InverseSQL string `json:"inverse_sql,omitempty"`
}

func newReceiptID(ref string) string {
	return fmt.Sprintf("%s-%s", ref, time.Now().UTC().Format("20060102T150405Z"))
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func writeReceipt(r *receipt) (string, error) {
	dir := receiptDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating receipt dir: %w", err)
	}
	path := filepath.Join(dir, r.Receipt+".json")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", fmt.Errorf("writing receipt: %w", err)
	}
	return path, nil
}

// resolveReceiptPath accepts either a receipt id or a path to the receipt file.
func resolveReceiptPath(idOrPath string) string {
	if strings.HasSuffix(idOrPath, ".json") || strings.Contains(idOrPath, string(os.PathSeparator)) {
		return idOrPath
	}
	return filepath.Join(receiptDir(), idOrPath+".json")
}

func readReceipt(idOrPath string) (*receipt, error) {
	path := resolveReceiptPath(idOrPath)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, notFoundErr(fmt.Errorf("reading receipt %q: %w", idOrPath, err))
	}
	var r receipt
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("parsing receipt %q: %w", path, err)
	}
	return &r, nil
}
