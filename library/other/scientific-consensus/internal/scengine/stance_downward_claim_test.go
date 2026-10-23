package scengine

import (
	"strings"
	"testing"
)

// The claim "X reduces Y" names neither a risk nor a benefit verb, so it is
// ambiguous and runs the baseline counting path, where "reduced the duration
// of Y" is no cue at all. These cases pin the downward-claim fallback that
// reads such a sentence as support, and the gates that keep it narrow.
// Every sentence is invented for this test.

const downwardTestClaim = "chewing gum reduces postoperative ileus"

const downwardTestTitle = "Chewing gum after surgery"

type downwardClaimCase struct {
	name      string
	claim     string
	title     string
	abstract  string
	wantStnce Stance
	wantConf  float64
}

var downwardClaimCases = []downwardClaimCase{
	{
		name:      "1 reduced duration supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "In this trial, chewing gum reduced the duration of postoperative ileus compared with controls.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		// The baseline null cue "did not reduc" decides; the fallback must not
		// run. Expected value is what the unchanged code returned.
		name:      "2 did not reduce stays refuting",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "In this trial, chewing gum did not reduce the duration of postoperative ileus compared with controls.",
		wantStnce: StanceRefuting,
		wantConf:  0.9,
	},
	{
		name:      "3 hypothesis framing",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "We hypothesized that chewing gum reduces postoperative ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "4 unrelated outcome",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced dental plaque in volunteers.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "5 negated comparative",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was not lower with chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "7 old support path keeps its confidence",
		claim:     "meditation reduces anxiety",
		title:     "Meditation in adults",
		abstract:  "Meditation significantly reduced anxiety scores in adults.",
		wantStnce: StanceSupporting,
		wantConf:  0.9,
	},
	{
		name:      "8 claim without a down verb",
		claim:     "gum improves recovery",
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "9 title-only hypothesis",
		claim:     downwardTestClaim,
		title:     "Chewing gum reduces postoperative ileus: a hypothesis",
		abstract:  "",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "10 cue and outcome in different sentences",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced pain. Ileus was assessed daily.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "11 intervention absent",
		claim:     downwardTestClaim,
		title:     "Feeding after surgery",
		abstract:  "Early feeding reduced postoperative ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// A title is not a reported finding; title-only works stay baseline.
		name:      "12 empty abstract, title-only finding",
		claim:     downwardTestClaim,
		title:     "Chewing gum reduced postoperative ileus",
		abstract:  "",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "13 question sentence",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Does chewing gum reduce postoperative ileus? We randomized 80 patients.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "14 purpose phrase",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "This trial aimed to reduce postoperative ileus with chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "15 intervention in another sentence",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum was given to all patients. The protocol reduced postoperative ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
}

func TestDownwardClaimStance(t *testing.T) {
	for _, c := range downwardClaimCases {
		t.Run(c.name, func(t *testing.T) {
			got, conf := ClassifyStance(c.title, c.abstract, c.claim)
			if got != c.wantStnce || !approxEq(conf, c.wantConf) {
				t.Errorf("ClassifyStance = %s %.2f, want %s %.2f", got, conf, c.wantStnce, c.wantConf)
			}
		})
	}
}

// Case 7 must stay on the counting path, not land on the fallback's value.
func TestDownwardClaimBaselineConfidenceDiffers(t *testing.T) {
	_, conf := ClassifyStance("Meditation in adults",
		"Meditation significantly reduced anxiety scores in adults.", "meditation reduces anxiety")
	if approxEq(conf, downwardClaimConfidence) {
		t.Errorf("baseline support confidence = %.2f, must differ from downwardClaimConfidence", conf)
	}
}

// Case 6: a harm claim never reaches the fallback; ClassifyStance must return
// exactly what the harm branch returns for the same text.
func TestDownwardClaimHarmClaimUntouched(t *testing.T) {
	claim := "chewing gum causes ileus"
	abstract := "In this trial, chewing gum reduced the duration of postoperative ileus compared with controls."
	got, conf := ClassifyStance(downwardTestTitle, abstract, claim)
	hay := strings.ToLower(normalizeText(downwardTestTitle + ". " + abstract))
	want, wantConf := classifyAgainstHarmClaim(hay, claim)
	if got != want || !approxEq(conf, wantConf) {
		t.Errorf("ClassifyStance = %s %.2f, harm branch = %s %.2f", got, conf, want, wantConf)
	}
}
