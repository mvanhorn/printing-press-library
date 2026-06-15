package classroomwrite

import "testing"

func TestBuildUpdateCourseBodyFlat(t *testing.T) {
	body := BuildUpdateCourseBody("CapCut & Video for Real Estate", "desc", CoursePrivacyLevelUnlock)
	if _, has := body["metadata"]; has {
		t.Fatal("course update must use flat body, not metadata wrapper")
	}
	if body["privacy"] != CoursePrivacyLevelUnlock {
		t.Fatalf("expected privacy=1, got %v", body["privacy"])
	}
	if body["title"] != "CapCut & Video for Real Estate" {
		t.Fatal("title must be preserved in flat body")
	}
}

func TestBuildUpdateLessonBodyNotMetadataWrapped(t *testing.T) {
	body := BuildUpdateLessonBody("Lesson", "[v2][]")
	if _, has := body["metadata"]; has {
		t.Fatal("lesson update must use flat body, not metadata wrapper")
	}
	if body["desc"] != "[v2][]" {
		t.Fatal("expected flat desc field")
	}
}

func TestHasLiteralFormattingArtifacts(t *testing.T) {
	if !HasLiteralFormattingArtifacts(`[v2][{"text":"**bold**"}]`) {
		t.Fatal("expected literal ** detection")
	}
	if HasLiteralFormattingArtifacts(`[v2][{"marks":[{"type":"bold"}]}]`) {
		t.Fatal("did not expect artifact detection for bold marks")
	}
}
