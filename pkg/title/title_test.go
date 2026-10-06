package title

import "testing"

// run steps the title with no fire until it is on a step that waits (fire or credits) or is over, and
// reports how many frames that took.
func run(t *Title, limit int) int {
	for i := 0; i < limit; i++ {
		if cur := t.Current(); cur == nil || cur.Frames == 0 {
			return i
		}
		t.Update(false, true)
	}
	return limit
}

func TestSequenceOrder(t *testing.T) {
	s := sequence()
	var pics []string
	for _, st := range s {
		if len(pics) == 0 || pics[len(pics)-1] != st.Picture || st.Picture == "" {
			pics = append(pics, st.Picture)
		}
	}
	want := []string{"NSILoader", "", "IT", "", "TA", "PF", "ST"}
	if len(pics) != len(want) {
		t.Fatalf("pictures %q, want %q", pics, want)
	}
	for i := range want {
		if pics[i] != want[i] {
			t.Fatalf("pictures %q, want %q", pics, want)
		}
	}
	if s[stepStory].Picture != "ST" {
		t.Errorf("stepStory points at %q", s[stepStory].Picture)
	}
}

// The delays come from the busy loops: the logo's $10C8E0 iterations and the intro text's
// $186A00 are about 209 and 304 frames.
func TestDelaysFromTheLoops(t *testing.T) {
	s := sequence()
	if s[0].Frames != 209 || s[2].Frames != 304 {
		t.Errorf("logo %d frames, intro text %d; want 209 and 304", s[0].Frames, s[2].Frames)
	}
}

func TestRunsToTheCreditsThenThePrompt(t *testing.T) {
	ti := New(90, true)
	n := run(ti, 5000)
	cur := ti.Current()
	if cur == nil || !cur.Credits {
		t.Fatalf("stopped on %+v after %d frames, want the credits", cur, n)
	}
	if got := ti.Steps[ti.Step].Palette; got != "7396" {
		t.Errorf("the credits run on palette %s, want 7396", got)
	}
	// All lines scroll past: 90 lines of 15 pixels at 2.34 frames a pixel.
	frames := 0
	for ; ti.Current() != nil && ti.Current().Credits && frames < 10000; frames++ {
		ti.Update(false, true)
	}
	if want := 90 * LinePixels * 234 / 100; frames < want-5 || frames > want+5 {
		t.Errorf("the credits took %d frames, want about %d", frames, want)
	}
	if cur := ti.Current(); cur == nil || cur.Picture != "PF" {
		t.Fatalf("after the credits: %+v, want the press fire picture", cur)
	}
}

func TestFireDuringTheCreditsGoesToTheStory(t *testing.T) {
	ti := New(90, true)
	run(ti, 5000)
	for i := 0; i < 100; i++ {
		ti.Update(false, true)
	}
	ti.Update(true, true)
	if cur := ti.Current(); cur == nil || cur.Picture != "ST" {
		t.Fatalf("fire in the credits went to %+v, want the story", cur)
	}
}

func TestPromptNeedsAPressNotAHold(t *testing.T) {
	ti := New(90, true)
	run(ti, 5000)
	for ti.Current() != nil && ti.Current().Credits {
		ti.Update(false, true)
	}
	if ti.Current().Picture != "PF" {
		t.Fatal("not on the prompt")
	}
	ti.Update(true, true) // pressed: on to the story
	if ti.Current().Picture != "ST" {
		t.Fatalf("fire on the prompt went to %q", ti.Current().Picture)
	}
	for i := 0; i < 50; i++ {
		ti.Update(true, true) // still held: stays on the story
	}
	if ti.Done() || ti.Current().Picture != "ST" {
		t.Fatal("a held fire ran through the story")
	}
	ti.Update(false, true)
	ti.Update(true, true)
	if !ti.Done() {
		t.Fatal("a new press after the story did not end the title")
	}
}

// Fire moves one page on from every screen: logo, intro text, silhouette, credits (to the
// story, as in the original), story (out).
func TestFireSkipsThroughThePages(t *testing.T) {
	ti := New(90, true)
	var seen []string
	for i := 0; i < 10 && !ti.Done(); i++ {
		for j := 0; j < 3; j++ { // a few frames on each page first
			ti.Update(false, true)
		}
		seen = append(seen, ti.Current().Picture)
		ti.Update(true, true)
		ti.Update(false, true)
	}
	want := []string{"NSILoader", "IT", "TA", "TA", "ST"}
	if len(seen) != len(want) {
		t.Fatalf("pages %q, want %q", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("pages %q, want %q", seen, want)
		}
	}
	if !ti.Done() {
		t.Error("the title did not end after fire on the story")
	}
}

func TestFireInTheBlackGapSkipsIt(t *testing.T) {
	ti := New(90, true)
	for ti.Step != 1 { // the black screen after the logo
		ti.Update(false, true)
	}
	ti.Update(true, true)
	if cur := ti.Current(); cur == nil || cur.Picture != "IT" {
		t.Fatalf("fire in the black gap went to %+v, want the intro text", cur)
	}
}

func TestSkip(t *testing.T) {
	ti := New(90, true)
	ti.Skip()
	if !ti.Done() || ti.Current() != nil {
		t.Fatal("Skip did not end the sequence")
	}
	ti.Update(true, true) // harmless after the end
}

// When the credits have passed, the original waits for the song to end; the text stays.
func TestCreditsWaitForTheSong(t *testing.T) {
	ti := New(90, true)
	run(ti, 5000)
	for i := 0; i < 90*LinePixels*3; i++ {
		ti.Update(false, false)
	}
	cur := ti.Current()
	if cur == nil || !cur.Credits {
		t.Fatalf("left the credits before the song ended: %+v", cur)
	}
	if !ti.CreditsDone() {
		t.Fatal("the credits should have scrolled to the end")
	}
	px := ti.ScrollPixels()
	ti.Update(false, false)
	if ti.ScrollPixels() != px {
		t.Error("the text moved while waiting for the song")
	}
	ti.Update(false, true)
	if cur := ti.Current(); cur == nil || cur.Picture != "PF" {
		t.Fatalf("after the song: %+v, want the press fire picture", cur)
	}
}

// Without the improvements fire only works where the original polls it: during the credits
// and on the "press fire" and story pages.
func TestOriginalFireHandling(t *testing.T) {
	ti := New(90, false)
	for i := 0; i < 20; i++ {
		ti.Update(i%2 == 0, true) // presses on the logo are ignored
	}
	if ti.Step != 0 {
		t.Fatalf("fire on the logo moved to step %d", ti.Step)
	}
	run(ti, 5000)
	if cur := ti.Current(); cur == nil || !cur.Credits {
		t.Fatalf("did not reach the credits: %+v", cur)
	}
	ti.Update(false, true)
	ti.Update(true, true)
	if cur := ti.Current(); cur == nil || cur.Picture != "ST" {
		t.Fatalf("fire in the credits went to %+v, want the story", cur)
	}
}
