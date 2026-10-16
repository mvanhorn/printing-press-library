package scengine

// Characterization of a measured gap in the heuristic ClassifyStance path.
//
// Every fixture below is a hand-written, gold-refuting abstract: the trial
// found no benefit or found harm for the intervention the claim promotes.
// Measured on the current code, only F3 and F8 classify refuting. The others
// fail because the claim's own benefit words appear in the abstract and the
// negation is phrased in ways the null cues do not cover: "did not lower",
// "did not prevent", "did not result in a lower", and "increased rather than
// reduced". Those phrases count as support, so F2, F4, F5 and F7 come out
// supporting with confidence >= 0.85. The "did not reduce" form is already
// handled by the redu branch of nullCues.
//
// An external benchmark (SciFact, CC BY-NC 2.0) showed the same pattern. None
// of its text or identifiers are included here.
//
// Part A guards the fixtures that already classify refuting. Part B pins the
// current wrong stance of every other fixture so that any change, including a
// fix, fails loudly and the table gets updated on purpose.

import "testing"

type refutingGapFixture struct{ id, claim, title, abstract string }

var refutingGapFixtures = []refutingGapFixture{
	{"F1", "Vitamin D supplementation reduces the risk of fractures in older adults.", "Vitamin D supplementation and fracture risk in community-dwelling older adults: a randomized controlled trial", "We randomly assigned 4,200 adults aged 70 years or older to daily vitamin D3 or placebo for five years. The primary outcome was any non-vertebral fracture. Fractures occurred in 11.2% of the vitamin D group and 10.9% of the placebo group (hazard ratio 1.03, 95% CI 0.88 to 1.21). Vitamin D supplementation did not reduce the risk of fractures compared with placebo."},
	{"F2", "Daily aspirin lowers the risk of a first heart attack in healthy elderly people.", "Low-dose aspirin for primary prevention in healthy elderly persons", "In this trial, 19,114 healthy persons aged 70 or older received 100 mg aspirin or placebo. After a median follow-up of 4.7 years, the rate of cardiovascular events was similar in the two groups (hazard ratio 0.95, 95% CI 0.83 to 1.08). Aspirin did not lower the risk of myocardial infarction, and major bleeding was significantly higher in the aspirin group."},
	{"F3", "Antibiotics shorten the duration of acute bronchitis symptoms.", "Amoxicillin versus placebo for acute lower respiratory tract infection in primary care", "Adults with acute cough were randomized to amoxicillin or placebo. Duration of moderately bad or worse symptoms was not significantly different between groups (median 6 days versus 7 days, hazard ratio 1.06, 95% CI 0.96 to 1.18). Antibiotics provided no benefit in symptom duration and caused more side effects."},
	{"F4", "Omega-3 fatty acid supplements prevent cardiovascular events.", "Marine n-3 fatty acids and prevention of cardiovascular disease", "A total of 25,871 participants were randomized to marine n-3 fatty acids at 1 g per day or placebo. During a median follow-up of 5.3 years, a major cardiovascular event occurred in 386 participants in the n-3 group and 419 in the placebo group (hazard ratio 0.92, 95% CI 0.80 to 1.06; P=0.24). Supplementation with n-3 fatty acids did not result in a lower incidence of major cardiovascular events than placebo."},
	{"F5", "Hormone replacement therapy reduces coronary heart disease in postmenopausal women.", "Risks and benefits of estrogen plus progestin in healthy postmenopausal women", "16,608 postmenopausal women were randomized to combined estrogen plus progestin or placebo. The trial was stopped early because coronary heart disease events increased in the hormone group (hazard ratio 1.29, 95% CI 1.02 to 1.63). Hormone therapy increased rather than reduced the risk of coronary heart disease."},
	{"F6", "Arthroscopic knee surgery relieves pain in osteoarthritis of the knee.", "A controlled trial of arthroscopic surgery for osteoarthritis of the knee", "Patients with knee osteoarthritis were randomized to arthroscopic debridement, arthroscopic lavage, or placebo surgery. At no point during two years of follow-up did either intervention group report less pain or better function than the placebo group. The outcomes after arthroscopic surgery were no better than those after a placebo procedure."},
	{"F7", "Beta-carotene supplementation lowers the incidence of lung cancer in smokers.", "Effect of beta carotene on lung cancer incidence in male smokers", "29,133 male smokers were randomly assigned to beta carotene, alpha-tocopherol, both, or placebo. Lung cancer incidence was 18% higher among men who received beta carotene than among those who did not (95% CI 3% to 36%). Beta carotene supplementation did not prevent lung cancer and may have increased its incidence."},
	{"F8", "Tight glucose control in intensive care reduces mortality.", "Intensive versus conventional glucose control in critically ill patients", "6,104 adults in intensive care units were randomized to intensive glucose control (target 4.5 to 6.0 mmol per liter) or conventional control. Ninety-day mortality was 27.5% in the intensive group and 24.9% in the conventional group (odds ratio 1.14, 95% CI 1.02 to 1.28). Intensive glucose control increased mortality among adults in the ICU."},
}

// refutingGapKnown lists the fixtures NOT classified refuting today, with
// their measured stance.
var refutingGapKnown = map[string]Stance{
	"F1": StanceMixed,
	"F2": StanceSupporting,
	"F4": StanceSupporting,
	"F5": StanceSupporting,
	"F6": StanceInconclusive,
	"F7": StanceSupporting,
}

// refutingGapHighConfSupporting is the measured number of gold-refuting
// fixtures classified supporting with confidence >= 0.85.
const refutingGapHighConfSupporting = 4

func TestRefutingGapRegressionGuard(t *testing.T) {
	for _, f := range refutingGapFixtures {
		if _, known := refutingGapKnown[f.id]; known {
			continue
		}
		if got, conf := ClassifyStance(f.title, f.abstract, f.claim); got != StanceRefuting {
			t.Errorf("fixture %s now classified %s (conf %.2f), want refuting", f.id, got, conf)
		}
	}
}

func TestRefutingGapKnownGap(t *testing.T) {
	highConfSupporting := 0
	for _, f := range refutingGapFixtures {
		got, conf := ClassifyStance(f.title, f.abstract, f.claim)
		if got == StanceSupporting && conf >= 0.85 {
			highConfSupporting++
		}
		want, known := refutingGapKnown[f.id]
		if !known {
			continue
		}
		if got != want {
			t.Errorf("fixture %s now classified %s (was %s) — if it is now refuting, the gap is fixed: move it to part A", f.id, got, want)
		}
	}
	if highConfSupporting != refutingGapHighConfSupporting {
		t.Errorf("gold-refuting fixtures classified supporting with conf >= 0.85: got %d, want %d", highConfSupporting, refutingGapHighConfSupporting)
	}
}
