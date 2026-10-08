package dropbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Plan struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source"`
	Ops       []Op      `json:"ops"`
}

type Op struct {
	Op             string `json:"op"`
	Path           string `json:"path,omitempty"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	Rev            string `json:"rev,omitempty"`
	URL            string `json:"url,omitempty"`
	Reason         string `json:"reason,omitempty"`
	ExpectFiles    *int   `json:"expect_files,omitempty"`
	ExpectBytes    *int64 `json:"expect_bytes,omitempty"`
	Keeper         string `json:"keeper,omitempty"`
	ContentHash    string `json:"content_hash,omitempty"`
	ExpectTreeHash string `json:"expect_tree_hash,omitempty"`
	// ExpectPathTreeHash pins the relative paths, content hashes, and sizes
	// of every file under a deleted folder. Counts and byte totals alone let
	// a same-size edit through.
	ExpectPathTreeHash string `json:"expect_path_tree_hash,omitempty"`
	ExpectDangling     bool   `json:"expect_dangling,omitempty"`
}

func (p Plan) ValidateShape() error {
	if p.Version != 1 || p.CreatedAt.IsZero() || p.Source == "" || p.Ops == nil {
		return fmt.Errorf("plan requires version 1, created_at, source, and ops")
	}
	for i, op := range p.Ops {
		if (op.ExpectFiles == nil) != (op.ExpectBytes == nil) {
			return fmt.Errorf("op %d: expect_files and expect_bytes must be set together", i)
		}
		for field, value := range map[string]string{"path": op.Path, "from": op.From, "to": op.To, "keeper": op.Keeper} {
			if value != "" {
				if err := ValidateCanonicalPath(value); err != nil {
					return fmt.Errorf("op %d %s: %w", i, field, err)
				}
			}
		}
		switch op.Op {
		case "mkdir", "delete":
			if op.Path != "" {
				continue
			}
		case "move":
			if op.From != "" && op.To != "" {
				continue
			}
		case "revoke_link":
			if op.URL != "" {
				continue
			}
		}
		return fmt.Errorf("op %d: unknown operation or missing required field: %s", i, op.Op)
	}
	return nil
}

func ReadPlan(path string) (Plan, error) {
	b, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- the plan path is the operator's own argument; reading it is the command's job
	if err != nil {
		return Plan{}, err
	}
	var p Plan
	if err := json.Unmarshal(b, &p); err != nil {
		return Plan{}, err
	}
	return p, p.ValidateShape()
}

func WritePlan(path string, p Plan) error {
	if err := p.ValidateShape(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), ".dropbox-plan-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
