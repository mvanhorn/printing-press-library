package classroomwrite

import "fmt"

// CoursePrivacyLevelUnlock is Skool metadata.privacy=1 (level unlock), used as
// the private draft gate for Courses 03 and 04 in this community.
const CoursePrivacyLevelUnlock = 1

type CourseUpdateResult struct {
	Course          CourseRecord `json:"course"`
	UpdatedAtBefore string       `json:"updated_at_before"`
	UpdatedAtAfter  string       `json:"updated_at_after"`
	PrivacyBefore   int          `json:"privacy_before"`
	PrivacyAfter    int          `json:"privacy_after"`
	TitleUnchanged  bool         `json:"title_unchanged"`
	LessonCount     int          `json:"lesson_count"`
	LessonIDs       []string     `json:"lesson_ids"`
	IdempotentNoOp  bool         `json:"idempotent_no_op,omitempty"`
}

// BuildUpdateCourseBody returns the flat api2 PUT body for top-level course
// metadata updates. Skool silently no-ops metadata-wrapped partial bodies.
func BuildUpdateCourseBody(title, desc string, privacy int) map[string]any {
	body := map[string]any{
		"title":   title,
		"desc":    desc,
		"privacy": privacy,
	}
	return body
}

func (a *API) UpdateCoursePrivacy(courseID string, privacy int) (*CourseUpdateResult, error) {
	if courseID == "" {
		return nil, fmt.Errorf("course id is required")
	}
	tree, err := a.GetCourseTree(courseID)
	if err != nil {
		return nil, err
	}
	course := tree.Course
	if err := AssertImportCourseTitle(course.Metadata.Title); err != nil {
		return nil, err
	}
	if err := AssertDraftCourse(course.State); err != nil {
		return nil, err
	}
	lessonIDs := lessonIDsFromTree(tree)
	beforePrivacy := course.Metadata.Privacy
	if beforePrivacy == privacy {
		return &CourseUpdateResult{
			Course:          course,
			UpdatedAtBefore: course.UpdatedAt,
			UpdatedAtAfter:  course.UpdatedAt,
			PrivacyBefore:   beforePrivacy,
			PrivacyAfter:    beforePrivacy,
			TitleUnchanged:  true,
			LessonCount:     len(lessonIDs),
			LessonIDs:       lessonIDs,
			IdempotentNoOp:  true,
		}, nil
	}
	body := BuildUpdateCourseBody(course.Metadata.Title, course.Metadata.Desc, privacy)
	path := fmt.Sprintf("%s/courses/%s", api2Base, courseID)
	raw, status, err := a.Client.PutWithHeaders(path, body, chromeHeaders())
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("update course privacy failed: HTTP %d: %s", status, truncate(raw))
	}
	afterTree, err := a.GetCourseTree(courseID)
	if err != nil {
		return nil, err
	}
	after := afterTree.Course
	result := &CourseUpdateResult{
		Course:          after,
		UpdatedAtBefore: course.UpdatedAt,
		UpdatedAtAfter:  after.UpdatedAt,
		PrivacyBefore:   beforePrivacy,
		PrivacyAfter:    after.Metadata.Privacy,
		TitleUnchanged:  after.Metadata.Title == course.Metadata.Title,
		LessonCount:     len(lessonIDsFromTree(afterTree)),
		LessonIDs:       lessonIDsFromTree(afterTree),
	}
	if after.Metadata.Privacy != privacy {
		return nil, fmt.Errorf("course privacy update did not persist: got privacy=%d want %d", after.Metadata.Privacy, privacy)
	}
	if err := AssertDraftCourse(after.State); err != nil {
		return nil, err
	}
	if !result.TitleUnchanged {
		return nil, fmt.Errorf("course title changed unexpectedly: got %q want %q", after.Metadata.Title, course.Metadata.Title)
	}
	if result.LessonCount != len(lessonIDs) {
		return nil, fmt.Errorf("lesson count changed: before=%d after=%d", len(lessonIDs), result.LessonCount)
	}
	for i, id := range lessonIDs {
		if i >= len(result.LessonIDs) || result.LessonIDs[i] != id {
			return nil, fmt.Errorf("lesson IDs changed after course privacy update")
		}
	}
	return result, nil
}

func lessonIDsFromTree(tree *CourseTree) []string {
	out := make([]string, 0, len(tree.Children))
	for _, child := range tree.Children {
		out = append(out, child.Course.ID)
	}
	return out
}

func (a *API) SnapshotCoursesByTitle(groupID string, titles []string) (map[string]CourseRecord, error) {
	courses, err := a.ListCourses(groupID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]CourseRecord, len(titles))
	for _, title := range titles {
		rec := FindCourseByTitle(courses, title)
		if rec == nil {
			return nil, fmt.Errorf("course %q not found remotely", title)
		}
		out[title] = *rec
	}
	return out, nil
}

func AssertCoursesUnchanged(before, after map[string]CourseRecord) error {
	for title, prev := range before {
		cur, ok := after[title]
		if !ok {
			return fmt.Errorf("course %q missing after operation", title)
		}
		if cur.State != prev.State {
			return fmt.Errorf("course %q state changed from %d to %d", title, prev.State, cur.State)
		}
		if cur.Metadata.Privacy != prev.Metadata.Privacy {
			return fmt.Errorf("course %q privacy changed from %d to %d", title, prev.Metadata.Privacy, cur.Metadata.Privacy)
		}
		if cur.Metadata.Title != prev.Metadata.Title {
			return fmt.Errorf("course %q title changed", title)
		}
	}
	return nil
}
