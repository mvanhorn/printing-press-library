// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored Withings decode helpers (no "DO NOT EDIT" header). These map
// the integer codes the Withings API returns — measurement type codes, AFib
// classifications, sleep-stage states, workout categories, notification
// "appli" subscription kinds — onto human-readable labels, and scale raw
// measure values by their power-of-ten unit exponent. The analytics commands
// and any future display code share these so a code/label divergence can't
// creep in across call sites.

package cli

import "math"

// withingsMeasureType describes one row of the Withings measurement-type table.
// real value = value * 10^unit (see scaleMeasure); `unit` here is the SI unit
// the scaled value is expressed in, not the power-of-ten exponent.
type withingsMeasureType struct {
	name  string
	unit  string // human unit label, e.g. "kg", "%", "mmHg"
	label string // longer human description
}

// withingsMeasureTypes maps Withings measure `type` codes to metadata. Codes
// are from the Withings Measure API getmeas spec (measures[].type). Unlisted
// codes resolve to an "unknown_<code>" name via withingsMeasureTypeName.
var withingsMeasureTypes = map[int]withingsMeasureType{
	1:   {"weight", "kg", "Weight"},
	4:   {"height", "m", "Height"},
	5:   {"fat_free_mass", "kg", "Fat-free (lean) mass"},
	6:   {"fat_ratio", "%", "Fat ratio"},
	8:   {"fat_mass", "kg", "Fat mass weight"},
	9:   {"diastolic_bp", "mmHg", "Diastolic blood pressure"},
	10:  {"systolic_bp", "mmHg", "Systolic blood pressure"},
	11:  {"heart_pulse", "bpm", "Heart pulse"},
	12:  {"temperature", "C", "Temperature"},
	54:  {"spo2", "%", "SpO2"},
	71:  {"body_temp", "C", "Body temperature"},
	73:  {"skin_temp", "C", "Skin temperature"},
	76:  {"muscle_mass", "kg", "Muscle mass"},
	77:  {"hydration", "kg", "Hydration"},
	88:  {"bone_mass", "kg", "Bone mass"},
	91:  {"pulse_wave_velocity", "m/s", "Pulse wave velocity"},
	123: {"vo2max", "mL/kg/min", "VO2 max"},
	130: {"afib_ppg", "", "Atrial fibrillation result (PPG)"},
	135: {"qrs_ms", "ms", "QRS interval duration"},
	136: {"pr_ms", "ms", "PR interval duration"},
	137: {"qt_ms", "ms", "QT interval duration"},
	138: {"qtc_ms", "ms", "Corrected QT interval duration"},
	139: {"afib_ecg", "", "Atrial fibrillation result (ECG)"},
}

// withingsMeasureTypeName returns the short snake_case name for a measure type
// code, or "unknown_<code>" when the code is not in the table.
func withingsMeasureTypeName(code int) string {
	if t, ok := withingsMeasureTypes[code]; ok {
		return t.name
	}
	return "unknown_" + itoa(code)
}

// withingsMeasureTypeUnit returns the human unit label (e.g. "kg", "%") for a
// measure type code, or "" when unknown or unitless.
func withingsMeasureTypeUnit(code int) string {
	if t, ok := withingsMeasureTypes[code]; ok {
		return t.unit
	}
	return ""
}

// withingsMeasureTypeLabel returns the long human description for a measure
// type code, or "Unknown measure type <code>" when unknown.
func withingsMeasureTypeLabel(code int) string {
	if t, ok := withingsMeasureTypes[code]; ok {
		return t.label
	}
	return "Unknown measure type " + itoa(code)
}

// scaleMeasure converts a raw Withings (value, unit) pair to its real value:
// real = value * 10^unit. unit is the power-of-ten exponent and is frequently
// negative (e.g. value=81250 unit=-3 => 81.25).
func scaleMeasure(value, unit int) float64 {
	return float64(value) * math.Pow10(unit)
}

// withingsAfibLabel maps a Withings AFib classification integer to a label.
// 0 = negative, 1 = positive (AFib detected), 2 = inconclusive. Used for both
// ECG ecg.afib and the type-130/139 measure results.
func withingsAfibLabel(code int) string {
	switch code {
	case 0:
		return "negative"
	case 1:
		return "afib"
	case 2:
		return "inconclusive"
	default:
		return "unknown"
	}
}

// withingsSleepStateLabel maps a sleep-stage state integer (from the /v2/sleep
// get series) to a label: 0 awake, 1 light, 2 deep, 3 rem.
func withingsSleepStateLabel(state int) string {
	switch state {
	case 0:
		return "awake"
	case 1:
		return "light"
	case 2:
		return "deep"
	case 3:
		return "rem"
	default:
		return "unknown"
	}
}

// withingsWorkoutCategories maps Withings workout category codes to labels.
// Codes are from the Withings getworkouts spec (series[].category). The set
// here covers the common activities; unlisted codes resolve to "Other".
var withingsWorkoutCategories = map[int]string{
	1:   "Walk",
	2:   "Run",
	3:   "Hiking",
	4:   "Skating",
	5:   "BMX",
	6:   "Bicycling",
	7:   "Swimming",
	8:   "Surfing",
	9:   "Kitesurfing",
	10:  "Windsurfing",
	11:  "Bodyboard",
	12:  "Tennis",
	13:  "Table tennis",
	14:  "Squash",
	15:  "Badminton",
	16:  "Lift weights",
	17:  "Calisthenics",
	18:  "Elliptical",
	19:  "Pilates",
	20:  "Basketball",
	21:  "Soccer",
	22:  "Football",
	23:  "Rugby",
	24:  "Volleyball",
	25:  "Waterpolo",
	26:  "Horse riding",
	27:  "Golf",
	28:  "Yoga",
	29:  "Dancing",
	30:  "Boxing",
	31:  "Fencing",
	32:  "Wrestling",
	33:  "Martial arts",
	34:  "Skiing",
	35:  "Snowboarding",
	36:  "Other",
	128: "No activity",
	187: "Rowing",
	188: "Zumba",
	191: "Baseball",
	192: "Handball",
	193: "Hockey",
	194: "Ice hockey",
	195: "Climbing",
	196: "Ice skating",
	272: "Multi-sport",
	306: "Hiit",
	307: "Indoor walk",
	308: "Indoor running",
	309: "Indoor cycling",
}

// withingsWorkoutCategoryName returns the label for a workout category code, or
// "Other" when the code is not in the table.
func withingsWorkoutCategoryName(code int) string {
	if name, ok := withingsWorkoutCategories[code]; ok {
		return name
	}
	return "Other"
}

// withingsApplis maps Withings notification "appli" subscription kinds to
// labels (used by the /notify endpoints). Codes from the Withings Notify spec.
var withingsApplis = map[int]string{
	1:  "Weight",
	2:  "Temperature",
	4:  "Blood pressure / Heart rate",
	16: "Activity",
	44: "Sleep",
	46: "User actions",
	50: "Bed in/out",
	51: "Inflate done",
	54: "ECG",
	55: "ECG failed",
	58: "Glucose",
}

// withingsAppliName returns the label for a notification appli code, or
// "Unknown appli <code>" when the code is not in the table.
func withingsAppliName(code int) string {
	if name, ok := withingsApplis[code]; ok {
		return name
	}
	return "Unknown appli " + itoa(code)
}

// itoa is a tiny strconv.Itoa shim kept local so this file's helpers don't
// pull strconv into callers that only want labels; mirrors the lean-import
// style of the surrounding hand-authored files.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
