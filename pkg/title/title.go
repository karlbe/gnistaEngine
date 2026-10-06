// Package title is the title sequence that the original shows before the game ($6690,
// $6852): the logo, the intro text, an animated silhouette with scrolling credits, a
// "press fire" screen and the story. Like the game state it is plain data stepped once per
// frame, without any drawing.
package title

// The original waits with busy loops ("subi.l #1,d0 / bne"), measured at about 5263
// iterations per 50 Hz frame (the same figure the endings use). Loading the pictures from
// floppy is not reproduced.
const iterationsPerFrame = 5263

func frames(iterations int) int { return (iterations + iterationsPerFrame/2) / iterationsPerFrame }

// Credits scroll one pixel per pass of a loop of $300C iterations ($68BA, about 2.34
// frames), and a new line enters below the picture every 15 passes ($689E).
const (
	scrollPassFrames100 = 234 // 100 * frames per pass
	LinePixels          = 15  // pixels between two credit lines
)

// Step is one stage of the sequence.
type Step struct {
	Picture string // file name of the picture ("" = black)
	Palette string // palette table to show it with ($7316...), "" = the picture's own palette
	Frames  int    // how long it stays; 0 = until fire
	Credits bool   // the credits scroll over the picture until they have all passed
	OnFire  int    // the step fire goes to; -1 = the next one
}

// Indices into sequence.
const (
	stepIntroText = 2
	stepPoses     = 4
	stepCredits   = 8
	stepStory     = 10
)

// sequence is the order of $6690: the logo ($6718 on: black, then the picture), the intro
// text, then the silhouette picture, whose arm and rifle move through five palette tables
// ($6728-$67B8: 7316, 7356, 7396, 7356, 7396), under the scrolling credits ($6852). When
// those are through, the original waits for fire on the "press fire" picture ($67C4-$681E)
// and then shows the story ($682A) while it loads the game. Fire during the credits goes
// straight to the story, as in the original. Not in the original: fire also skips the pages
// before (the logo and the black gaps go on to the intro text, the intro text on to the
// silhouette, which goes on to the credits), so a press always moves one page on.
func sequence() []Step {
	pose := frames(0xC350) // the delay after each palette change
	return []Step{
		{Picture: "NSILoader", Frames: frames(0x10C8E0), OnFire: stepIntroText},
		{Frames: frames(0x186A0), OnFire: stepIntroText},
		{Picture: "IT", Frames: frames(0x186A00), OnFire: stepPoses},
		{Frames: frames(0x186A0), OnFire: stepPoses},
		{Picture: "TA", Palette: "7316", Frames: pose, OnFire: stepCredits},
		{Picture: "TA", Palette: "7356", Frames: pose, OnFire: stepCredits},
		{Picture: "TA", Palette: "7396", Frames: pose, OnFire: stepCredits},
		{Picture: "TA", Palette: "7356", Frames: pose, OnFire: stepCredits},
		{Picture: "TA", Palette: "7396", Credits: true, OnFire: stepStory},
		{Picture: "PF", OnFire: stepStory},
		{Picture: "ST", OnFire: -1},
	}
}

// Title is the running sequence.
type Title struct {
	Steps []Step
	Step  int
	Frame int // frames spent in the current step
	Lines int // the number of credit lines
	Fire  bool

	// Improve switches on fire skipping the pages before the credits. The original only
	// reacts to fire during the credits and on the "press fire" and story pages.
	Improve bool
}

// New starts the sequence for credits of the given number of lines.
func New(creditLines int, improve bool) *Title {
	return &Title{Steps: sequence(), Lines: creditLines, Improve: improve}
}

// Current returns the step on screen, or nil when the sequence is over.
func (t *Title) Current() *Step {
	if t.Step >= len(t.Steps) {
		return nil
	}
	return &t.Steps[t.Step]
}

// Done reports whether the sequence is over and the game should start.
func (t *Title) Done() bool { return t.Step >= len(t.Steps) }

// Skip ends the sequence at once.
func (t *Title) Skip() { t.Step = len(t.Steps) }

// ScrollPixels is how far the credits have scrolled in the current step.
func (t *Title) ScrollPixels() int { return t.Frame * 100 / scrollPassFrames100 }

// CreditsDone reports whether every credit line has been printed and has passed its 15
// pixels, which is when the original's loop ends (the line after the last starts with $01).
func (t *Title) CreditsDone() bool { return t.ScrollPixels() >= LinePixels*t.Lines }

// Update advances the sequence by one frame. fire is the fire button; a screen reacts to
// a press, so holding fire does not run through several screens. musicDone is the song's
// end flag ($9A88): when the credits have all passed, the original waits for it ($67C4)
// with the text still on the screen.
func (t *Title) Update(fire, musicDone bool) {
	cur := t.Current()
	if cur == nil {
		return
	}
	pressed := fire && !t.Fire
	t.Fire = fire
	switch {
	case pressed && (t.Improve || cur.Credits || cur.Frames == 0):
		next := cur.OnFire
		if next < 0 {
			next = t.Step + 1
		}
		t.goTo(next)
	case cur.Credits:
		if !t.CreditsDone() {
			t.Frame++
		}
		if t.CreditsDone() && musicDone {
			t.goTo(t.Step + 1)
		}
	case cur.Frames == 0:
		// waits for fire
	default:
		if t.Frame++; t.Frame >= cur.Frames {
			t.goTo(t.Step + 1)
		}
	}
}

func (t *Title) goTo(step int) {
	t.Step, t.Frame = step, 0
}
