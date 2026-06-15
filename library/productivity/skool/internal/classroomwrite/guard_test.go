package classroomwrite

import (
	"testing"
)

func TestValidateCommunityIdentity(t *testing.T) {
	err := ValidateCommunityIdentity(CommunityIdentity{
		Slug:        AllowedCommunitySlug,
		GroupID:     AllowedGroupID,
		DisplayName: AllowedCommunityName,
	})
	if err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
	err = ValidateCommunityIdentity(CommunityIdentity{Slug: "other"})
	if err == nil {
		t.Fatal("expected slug mismatch")
	}
}

func TestAssertImportCourseTitle(t *testing.T) {
	if err := AssertImportCourseTitle("CapCut & Video for Real Estate"); err != nil {
		t.Fatalf("allowed title rejected: %v", err)
	}
	if err := AssertImportCourseTitle("Start Here"); err == nil {
		t.Fatal("protected title should be rejected")
	}
	if err := AssertImportCourseTitle("Creative AI Tools"); err == nil {
		t.Fatal("non-allowlisted title should be rejected")
	}
}

func TestAssertDraftPrivate(t *testing.T) {
	if err := AssertDraftPrivate(1, 1); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
	if err := AssertDraftPrivate(2, 1); err == nil {
		t.Fatal("expected published state rejection")
	}
}

func TestAssertPrivacyUnchanged(t *testing.T) {
	if err := AssertPrivacyUnchanged(1, 1); err != nil {
		t.Fatal(err)
	}
	if err := AssertPrivacyUnchanged(1, 0); err == nil {
		t.Fatal("expected privacy change error")
	}
}

func TestAssertDraftCourse(t *testing.T) {
	if err := AssertDraftCourse(1); err != nil {
		t.Fatal(err)
	}
	if err := AssertDraftCourse(2); err == nil {
		t.Fatal("expected draft-only error")
	}
}
