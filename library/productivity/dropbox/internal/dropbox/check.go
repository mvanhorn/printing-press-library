package dropbox

type SnapshotEntry struct {
	Tag, Rev, EntryID, ContentHash, SharedFolderID, ParentSharedFolderID, DevKind string
	Size                                                                          int64
}

// PlanSnapshot is an immutable local index view.
type PlanSnapshot interface {
	Entries() map[string]SnapshotEntry
	LinkPath(url string) (string, bool)
	HasDevDescendant(path string) (bool, error)
}

type CheckOptions struct {
	MaxOps              int
	AllowNonemptyDelete bool
	AllowUnshare        bool
	RequireComplete     bool
}
type CheckResult struct {
	Seq     int    `json:"seq"`
	Op      string `json:"op"`
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type CheckReport struct {
	OK       bool          `json:"ok"`
	Results  []CheckResult `json:"results"`
	Errors   int           `json:"errors"`
	Warnings int           `json:"warnings"`
}

func checkPath(p string) string { return PathKey(p) }
func checkParent(p string) string {
	parent, _ := ParentBase(p)
	return parent
}

func CheckPlan(plan Plan, snapshot PlanSnapshot, opts CheckOptions) CheckReport {
	return newPlanCheckRun(plan, snapshot, opts).run(plan)
}
