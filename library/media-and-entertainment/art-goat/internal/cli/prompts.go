// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

// pp:novel-static-reference
//
// art-goat contemplative prompts. This is hand-curated reference data
// scoped by the anti-reimplementation carve-out — the CLI provides the
// frame ("what to notice"); the museum API provides the content ("what's
// in front of you"). Prompts are deliberately universal: they work on a
// Hokusai woodblock, an Egyptian relief, a Rothko, or a Hubble image.
//
// Keep the list short. ~25 universal frames is enough; expanding past
// ~50 would drift toward synthesized content the rule rejects.

package cli

import (
	"math/rand"
	"time"
)

var contemplativePrompts = []string{
	"What do you notice first? What changes the longer you look?",
	"Where is your eye drawn? Where does it want to go next?",
	"What is this work refusing to tell you?",
	"What time of day is this? What season? What weather?",
	"What was the artist's body doing while making this?",
	"If you could touch one part of this, what would it be?",
	"What is just outside the frame?",
	"What sound, if any, would this make?",
	"What does this remind you of from your own life?",
	"What would you remove from this if you could? What would you add?",
	"Where is the silence in this work?",
	"What is the smallest detail you can find?",
	"If this were a single line of text, what would it say?",
	"What feeling is the maker asking you to share?",
	"What feeling do you bring to it that they didn't intend?",
	"How long do you think this took to make? Why does that matter?",
	"What technology, materials, or hands made this possible?",
	"What is this work an argument against?",
	"What did the maker know that you don't?",
	"What do you know now that the maker couldn't have?",
	"Which part of this would you photograph if you could only take one frame?",
	"What is the scale of what you are looking at?",
	"If you sat with this for a week, what would change in how you see it?",
	"What does this work want from you? Are you willing to give it?",
	"What does it cost to look this carefully?",
}

// pickPrompt returns a deterministic prompt for a given seed (so today's
// pick is the same for a given work + date), or a random one when seed=0.
func pickPrompt(seed int64) string {
	r := rand.New(rand.NewSource(seed))
	if seed == 0 {
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return contemplativePrompts[r.Intn(len(contemplativePrompts))]
}
