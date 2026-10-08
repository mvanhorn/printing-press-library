package dropbox

import (
	"fmt"
	"sort"
)

type planCheckRun struct {
	snapshot               PlanSnapshot
	state                  *planState
	opts                   CheckOptions
	report                 CheckReport
	deletes                map[string]bool
	deletePaths            []string
	mkdirs                 map[string]bool
	moveTouched            map[string]bool
	moveDescendantsTouched map[string]bool
	moveDests              map[string]bool
	priorDeletes           map[string]bool
	priorDeleteDescendants map[string]bool
}

func newPlanCheckRun(plan Plan, snapshot PlanSnapshot, opts CheckOptions) *planCheckRun {
	if opts.MaxOps <= 0 {
		opts.MaxOps = 5000
	}
	c := &planCheckRun{snapshot: snapshot, state: newPlanState(snapshot.Entries()), opts: opts,
		report:  CheckReport{OK: true, Results: make([]CheckResult, 0)},
		deletes: map[string]bool{}, mkdirs: map[string]bool{}, moveTouched: map[string]bool{},
		moveDests: map[string]bool{}, priorDeletes: map[string]bool{}, moveDescendantsTouched: map[string]bool{}, priorDeleteDescendants: map[string]bool{}}
	for _, op := range plan.Ops {
		if op.Op == "delete" {
			p := checkPath(op.Path)
			c.deletes[p] = true
			c.deletePaths = append(c.deletePaths, p)
		}
		if op.Op == "mkdir" {
			c.mkdirs[checkPath(op.Path)] = true
		}
	}
	sort.Strings(c.deletePaths)
	return c
}
func (c *planCheckRun) add(seq int, op, status, code, message string) {
	c.report.Results = append(c.report.Results, CheckResult{seq, op, status, code, message})
	if status == "error" {
		c.report.Errors++
		c.report.OK = false
	}
	if status == "warn" {
		c.report.Warnings++
	}
}
func withinAnyAncestor(p string, paths map[string]bool) bool {
	for p = checkPath(p); p != ""; p = checkParent(p) {
		if paths[p] {
			return true
		}
	}
	return false
}
func markAncestors(p string, paths map[string]bool) {
	for p = checkParent(checkPath(p)); p != ""; p = checkParent(p) {
		paths[p] = true
	}
}
func (c *planCheckRun) rootCheck(seq int, kind string, paths ...string) {
	if !c.opts.RequireComplete {
		return
	}
	complete, ok := c.snapshot.(interface{ RootComplete(string) bool })
	if !ok {
		return
	}
	for _, p := range paths {
		if p != "" && !complete.RootComplete(p) {
			c.add(seq, kind, "error", "index_incomplete", "index root is incomplete for "+p+"; run: dropbox-pp-cli index")
			return
		}
	}
}
func (c *planCheckRun) warnings(seq int, kind string, op Op) {
	for _, p := range []string{op.Path, op.From, op.To} {
		if dev, ok := DevDirKind(p); ok {
			c.add(seq, kind, "warn", "dev_dir", "path is inside "+dev+": "+p)
			return
		}
	}
	if kind != "move" && kind != "delete" {
		return
	}
	p := op.Path
	if kind == "move" {
		p = op.From
	}
	e, ok := c.state.get(p)
	if !ok || e.Tag != "folder" {
		return
	}
	found := false
	c.state.eachDescendant(p, func(_ string, child SnapshotEntry) { found = found || child.DevKind != "" })
	if found {
		c.add(seq, kind, "warn", "dev_dir", "folder contains a development directory: "+p)
	}
}
func (c *planCheckRun) destinationParent(seq int, kind, p string) {
	if q := checkParent(p); q != "" {
		if e, ok := c.state.get(q); !ok || e.Tag != "folder" {
			c.add(seq, kind, "error", "parent_missing", "destination parent is missing: "+q)
		}
	}
}
func (c *planCheckRun) mkdir(seq int, op Op, startErrors int) {
	c.destinationParent(seq, "mkdir", op.Path)
	key := checkPath(op.Path)
	lower, upper := key+"/", key+"0"
	i := sort.SearchStrings(c.deletePaths, lower)
	if withinAnyAncestor(key, c.deletes) || i < len(c.deletePaths) && c.deletePaths[i] < upper {
		c.add(seq, "mkdir", "error", "plan_conflict", "mkdir conflicts with delete: "+op.Path)
	}
	if e, ok := c.state.get(key); ok {
		if e.Tag == "folder" {
			c.add(seq, "mkdir", "warn", "mkdir_exists", "folder already exists: "+op.Path)
		} else {
			c.add(seq, "mkdir", "error", "dest_exists", "destination exists: "+op.Path)
		}
	} else if c.report.Errors == startErrors {
		parent, _ := c.state.get(checkParent(key))
		share := parent.SharedFolderID
		if share == "" {
			share = parent.ParentSharedFolderID
		}
		c.state.put(key, &SnapshotEntry{Tag: "folder", ParentSharedFolderID: share})
	}
}
func (c *planCheckRun) move(seq int, op Op, startErrors int) {
	from, to := checkPath(op.From), checkPath(op.To)
	if withinAnyAncestor(from, c.moveTouched) || withinAnyAncestor(to, c.moveTouched) || c.moveDescendantsTouched[from] || c.moveDescendantsTouched[to] {
		c.add(seq, "move", "error", "batch_dependency", "move touches a path used by an earlier move")
	}
	src, exists := c.state.get(from)
	if !exists {
		c.add(seq, "move", "error", "source_missing", "source is missing: "+op.From)
	}
	if exists && op.Rev != "" && src.Rev != op.Rev {
		c.add(seq, "move", "error", "rev_mismatch", "source revision changed: "+op.From)
	}
	c.destinationParent(seq, "move", op.To)
	if c.moveDests[to] {
		c.add(seq, "move", "error", "dest_collision", "another move has this destination: "+op.To)
	}
	c.moveDests[to] = true
	if _, occupied := c.state.get(to); occupied && from != to {
		c.add(seq, "move", "error", "dest_exists", "destination exists: "+op.To)
	}
	if from == to && op.From != op.To {
		c.add(seq, "move", "warn", "case_only_rename", "case-only rename: "+op.To)
	}
	if src.Tag == "folder" && PathWithin(to, from) && from != to {
		c.add(seq, "move", "error", "dest_collision", "folder cannot move into itself: "+op.To)
	}
	lower, upper := from+"/", from+"0"
	i := sort.SearchStrings(c.deletePaths, lower)
	if c.deletes[from] || i < len(c.deletePaths) && c.deletePaths[i] < upper || withinAnyAncestor(to, c.deletes) {
		c.add(seq, "move", "error", "plan_conflict", "move source contains a deleted path or destination lies inside a deleted path")
	}
	c.rootCheck(seq, "move", op.From, op.To)
	if exists {
		parent, _ := c.state.get(checkParent(to))
		srcShare, dstShare := src.SharedFolderID, parent.SharedFolderID
		if srcShare == "" {
			srcShare = src.ParentSharedFolderID
		}
		if dstShare == "" {
			dstShare = parent.ParentSharedFolderID
		}
		if srcShare != dstShare {
			c.add(seq, "move", "warn", "cross_share", "source and destination have different shared folder contexts")
		}
	}
	c.moveTouched[from], c.moveTouched[to] = true, true
	markAncestors(from, c.moveDescendantsTouched)
	markAncestors(to, c.moveDescendantsTouched)
	if c.report.Errors == startErrors && from != to {
		c.state.move(from, to)
	}
}
func (c *planCheckRun) delete(seq int, op Op, startErrors int) {
	key := checkPath(op.Path)
	c.priorDeletes[key] = true
	markAncestors(key, c.priorDeleteDescendants)
	c.rootCheck(seq, "delete", op.Path)
	src, exists := c.state.get(key)
	if !exists {
		c.add(seq, "delete", "error", "source_missing", "source is missing: "+op.Path)
	}
	if exists && op.Rev != "" && src.Rev != op.Rev {
		c.add(seq, "delete", "error", "rev_mismatch", "source revision changed: "+op.Path)
	}
	if c.mkdirs[key] {
		c.add(seq, "delete", "error", "plan_conflict", "delete conflicts with mkdir: "+op.Path)
	}
	if op.Keeper != "" {
		if withinAnyAncestor(op.Keeper, c.deletes) {
			c.add(seq, "delete", "error", "keeper_deleted", "keeper is deleted by this plan: "+op.Keeper)
		}
		keeper, ok := c.state.get(op.Keeper)
		if !ok {
			c.add(seq, "delete", "error", "keeper_missing", "keeper is missing: "+op.Keeper)
		} else if op.ExpectTreeHash != "" {
			if c.state.treeHash(op.Keeper) != op.ExpectTreeHash {
				c.add(seq, "delete", "error", "keeper_tree_mismatch", "keeper tree changed from planned original")
			}
		} else if op.ContentHash == "" || keeper.ContentHash != op.ContentHash {
			c.add(seq, "delete", "error", "keeper_hash_mismatch", "keeper content hash differs from planned duplicate")
		}
	}
	if !exists {
		return
	}
	if op.ExpectPathTreeHash != "" && c.state.treeHash(key) != op.ExpectPathTreeHash {
		c.add(seq, "delete", "error", "delete_tree_mismatch", "files under the delete target changed since the plan was written: "+op.Path)
	}
	shared, files, descendants := false, 0, 0
	var bytes int64
	c.state.eachWithin(key, func(p string, e SnapshotEntry) {
		if e.SharedFolderID != "" {
			shared = true
		}
		if p != key {
			descendants++
		}
		if p != key && e.Tag == "file" {
			files++
			bytes += e.Size
		}
	})
	if shared && !c.opts.AllowUnshare {
		c.add(seq, "delete", "error", "shared_folder_delete", "delete target contains a shared folder")
	}
	if src.Tag == "folder" && descendants > 0 {
		if op.ExpectFiles != nil && op.ExpectBytes != nil && *op.ExpectFiles == files && *op.ExpectBytes == bytes {
			c.add(seq, "delete", "warn", "delete_nonempty_attested", fmt.Sprintf("folder contains %d indexed files (%d bytes), as attested", files, bytes))
		} else if files > 0 {
			message := fmt.Sprintf("folder contains %d indexed files (%d bytes)", files, bytes)
			if c.opts.AllowNonemptyDelete && op.ExpectFiles == nil && op.ExpectBytes == nil {
				c.add(seq, "delete", "warn", "delete_nonempty_allowed", message+", allowed by --allow-nonempty-delete")
			} else {
				c.add(seq, "delete", "error", "delete_nonempty_folder", message)
			}
		}
	}
	if c.report.Errors == startErrors {
		c.state.delete(key)
	}
}
func (c *planCheckRun) revoke(seq int, op Op) {
	p, ok := c.snapshot.LinkPath(op.URL)
	if !ok {
		c.add(seq, "revoke_link", "error", "unknown_link", "link is not in the local index")
		return
	}
	if op.ExpectDangling {
		if _, exists := c.state.base[checkPath(p)]; exists {
			c.add(seq, "revoke_link", "error", "link_target_exists", "linked path still exists: "+p)
		}
	}
}
func (c *planCheckRun) run(plan Plan) CheckReport {
	if len(plan.Ops) > c.opts.MaxOps {
		c.add(0, "plan", "error", "op_cap", fmt.Sprintf("%d operations exceed --max-ops %d", len(plan.Ops), c.opts.MaxOps))
	}
	for _, kind := range []string{"mkdir", "move", "delete", "revoke_link"} {
		for i, op := range plan.Ops {
			if op.Op != kind {
				continue
			}
			seq, startErrors, startResults := i+1, c.report.Errors, len(c.report.Results)
			if kind == "delete" && withinAnyAncestor(op.Path, c.priorDeletes) {
				c.add(seq, kind, "covered", "covered", "covered by an earlier delete: "+op.Path)
				continue
			}
			if kind == "delete" && c.priorDeleteDescendants[checkPath(op.Path)] {
				c.add(seq, kind, "error", "batch_dependency", "delete overlaps a path used by an earlier delete")
			}
			c.warnings(seq, kind, op)
			switch kind {
			case "mkdir":
				c.mkdir(seq, op, startErrors)
			case "move":
				c.move(seq, op, startErrors)
			case "delete":
				c.delete(seq, op, startErrors)
			case "revoke_link":
				c.revoke(seq, op)
			}
			if len(c.report.Results) == startResults {
				c.add(seq, kind, "ok", "ok", "ready")
			}
		}
	}
	for i, op := range plan.Ops {
		switch op.Op {
		case "mkdir", "move", "delete", "revoke_link":
		default:
			c.add(i+1, op.Op, "error", "unknown_op", "unsupported operation")
		}
	}
	return c.report
}
