package classroomwrite

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/skool/internal/client"
)

const api2Base = "https://api2.skool.com"

type API struct {
	Client *client.Client
}

func NewAPI(c *client.Client) *API {
	return &API{Client: c}
}

func chromeHeaders() map[string]string {
	return map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36",
		"Origin":     "https://www.skool.com",
		"Referer":    "https://www.skool.com/",
	}
}

func (a *API) ListCourses(groupID string) ([]CourseRecord, error) {
	path := fmt.Sprintf("%s/groups/%s/courses", api2Base, groupID)
	raw, err := a.Client.Get(path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Courses []CourseRecord `json:"courses"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing courses list: %w", err)
	}
	return resp.Courses, nil
}

func (a *API) GetCourseTree(courseID string) (*CourseTree, error) {
	path := fmt.Sprintf("%s/courses/%s", api2Base, courseID)
	raw, err := a.Client.Get(path, nil)
	if err != nil {
		return nil, err
	}
	var tree CourseTree
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("parsing course tree: %w", err)
	}
	return &tree, nil
}

// GetLesson fetches a single lesson (module) under a parent course.
// Skool returns HTTP 400 for GET /courses/{lessonID}; lessons require parent course + module_id.
func (a *API) GetLesson(parentCourseID, lessonID string) (*CourseRecord, error) {
	path := fmt.Sprintf("%s/courses/%s", api2Base, parentCourseID)
	raw, err := a.Client.Get(path, map[string]string{"module_id": lessonID})
	if err != nil {
		return nil, err
	}
	var tree CourseTree
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("parsing lesson tree: %w", err)
	}
	for _, child := range tree.Children {
		if child.Course.ID == lessonID {
			rec := child.Course
			return &rec, nil
		}
	}
	return nil, fmt.Errorf("lesson %q not found under course %q", lessonID, parentCourseID)
}

type LessonUpdateResult struct {
	Lesson           CourseRecord `json:"lesson"`
	UpdatedAtBefore  string       `json:"updated_at_before"`
	UpdatedAtAfter   string       `json:"updated_at_after"`
	DescMatched      bool         `json:"desc_matched"`
	TitleUnchanged   bool         `json:"title_unchanged"`
	ParentCourseID   string       `json:"parent_course_id"`
	ParentPrivacy    int          `json:"parent_privacy"`
	IdempotentNoOp   bool         `json:"idempotent_no_op,omitempty"`
}

func (a *API) UpdateLessonBody(parentCourseID, lessonID, desc string) (*LessonUpdateResult, error) {
	return a.UpdateLesson(parentCourseID, lessonID, UpdateLessonOptions{Desc: desc})
}

type UpdateLessonOptions struct {
	Title string
	Desc  string
}

func (a *API) UpdateLesson(parentCourseID, lessonID string, opts UpdateLessonOptions) (*LessonUpdateResult, error) {
	if parentCourseID == "" || lessonID == "" {
		return nil, fmt.Errorf("parent course id and lesson id are required")
	}
	parent, err := a.GetCourseTree(parentCourseID)
	if err != nil {
		return nil, err
	}
	parentPrivacy := parent.Course.Metadata.Privacy
	if err := AssertDraftPrivate(parent.Course.State, parent.Course.Metadata.Privacy); err != nil {
		return nil, err
	}
	current, err := a.GetLesson(parentCourseID, lessonID)
	if err != nil {
		return nil, err
	}
	if current.ParentID != parentCourseID {
		return nil, fmt.Errorf("lesson %q parent_id %q does not match course-id %q", lessonID, current.ParentID, parentCourseID)
	}
	if err := AssertLessonDraft(current.State); err != nil {
		return nil, err
	}
	title := current.Metadata.Title
	if opts.Title != "" {
		title = opts.Title
	}
	if opts.Desc == "" {
		return nil, fmt.Errorf("lesson desc is required")
	}
	if DescEquivalent(current.Metadata.Desc, opts.Desc) {
		result := &LessonUpdateResult{
			Lesson:          *current,
			UpdatedAtBefore: current.UpdatedAt,
			UpdatedAtAfter:  current.UpdatedAt,
			DescMatched:     true,
			TitleUnchanged:  true,
			ParentCourseID:  parentCourseID,
			ParentPrivacy:   parentPrivacy,
			IdempotentNoOp:  true,
		}
		return result, nil
	}
	body := BuildUpdateLessonBody(title, opts.Desc)
	path := fmt.Sprintf("%s/courses/%s", api2Base, lessonID)
	raw, status, err := a.Client.PutWithHeaders(path, body, chromeHeaders())
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("update lesson failed: HTTP %d: %s", status, truncate(raw))
	}
	after, err := a.GetLesson(parentCourseID, lessonID)
	if err != nil {
		return nil, err
	}
	result := &LessonUpdateResult{
		Lesson:          *after,
		UpdatedAtBefore: current.UpdatedAt,
		UpdatedAtAfter:  after.UpdatedAt,
		DescMatched:     DescEquivalent(after.Metadata.Desc, opts.Desc),
		TitleUnchanged:  after.Metadata.Title == current.Metadata.Title,
		ParentCourseID:  parentCourseID,
		ParentPrivacy:   parentPrivacy,
	}
	if after.Metadata.Desc == current.Metadata.Desc {
		return nil, fmt.Errorf("lesson body update did not persist (stored desc unchanged; updated_at before=%s after=%s)", current.UpdatedAt, after.UpdatedAt)
	}
	if after.UpdatedAt == current.UpdatedAt {
		return nil, fmt.Errorf("lesson body update did not persist (updated_at unchanged: %s)", current.UpdatedAt)
	}
	if !result.DescMatched {
		return nil, fmt.Errorf("lesson body update persisted but remote desc does not match expected normalized content")
	}
	if opts.Title == "" && !result.TitleUnchanged {
		return nil, fmt.Errorf("lesson title changed unexpectedly: got %q want %q", after.Metadata.Title, current.Metadata.Title)
	}
	if err := AssertLessonDraft(after.State); err != nil {
		return nil, err
	}
	parentAfter, err := a.GetCourseTree(parentCourseID)
	if err != nil {
		return nil, err
	}
	if err := AssertDraftPrivate(parentAfter.Course.State, parentAfter.Course.Metadata.Privacy); err != nil {
		return nil, err
	}
	if err := AssertPrivacyUnchanged(parentPrivacy, parentAfter.Course.Metadata.Privacy); err != nil {
		return nil, err
	}
	return result, nil
}

func BuildUpdateLessonBody(title, desc string) map[string]any {
	return map[string]any{
		"transcript": nil,
		"video_id":   "",
		"title":      title,
		"desc":       desc,
	}
}

func (a *API) CreateCourse(groupID, title, desc string) (*CourseRecord, error) {
	body := BuildCreateCourseBody(groupID, title, desc)
	raw, status, err := a.Client.PostWithHeaders(api2Base+"/courses", body, chromeHeaders())
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("create course failed: HTTP %d: %s", status, truncate(raw))
	}
	rec, err := decodeCourseResponse(raw)
	if err != nil {
		return nil, err
	}
	if err := AssertDraftPrivate(rec.State, rec.Metadata.Privacy); err != nil {
		return nil, err
	}
	return rec, nil
}

func (a *API) CreateLesson(groupID, courseID, title, desc string) (*CourseRecord, error) {
	body := BuildCreateLessonBody(groupID, courseID, title, desc)
	raw, status, err := a.Client.PostWithHeaders(api2Base+"/courses", body, chromeHeaders())
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("create lesson failed: HTTP %d: %s", status, truncate(raw))
	}
	rec, err := decodeCourseResponse(raw)
	if err != nil {
		return nil, err
	}
	if err := AssertLessonDraft(rec.State); err != nil {
		return nil, err
	}
	return rec, nil
}

func (a *API) UpdateCourseMetadata(courseID string, metadata map[string]any, state int) (*CourseRecord, error) {
	// Deprecated path: metadata-wrapped PUT silently no-ops for course fields.
	// Use UpdateCoursePrivacy (flat body) instead.
	title, _ := metadata["title"].(string)
	desc, _ := metadata["desc"].(string)
	privacy, _ := metadata["privacy"].(int)
	body := BuildUpdateCourseBody(title, desc, privacy)
	if state > 0 {
		body["state"] = state
	}
	path := fmt.Sprintf("%s/courses/%s", api2Base, courseID)
	raw, status, err := a.Client.PutWithHeaders(path, body, chromeHeaders())
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("update course failed: HTTP %d: %s", status, truncate(raw))
	}
	rec, err := decodeCourseResponse(raw)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func BuildCreateCourseBody(groupID, title, desc string) map[string]any {
	return map[string]any{
		"group_id":  groupID,
		"unit_type": "course",
		"state":     1,
		"metadata": map[string]any{
			"title":       title,
			"desc":        desc,
			"privacy":     1,
			"cover_image": "",
		},
	}
}

func BuildCreateLessonBody(groupID, courseID, title, desc string) map[string]any {
	return map[string]any{
		"group_id":  groupID,
		"unit_type": "module",
		"parent_id": courseID,
		"root_id":   courseID,
		"state":     1,
		"metadata": map[string]any{
			"title":     title,
			"desc":      desc,
			"resources": "[]",
		},
	}
}

func decodeCourseResponse(raw json.RawMessage) (*CourseRecord, error) {
	var wrap struct {
		Course CourseRecord `json:"course"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && wrap.Course.ID != "" {
		return &wrap.Course, nil
	}
	var direct CourseRecord
	if err := json.Unmarshal(raw, &direct); err != nil {
		return nil, fmt.Errorf("parsing course response: %w", err)
	}
	if direct.ID == "" {
		return nil, fmt.Errorf("course response missing id")
	}
	return &direct, nil
}

func flattenChildren(tree *CourseTree) []CourseRecord {
	out := make([]CourseRecord, 0, len(tree.Children))
	for _, child := range tree.Children {
		out = append(out, child.Course)
	}
	return out
}

func FindCourseByTitle(courses []CourseRecord, title string) *CourseRecord {
	want := NormalizeTitle(title)
	for i := range courses {
		if NormalizeTitle(courses[i].Metadata.Title) == want {
			return &courses[i]
		}
	}
	return nil
}

func FindLessonByTitle(lessons []CourseRecord, title string) *CourseRecord {
	want := NormalizeTitle(title)
	for i := range lessons {
		if NormalizeTitle(lessons[i].Metadata.Title) == want {
			return &lessons[i]
		}
	}
	return nil
}

func truncate(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
