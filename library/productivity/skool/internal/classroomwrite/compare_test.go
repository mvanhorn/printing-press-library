package classroomwrite

import "testing"

func TestDescEquivalent(t *testing.T) {
	a := `[v2][{"type":"paragraph","content":[{"type":"text","text":"Hello","marks":[{"type":"bold"}]}]}]`
	b := `[v2][{"type":"paragraph","content":[{"type":"text","marks":[{"type":"bold"}],"text":"Hello"}]}]`
	if !DescEquivalent(a, b) {
		t.Fatal("expected semantic equivalence")
	}
	if DescEquivalent(a, `[v2][{"type":"paragraph","content":[{"type":"text","text":"Other"}]}]`) {
		t.Fatal("expected mismatch")
	}
}

func TestDescEquivalentRequiresV2Prefix(t *testing.T) {
	if DescEquivalent("plain text", `[v2][]`) {
		t.Fatal("expected false without [v2] prefix")
	}
}
