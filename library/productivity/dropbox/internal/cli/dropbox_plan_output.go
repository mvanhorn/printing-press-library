package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
)

// A preview mixing mkdir and move ops falls under --agent's 80% key-frequency
// rule, which would strip from/to from the moves; keep the fields that say
// what each op does.
var planOpKeepFields = []string{"op", "path", "from", "to", "rev", "url", "keeper"}

type planOutput struct {
	PlanPath    string        `json:"plan_path"`
	PlanWritten bool          `json:"plan_written"`
	PlanOps     int           `json:"plan_ops"`
	Plan        *dropbox.Plan `json:"plan,omitempty"`
}

func outputPlan(path, source string, ops []dropbox.Op, force, printPlan bool) (planOutput, error) {
	if ops == nil {
		ops = make([]dropbox.Op, 0)
	}
	ops = withoutCoveredDeletes(ops)
	p := dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: source, Ops: ops}
	result := planOutput{PlanPath: path, PlanOps: len(ops)}
	if printPlan {
		result.Plan = &p
	}
	if path == "" {
		return result, nil
	}
	if _, err := os.Stat(path); err == nil {
		if _, err := dropbox.ReadPlan(path); err != nil && !force {
			return result, usageErr(fmt.Errorf("%s is not a valid plan file; use --force to overwrite: %w", path, err))
		}
	} else if !os.IsNotExist(err) {
		return result, err
	}
	if err := dropbox.WritePlan(path, p); err != nil {
		return result, err
	}
	result.PlanWritten = true
	return result, nil
}

func withoutCoveredDeletes(ops []dropbox.Op) []dropbox.Op {
	deletes := make(map[string]bool)
	for _, op := range ops {
		if op.Op == "delete" {
			deletes[strings.ToLower(strings.TrimRight(op.Path, "/"))] = true
		}
	}
	seen := make(map[string]bool)
	out := make([]dropbox.Op, 0, len(ops))
	for _, op := range ops {
		if op.Op != "delete" {
			out = append(out, op)
			continue
		}
		p := strings.ToLower(strings.TrimRight(op.Path, "/"))
		if seen[p] {
			continue
		}
		covered := false
		for parent, _ := dropbox.ParentBase(p); parent != ""; parent, _ = dropbox.ParentBase(parent) {
			if deletes[parent] {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, op)
			seen[p] = true
		}
	}
	return out
}
