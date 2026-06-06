package foxnews

import "testing"

func TestSectionsIncludesVideoPath(t *testing.T) {
	var video *Section
	for i := range Sections {
		if Sections[i].ID == "video" {
			video = &Sections[i]
			break
		}
	}
	if video == nil || video.Path != "videos.xml" {
		t.Fatalf("video section: %#v", video)
	}
}
