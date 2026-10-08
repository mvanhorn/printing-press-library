package dropbox

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCheckPlanKeeperDeletedRuleScale(t *testing.T) {
	if testing.Short() {
		t.Skip("5,000 keeper checks")
	}
	entries := make(map[string]SnapshotEntry, 10000)
	ops := make([]Op, 0, 5000)
	for i := 0; i < 5000; i++ {
		remove := fmt.Sprintf("/remove/%05d", i)
		keep := fmt.Sprintf("/keep/%05d", i)
		entries[remove] = SnapshotEntry{Tag: "file", ContentHash: "same"}
		entries[keep] = SnapshotEntry{Tag: "file", ContentHash: "same"}
		ops = append(ops, Op{Op: "delete", Path: remove, Keeper: keep, ContentHash: "same"})
	}
	started := time.Now()
	report := CheckPlan(Plan{Ops: ops}, testSnapshot{entries: entries}, CheckOptions{MaxOps: 5000})
	duration := time.Since(started)
	if !report.OK || len(report.Results) != len(ops) || duration >= time.Second {
		t.Fatalf("keeper rule: ok=%t results=%d duration=%s errors=%d", report.OK, len(report.Results), duration, report.Errors)
	}
	t.Logf("5,000 keeper checks: %s", duration)
}

type testSnapshot struct {
	entries map[string]SnapshotEntry
	links   map[string]string
	files   []string
}

func (s testSnapshot) Entries() map[string]SnapshotEntry { return s.entries }
func (s testSnapshot) LinkPath(u string) (string, bool)  { p, ok := s.links[u]; return p, ok }
func (s testSnapshot) HasDevDescendant(p string) (bool, error) {
	for q, e := range s.entries {
		if strings.HasPrefix(q, strings.ToLower(p)+"/") && e.DevKind != "" {
			return true, nil
		}
	}
	return false, nil
}
func TestCheckPlanCases(t *testing.T) {
	s := testSnapshot{entries: map[string]SnapshotEntry{"/a": {Tag: "folder"}, "/a/f": {Tag: "file", Rev: "new", ParentSharedFolderID: "share"}, "/b": {Tag: "folder"}}, links: map[string]string{}}
	for _, tc := range []struct {
		name string
		ops  []Op
		code string
		ok   bool
	}{
		{"rev mismatch", []Op{{Op: "move", From: "/a/f", To: "/b/f", Rev: "old"}}, "rev_mismatch", false},
		{"earlier mkdir", []Op{{Op: "mkdir", Path: "/b/new"}, {Op: "move", From: "/a/f", To: "/b/new/f"}}, "cross_share", true},
		{"collision", []Op{{Op: "move", From: "/a/f", To: "/b/f"}, {Op: "move", From: "/a/f", To: "/b/f"}}, "dest_collision", false},
		{"clean", []Op{{Op: "move", From: "/a/f", To: "/a/g", Rev: "new"}}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := CheckPlan(Plan{Ops: tc.ops}, s, CheckOptions{MaxOps: 5000})
			if r.OK != tc.ok {
				t.Fatalf("report=%+v", r)
			}
			if tc.code != "" {
				found := false
				for _, x := range r.Results {
					if x.Code == tc.code {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s: %+v", tc.code, r)
				}
			}
		})
	}
	ops := make([]Op, 5001)
	for i := range ops {
		ops[i] = Op{Op: "revoke_link", URL: fmt.Sprint(i)}
	}
	r := CheckPlan(Plan{Ops: ops}, s, CheckOptions{MaxOps: 5000})
	if r.OK || r.Results[0].Code != "op_cap" {
		t.Fatalf("op cap: %+v", r.Results[0])
	}
}

