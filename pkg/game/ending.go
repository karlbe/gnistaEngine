package game

import "github.com/karlbe/gnistaEngine/pkg/input"

// Outcomes ($21EA). The original never sets 1 (which would show DO).
const (
	OutcomeExplosion = 2 // time ran out, the player died or cut the wrong wire
	OutcomeWin       = 3 // the right wire
)

// Frame counts of the ending's busy loops, measured on the emulator (the loops run about
// 4% slower than their cycle counts because of DMA). Loading the pictures from floppy
// (1-3 s each) is not reproduced.
const (
	flashFrames   = 95  // $94: 500000 iterations with the flash palette
	blackFrames   = 22  // $188: 100000 iterations
	pictureFrames = 380 // $EC: 2000000 iterations per picture
)

// EndingStep is one screen of an ending: the game view with the flash palette, black, or
// a picture, shown for a number of frames or until fire.
type EndingStep struct {
	Flash   bool
	Picture string // file name on the disk; "" is black
	Frames  int
	Fire    bool
}

// Ending is the sequence the original's main loop shows after the game ($30-$130).
type Ending struct {
	Steps []EndingStep
	Step  int
	Wait  int
}

// Current returns the step on screen, or nil when the ending is over.
func (e *Ending) Current() *EndingStep {
	if e.Step >= len(e.Steps) {
		return nil
	}
	return &e.Steps[e.Step]
}

func (s *State) newEnding() *Ending {
	e := &Ending{}
	if s.Engine.G.Outcome == OutcomeExplosion {
		e.Steps = []EndingStep{
			{Flash: true, Frames: flashFrames},
			{Frames: blackFrames},
			{Picture: "ET", Frames: pictureFrames},
			{Picture: "EX", Frames: pictureFrames},
		}
	} else {
		e.Steps = []EndingStep{
			{Frames: blackFrames},
			{Picture: "WT", Fire: true},
			{Picture: "HC", Fire: true},
			{Picture: "EN", Fire: true},
		}
	}
	e.Wait = e.Steps[0].Frames
	return e
}

// stepEnding advances the ending by one frame. A step that waits for fire ends as soon as
// fire is down, as the original polls the button.
func (e *Ending) step(in input.Actions) {
	cur := e.Current()
	if cur == nil {
		return
	}
	if cur.Fire {
		if !in.Has(input.Fire) {
			return
		}
	} else if e.Wait--; e.Wait > 0 {
		return
	}
	e.Step++
	if next := e.Current(); next != nil {
		e.Wait = next.Frames
	}
}

// Finished reports whether the ending has been shown; the original then starts again
// from the title.
func (s *State) Finished() bool { return s.Ending != nil && s.Ending.Current() == nil }
