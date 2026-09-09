package ocr

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestCommandEngineFromEnvironmentRequiresExplicitCommand(t *testing.T) {
	t.Setenv("KVMCTL_OCR_COMMAND", "")
	_, err := CommandEngineFromEnvironment()
	if !errors.Is(err, ErrOCRUnavailable) {
		t.Fatalf("error = %v, want OCR unavailable", err)
	}
}

func TestCommandEngineRecognizesTesseractTSVFromStandardInput(t *testing.T) {
	tesseract, err := exec.LookPath("tesseract")
	if err != nil {
		t.Skip("tesseract is not installed")
	}
	t.Setenv("KVMCTL_OCR_COMMAND", tesseract)
	t.Setenv("KVMCTL_OCR_PROTOCOL", "")
	engine, err := CommandEngineFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	image := makePNG(t, 31, 17)
	width, height, words, err := engine.Recognize(image)
	if err != nil {
		t.Fatal(err)
	}
	if width != 31 || height != 17 {
		t.Fatalf("dimensions = %dx%d, want 31x17", width, height)
	}
	if words == nil {
		t.Fatal("words must be a non-nil empty slice when no text is recognized")
	}
}

func TestParseJSONResponseRejectsMissingWordConfidence(t *testing.T) {
	_, _, _, err := parseJSONResponse([]byte(`{"width":10,"height":10,"words":[{"text":"ok","x":1,"y":2,"width":3,"height":4}]}`))
	if !errors.Is(err, ErrOCRFailed) {
		t.Fatalf("error = %v, want OCR failure", err)
	}
}

func TestParseTesseractTSVIncludesLinePhrases(t *testing.T) {
	tsv := strings.Join([]string{
		"level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext",
		"1\t1\t0\t0\t0\t0\t0\t0\t100\t50\t-1\t",
		"5\t1\t1\t1\t1\t1\t10\t10\t20\t10\t95.0\tSave",
		"5\t1\t1\t1\t1\t2\t32\t10\t30\t10\t96.0\tChanges",
	}, "\n") + "\n"
	width, height, words, err := parseTesseractTSV([]byte(tsv))
	if err != nil {
		t.Fatal(err)
	}
	if width != 100 || height != 50 {
		t.Fatalf("dimensions = %dx%d", width, height)
	}
	found := map[string]Word{}
	for _, word := range words {
		found[word.Text] = word
	}
	if _, ok := found["Save"]; !ok {
		t.Fatalf("missing word Save: %#v", words)
	}
	if _, ok := found["Changes"]; !ok {
		t.Fatalf("missing word Changes: %#v", words)
	}
	phrase, ok := found["Save Changes"]
	if !ok {
		t.Fatalf("missing line phrase Save Changes: %#v", words)
	}
	if phrase.X != 10 || phrase.Width != 52 || phrase.Confidence != 95.0 {
		t.Fatalf("phrase bounds/confidence = %#v", phrase)
	}
}