func TestCheckPlanIntegrityBeforeMutation(t *testing.T) {
	s := testSnapshot{entries: map[string]SnapshotEntry{
		"/a": {Tag: "folder"}, "/a/f": {Tag: "file", Rev: "r"},
		"/b": {Tag: "folder"}, "/p": {Tag: "folder"},
		"/p/f":    {Tag: "file", Rev: "r"},
		"/shared": {Tag: "folder"}, "/shared/child": {Tag: "file", SharedFolderID: "share"},
	}, files: []string{"/a/f", "/p/f", "/shared/child"}}
	for _, tc := range []struct {
		name string
		ops  []Op
		code string
	}{
		{"moved source cannot move twice", []Op{{Op: "move", From: "/a/f", To: "/b/f"}, {Op: "move", From: "/a/f", To: "/b/g"}}, "source_missing"},
		{"delete descendant conflicts with folder move", []Op{{Op: "delete", Path: "/a/f"}, {Op: "move", From: "/a", To: "/b/a"}}, "plan_conflict"},
		{"delete and mkdir same path", []Op{{Op: "delete", Path: "/p/f"}, {Op: "mkdir", Path: "/p/f"}}, "plan_conflict"},
		{"nonempty folder requires acknowledgement", []Op{{Op: "delete", Path: "/a"}}, "delete_nonempty_folder"},
		{"shared descendant blocks delete", []Op{{Op: "delete", Path: "/shared"}}, "shared_folder_delete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := CheckPlan(Plan{Ops: tc.ops}, s, CheckOptions{})
			if r.OK {
				t.Fatalf("unsafe plan accepted: %+v", r)
			}
			found := false
			for _, item := range r.Results {
				if item.Code == tc.code && item.Status == "error" {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing error %s: %+v", tc.code, r)
			}
		})
	}
}

func TestCheckPlanAcknowledgementsLinksAndDuplicates(t *testing.T) {
	files, bytes := 1, int64(7)
	s := testSnapshot{entries: map[string]SnapshotEntry{
		"/docs": {Tag: "folder"}, "/docs/f": {Tag: "file", Size: 7, ContentHash: "same"},
		"/keeper": {Tag: "file", ContentHash: "same"},
		"/shared": {Tag: "folder"}, "/shared/nested": {Tag: "folder", SharedFolderID: "s"},
		"/repo": {Tag: "folder"}, "/repo/node_modules": {Tag: "folder", DevKind: "node_modules"},
	}, links: map[string]string{"url": "/docs/f"}}
	for _, tc := range []struct {
		name string
		ops  []Op
		opts CheckOptions
		code string
		ok   bool
	}{
		{"matched folder totals", []Op{{Op: "delete", Path: "/docs", ExpectFiles: &files, ExpectBytes: &bytes}}, CheckOptions{}, "", true},
		{"wrong byte total", []Op{{Op: "delete", Path: "/docs", ExpectFiles: &files, ExpectBytes: new(int64)}}, CheckOptions{}, "delete_nonempty_folder", false},
		{"nonempty override", []Op{{Op: "delete", Path: "/docs"}}, CheckOptions{AllowNonemptyDelete: true}, "", true},
		{"shared override", []Op{{Op: "delete", Path: "/shared"}}, CheckOptions{AllowUnshare: true}, "", true},
		{"missing keeper", []Op{{Op: "delete", Path: "/docs/f", Keeper: "/missing", ContentHash: "same"}}, CheckOptions{}, "keeper_missing", false},
		{"changed keeper", []Op{{Op: "delete", Path: "/docs/f", Keeper: "/keeper", ContentHash: "other"}}, CheckOptions{}, "keeper_hash_mismatch", false},
		{"link target still exists", []Op{{Op: "revoke_link", URL: "url", ExpectDangling: true}}, CheckOptions{}, "link_target_exists", false},
		{"link target existed at refresh", []Op{{Op: "delete", Path: "/docs/f"}, {Op: "revoke_link", URL: "url", ExpectDangling: true}}, CheckOptions{}, "link_target_exists", false},
		{"dev descendant", []Op{{Op: "move", From: "/repo", To: "/repo2"}}, CheckOptions{}, "dev_dir", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := CheckPlan(Plan{Ops: tc.ops}, s, tc.opts)
			if r.OK != tc.ok {
				t.Fatalf("report=%+v", r)
			}
			if tc.code != "" {
				found := false
				for _, item := range r.Results {
					if item.Code == tc.code {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s: %+v", tc.code, r)
				}
			}
		})
	}
}

