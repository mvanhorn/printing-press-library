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
	{
		// A negated claim is not a "X reduces Y" claim; the fallback must not
		// read the positive finding as support for it.
		name:      "16 negated claim",
		claim:     "chewing gum does not reduce postoperative ileus",
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced the duration of postoperative ileus in this trial.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "17 control group lower",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was lower in controls than in the chewing gum group.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "18 intervention group lower than controls",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was lower in the chewing gum group than in controls.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		name:      "19 cue on another outcome, claimed outcome unchanged",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced pain but postoperative ileus was unchanged.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "20 cue on another outcome, but not the claimed one",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced pain but not postoperative ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "21 passive form supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was reduced with chewing gum in this trial.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		name:      "22 neither nor",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum neither reduced ileus nor shortened recovery.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// No null cue fires on "did not shorten", so only the local negation
		// skip keeps this from reading as support.
		name:      "23 negated cue without a null cue",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum did not shorten postoperative ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// No "than" comparator, so only controlRightAfterCue applies.
		name:      "24 reduced in the control group",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was reduced in the control group of this chewing gum trial.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// "with usual care" is not "in/among", so only thanIntervention applies.
		name:      "25 shorter than with the intervention",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was shorter with usual care than with chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// "postoperative" is a modifier; the reduced outcome is pain, not ileus.
		name:      "26 cue on another outcome sharing a modifier",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced postoperative pain.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "27 intervention is the comparator before the cue",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Compared with chewing gum, sham feeding reduced postoperative ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "28 reduced only in the control arm",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was reduced only in the control arm, not in the chewing gum arm.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "29 cue on another outcome across a clause break",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum shortened hospital stay and reduced costs, and postoperative ileus resolved in all patients.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "30 unlike the intervention",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Coffee, unlike chewing gum, reduced postoperative ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// The comparator follows the cue, so the intervention is the subject.
		name:      "31 comparator after the cue still supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced postoperative ileus compared with controls.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		name:      "32 head outcome after other words still supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced the duration of postoperative ileus in this trial.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		// The comma follows the outcome, not the cue, so it breaks nothing.
		name:      "33 clause break after the outcome still supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced postoperative ileus, shortening hospital stay.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		name:      "34 reduced in the intervention group still supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was reduced in the chewing gum group.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		// Passive form of case 26: only the head outcome stem may tie.
		name:      "35 passive cue on another outcome sharing a modifier",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative pain was reduced with chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// Passive form of case 29: the comma ends the clause naming ileus.
		name:      "36 passive cue on another outcome across a clause break",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus resolved, and pain was reduced with chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "37 speculative modal before the cue",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum may reduce postoperative ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "38 bare than before the intervention",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Sham feeding reduced postoperative ileus more than chewing gum did.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "39 negation between the cue and the intervention",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was reduced in the subgroup that did not receive chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "40 contrast after the cue in the passive form",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was reduced by sham feeding but not by chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		name:      "41 contrast after the outcome",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced postoperative ileus in previous studies, but not in ours.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// The comparator clause ends at the comma, before the intervention.
		name:      "42 comparator clause before the intervention still supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Compared with controls, chewing gum reduced postoperative ileus.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		name:      "43 bare than before controls still supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced postoperative ileus more than controls did.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		name:      "44 intervention before the cue still supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum, given to all patients, reduced postoperative ileus.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		name:      "45 no contrast after the outcome still supports",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Chewing gum reduced postoperative ileus in this trial.",
		wantStnce: StanceSupporting,
		wantConf:  downwardClaimConfidence,
	},
	{
		// The intervention sits beyond bareThanAfter's reach from "than ",
		// so only thanIntervention applies.
		name:      "46 than in a group given the intervention",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was shorter with sham feeding than in the group that was given chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// The contrast sits before the passive cue, so only the outcome
		// tie's contrast check applies.
		name:      "47 passive cue after a contrast on the claimed outcome",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was unchanged but pain was reduced with chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// "nor" sits before the cue, so only neitherNorCues applies.
		name:      "48 neither nor before the cue",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Neither coffee nor chewing gum reduced postoperative ileus.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// No contrast or negation word, so only controlRightAfterCue applies.
		name:      "49 reduced only in the control arm of the trial",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was reduced only in the control arm of this chewing gum trial.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// "without" is not a contrast word, so only
		// interventionNegationCues applies.
		name:      "50 without the intervention after the cue",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Postoperative ileus was shorter in patients without chewing gum.",
		wantStnce: StanceInconclusive,
		wantConf:  0.2,
	},
	{
		// The first comparator names controls; the second names the
		// intervention.
		name:      "51 second comparator names the intervention",
		claim:     downwardTestClaim,
		title:     downwardTestTitle,
		abstract:  "Compared with controls, and unlike chewing gum, coffee reduced postoperative ileus.",
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
