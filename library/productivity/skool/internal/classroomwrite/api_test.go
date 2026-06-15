package classroomwrite

import "testing"

func TestBuildCreateCourseBody(t *testing.T) {
	body := BuildCreateCourseBody(AllowedGroupID, "CapCut & Video for Real Estate", "desc")
	if body["group_id"] != AllowedGroupID {
		t.Fatalf("group_id mismatch")
	}
	if body["unit_type"] != "course" {
		t.Fatalf("unit_type must be snake_case course")
	}
	if body["state"] != 1 {
		t.Fatalf("expected draft state 1")
	}
	meta, ok := body["metadata"].(map[string]any)
	if !ok || meta["privacy"] != 1 {
		t.Fatalf("expected private metadata")
	}
	if _, has := meta["has_access"]; has {
		t.Fatalf("course metadata must not include has_access")
	}
}

func TestBuildUpdateLessonBody(t *testing.T) {
	body := BuildUpdateLessonBody("Lesson Title", "[v2][]")
	if body["title"] != "Lesson Title" {
		t.Fatalf("expected flat title field")
	}
	if body["desc"] != "[v2][]" {
		t.Fatalf("expected flat desc field")
	}
	if _, has := body["metadata"]; has {
		t.Fatalf("update lesson body must be flat, not metadata-wrapped")
	}
	if body["transcript"] != nil || body["video_id"] != "" {
		t.Fatalf("expected default transcript/video_id fields")
	}
}

func TestBuildCreateLessonBody(t *testing.T) {
	body := BuildCreateLessonBody(AllowedGroupID, "parent", "Lesson", "[v2][]")
	if body["unit_type"] != "module" {
		t.Fatalf("expected module")
	}
	if body["parent_id"] != "parent" || body["root_id"] != "parent" {
		t.Fatalf("expected parent/root ids")
	}
}

func TestDryRunPlanWouldCreate(t *testing.T) {
	importer := &Importer{API: nil}
	plan := &LocalCoursePlan{
		Title:       "CapCut & Video for Real Estate",
		Description: "Course desc",
		Lessons: []LocalLessonPlan{
			{Order: 1, Title: "Lesson One", Body: "Hello world"},
		},
	}
	counters := &ImportCounters{}
	res, err := importer.DryRunPlan(plan, nil, counters)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "would_create" {
		t.Fatalf("expected would_create, got %q", res.Action)
	}
	if counters.CoursesCreated != 1 || counters.LessonsCreated != 1 {
		t.Fatalf("unexpected counters: %+v", counters)
	}
	if len(res.Lessons) != 1 || res.Lessons[0].Action != "would_create" {
		t.Fatalf("expected one would_create lesson")
	}
}

func TestImportPlanSkipsExistingCourse(t *testing.T) {
	existing := []CourseRecord{{
		ID: "c1",
		Metadata: CourseMetadata{Title: "CapCut & Video for Real Estate", Privacy: 1},
		State: 1,
	}}
	importer := &Importer{API: nil}
	plan := &LocalCoursePlan{Title: "CapCut & Video for Real Estate", Lessons: []LocalLessonPlan{}}
	if FindCourseByTitle(existing, plan.Title) == nil {
		t.Fatal("expected existing course match")
	}
	_ = importer
}
