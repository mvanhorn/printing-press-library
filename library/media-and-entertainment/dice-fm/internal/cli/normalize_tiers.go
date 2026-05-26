package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// tierAxes holds the structured axis values extracted from a canonicalized
// ticket-type name. All fields are zero-value when not detected.
type tierAxes struct {
	AccessClass     string // "ga" | "vip" | "premium" | ""
	SalesStage      string // "super_early_bird" | "early_bird" | "final_release" | "last_chance" | "tier_n" | ""
	EntryWindowType string // "deadline" | "anytime" | "door" | ""
	EntryWindowTime string // "HH:MM" 24h when type=deadline, else ""
	GroupSize       int    // party size (You+N -> N+1); 0 = unknown
	CompFlag        bool
	Matched         bool // true if any axis was confidently assigned
}

// Compiled package-level regexps for tier axis extraction.
// All operate on already-canonicalized lowercase input.
var (
	reAccessVIP      = regexp.MustCompile(`\bvip\b`)
	reAccessGA       = regexp.MustCompile(`\b(general admission|ga)\b`)
	reSuperEarlyBird = regexp.MustCompile(`\bsuper early bird`)
	reEarlyBird      = regexp.MustCompile(`\bearly bird`)
	// reFinalRelease matches "final release", "final tickets", or bare "final"
	// only at end-of-string. The bare-final arm requires a word boundary and
	// trailing whitespace-or-end so "final countdown" and "finalists" do not
	// match.
	reFinalRelease = regexp.MustCompile(`(?:\bfinal (release|tickets)\b)|(?:\bfinal\b\s*$)`)
	reLastChance   = regexp.MustCompile(`\blast chance\b`)
	reTierN        = regexp.MustCompile(`\b(tier|ga) ?(\d+)\b`)
	// reDeadline matches existing "must enter/arrive by <time>" phrasing.
	reDeadline = regexp.MustCompile(`must (?:enter|arrive) by (\d{1,2})(?::(\d{2}))? ?(am|pm)`)
	// reDeadlineSynonym matches additional entry-window deadline phrasings:
	//   "entry by 9pm", "enter before 10pm", "arrive before 8pm",
	//   "must be in by 11pm", "be in by 10pm".
	// Capture groups: (1) hour, (2) optional minutes, (3) am/pm.
	reDeadlineSynonym = regexp.MustCompile(`(?:must )?(?:enter|entry|arrive|be in) (?:by|before) (\d{1,2})(?::(\d{2}))? ?(am|pm)`)
	reAnytime         = regexp.MustCompile(`\banytime\b`)
	reDoor            = regexp.MustCompile(`\bat the door\b|\bat door\b`)
	reYouPlus         = regexp.MustCompile(`you ?\+ ?(\d+)`)
	reGroupParens     = regexp.MustCompile(`\b(?:group|party|table)\b.*\((\d+)\)`)
	// reTableBoothN matches "table/booth (of|for)? <number>". Bare table/booth
	// without a number must NOT set group_size (ambiguous seating vs. ticket class).
	reTableBoothN  = regexp.MustCompile(`\b(?:table|booth) (?:of |for )?(\d+)\b`)
	reDoubleTriple = regexp.MustCompile(`\b(double|triple|quad)\b`)
	reComp         = regexp.MustCompile(`\bcomp\b|complimentary|reward ticket|guest ?list|guestlist|\bguest\b`)
)

// extractTierAxes extracts structured axes from a canonicalized ticket-type
// name. The input must already be processed by canonicalizeName.
func extractTierAxes(s string) tierAxes {
	var ax tierAxes

	// Access class.
	if reAccessVIP.MatchString(s) {
		ax.AccessClass = "vip"
		ax.Matched = true
	} else if reAccessGA.MatchString(s) {
		ax.AccessClass = "ga"
		ax.Matched = true
	}

	// Sales stage — check super_early_bird before early_bird to avoid prefix match.
	if reSuperEarlyBird.MatchString(s) {
		ax.SalesStage = "super_early_bird"
		ax.Matched = true
	} else if reEarlyBird.MatchString(s) {
		ax.SalesStage = "early_bird"
		ax.Matched = true
	} else if reFinalRelease.MatchString(s) {
		ax.SalesStage = "final_release"
		ax.Matched = true
	} else if reLastChance.MatchString(s) {
		ax.SalesStage = "last_chance"
		ax.Matched = true
	} else if reTierN.MatchString(s) {
		ax.SalesStage = "tier_n"
		ax.Matched = true
	}

	// Entry window — deadline parses time to 24h HH:MM.
	// reDeadline handles "must enter/arrive by <time>"; reDeadlineSynonym covers
	// the broader synonym set. Try the narrower pattern first so the existing
	// "must enter by" and "must arrive by" tests continue to match unchanged.
	var deadlineMatch []string
	if m := reDeadline.FindStringSubmatch(s); m != nil {
		deadlineMatch = m
	} else if m := reDeadlineSynonym.FindStringSubmatch(s); m != nil {
		deadlineMatch = m
	}
	if deadlineMatch != nil {
		hour, _ := strconv.Atoi(deadlineMatch[1])
		minStr := deadlineMatch[2]
		ampm := deadlineMatch[3]
		var minVal int
		if minStr != "" {
			minVal, _ = strconv.Atoi(minStr)
		}
		ax.EntryWindowType = "deadline"
		ax.EntryWindowTime = parse12hTo24h(hour, minVal, ampm)
		ax.Matched = true
	} else if reAnytime.MatchString(s) {
		ax.EntryWindowType = "anytime"
		ax.Matched = true
	} else if reDoor.MatchString(s) {
		ax.EntryWindowType = "door"
		ax.Matched = true
	}

	// Group size.
	if m := reYouPlus.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		ax.GroupSize = n + 1
		ax.Matched = true
	} else if m := reGroupParens.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		ax.GroupSize = n
		ax.Matched = true
	} else if m := reTableBoothN.FindStringSubmatch(s); m != nil {
		// "table for 6" / "booth of 4" / "table 3" -> group_size N.
		// Bare "table"/"booth" without a number does not reach this branch.
		n, _ := strconv.Atoi(m[1])
		ax.GroupSize = n
		ax.Matched = true
	} else if m := reDoubleTriple.FindStringSubmatch(s); m != nil {
		switch m[1] {
		case "double":
			ax.GroupSize = 2
		case "triple":
			ax.GroupSize = 3
		case "quad":
			ax.GroupSize = 4
		}
		ax.Matched = true
	}

	// Comp flag.
	if reComp.MatchString(s) {
		ax.CompFlag = true
		ax.Matched = true
	}

	return ax
}

// parse12hTo24h converts a 12-hour time (hour, minute, "am"/"pm") to "HH:MM".
func parse12hTo24h(hour, min int, ampm string) string {
	ampm = strings.ToLower(strings.TrimSpace(ampm))
	switch ampm {
	case "pm":
		if hour != 12 {
			hour += 12
		}
	case "am":
		if hour == 12 {
			hour = 0
		}
	}
	return fmt.Sprintf("%02d:%02d", hour, min)
}