func TestCheckPlanRoundTwoSafety(t *testing.T) {
	files, bytes := 1, int64(7)
	s := testSnapshot{entries: map[string]SnapshotEntry{"/a": {Tag: "folder"}, "/a/f": {Tag: "file", Size: 7, EntryID: "id-f"}, "/b": {Tag: "folder"}, "/empty": {Tag: "folder"}, "/empty/child": {Tag: "folder"}}, links: map[string]string{"live": "/a/f", "gone": "/gone"}}
	for _, tc := range []struct {
		name         string
		ops          []Op
		code, status string
	}{
		{"move dependency", []Op{{Op: "move", From: "/a/f", To: "/b/f"}, {Op: "move", From: "/b/f", To: "/b/g"}}, "batch_dependency", "error"},
		{"move ancestor dependency", []Op{{Op: "move", From: "/a/f", To: "/b/f"}, {Op: "move", From: "/a", To: "/b/a"}}, "batch_dependency", "error"},
		{"delete ancestor dependency", []Op{{Op: "delete", Path: "/a/f"}, {Op: "delete", Path: "/a"}}, "batch_dependency", "error"},
		{"covered delete", []Op{{Op: "delete", Path: "/a", ExpectFiles: &files, ExpectBytes: &bytes}, {Op: "delete", Path: "/a/f"}}, "covered", "covered"},
		{"attested delete", []Op{{Op: "delete", Path: "/a", ExpectFiles: &files, ExpectBytes: &bytes}}, "delete_nonempty_attested", "warn"},
		{"attested empty subtree", []Op{{Op: "delete", Path: "/empty", ExpectFiles: new(int), ExpectBytes: new(int64)}}, "delete_nonempty_attested", "warn"},
		{"public link", []Op{{Op: "revoke_link", URL: "live"}}, "ok", "ok"},
		{"dangling link now exists", []Op{{Op: "revoke_link", URL: "live", ExpectDangling: true}}, "link_target_exists", "error"},
		{"unknown link", []Op{{Op: "revoke_link", URL: "unknown"}}, "unknown_link", "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := CheckPlan(Plan{Ops: tc.ops}, s, CheckOptions{})
			for _, v := range r.Results {
				if v.Code == tc.code && v.Status == tc.status {
					return
				}
			}
			t.Fatalf("missing %s/%s: %+v", tc.code, tc.status, r)
		})
	}
}

func TestCheckPlanKeeperDeletedElsewhereAndAllowedWarning(t *testing.T) {
	s := testSnapshot{entries: map[string]SnapshotEntry{
		"/discard": {Tag: "folder"}, "/discard/duplicate": {Tag: "file", ContentHash: "same"},
		"/keep": {Tag: "folder"}, "/keep/copy": {Tag: "file", ContentHash: "same"},
	}}
	for _, deleted := range []string{"/keep", "/keep/copy"} {
		r := CheckPlan(Plan{Ops: []Op{{Op: "delete", Path: "/discard/duplicate", Keeper: "/keep/copy", ContentHash: "same"}, {Op: "delete", Path: deleted}}}, s, CheckOptions{})
		found := false
		for _, item := range r.Results {
			if item.Code == "keeper_deleted" && item.Status == "error" {
				found = true
			}
		}
		if !found {
			t.Fatalf("deleted %s: %+v", deleted, r)
		}
	}
	r := CheckPlan(Plan{Ops: []Op{{Op: "delete", Path: "/keep"}}}, s, CheckOptions{AllowNonemptyDelete: true})
	found := false
	for _, item := range r.Results {
		if item.Code == "delete_nonempty_allowed" && item.Status == "warn" && strings.Contains(item.Message, "1 indexed files") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing allowed warning: %+v", r)
	}
}

func TestCheckPlanAllowsMoveOutBeforeFolderDelete(t *testing.T) {
	s := testSnapshot{entries: map[string]SnapshotEntry{
		"/inbox": {Tag: "folder"}, "/inbox/a": {Tag: "file", Size: 3},
		"/archive": {Tag: "folder"},
	}}
	r := CheckPlan(Plan{Ops: []Op{{Op: "move", From: "/Inbox/a", To: "/Archive/a"}, {Op: "delete", Path: "/Inbox"}}}, s, CheckOptions{})
	if !r.OK {
		t.Fatalf("move out then delete should be valid: %+v", r)
	}
}
