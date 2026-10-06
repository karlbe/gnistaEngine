package audio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
)

type loaded struct {
	song           *Song
	title, ambient []Entry
	code           []byte
}

func loadMusic(t *testing.T) loaded {
	root := filepath.Join("..", "..", "assets-local")
	read := func(p ...string) []byte {
		b, err := os.ReadFile(filepath.Join(append([]string{root}, p...)...))
		if err != nil {
			t.Skip("original data not available")
		}
		return b
	}
	code := read("extracted", "code.bin")
	song, err := ParseSong(read("adf", "NSIA"))
	if err != nil {
		t.Fatal(err)
	}
	sample := func(name string) []int8 {
		s, err := amiga.DecodeSample(read("adf", name))
		if err != nil {
			t.Fatal(err)
		}
		return s.Data
	}
	title, err := ParseMusicInstruments(code)
	if err != nil {
		t.Fatal(err)
	}
	bank := sample("NSIMusicSound")
	for i := range title {
		title[i].Data = bank
	}
	words, err := ParseAmbientLengths(code)
	if err != nil {
		t.Fatal(err)
	}
	var ambient []Entry
	for i, name := range []string{"IAZ", "DAS", "DBY"} {
		ambient = append(ambient, Entry{Words: words[i], Data: sample(name)})
	}
	return loaded{song, title, ambient, code}
}

type note struct {
	tick  int
	ch    int
	start bool // false: LoadEntry
	e     Entry
}

type recorder struct {
	now   *int
	notes []note
}

func (r *recorder) LoadEntry(ch int, e Entry) { r.notes = append(r.notes, note{*r.now, ch, false, e}) }
func (r *recorder) Start(ch int, once bool) {
	if !once {
		panic("music notes play once")
	}
	r.notes = append(r.notes, note{*r.now, ch, true, Entry{}})
}

func TestParseSongAndInstruments(t *testing.T) {
	m := loadMusic(t)
	if len(m.title) != 10 {
		t.Fatalf("%d title instruments, want 10", len(m.title))
	}
	for i := 0; i+1 < len(m.title); i++ {
		if m.title[i+1].Offset <= m.title[i].Offset {
			t.Errorf("instrument %d starts at %d, not after instrument %d at %d", i+1, m.title[i+1].Offset, i, m.title[i].Offset)
		}
	}
	last := m.title[len(m.title)-1]
	if last.Offset+2*last.Words > len(m.title[0].Data)+4 { // the original reads 2 bytes past the file
		t.Errorf("the last instrument runs past the sample file")
	}
	if len(m.ambient) != 3 || m.ambient[0].Words != 2500 || m.ambient[1].Words != 2000 || m.ambient[2].Words != 1000 {
		t.Errorf("ambient samples %v", []int{m.ambient[0].Words, m.ambient[1].Words, m.ambient[2].Words})
	}
	if m.song.patterns[0] != 0 || m.song.patterns[1] <= m.song.patterns[0] {
		t.Errorf("pattern starts %v", m.song.patterns[:3])
	}
}

// The title song plays to its end by itself: each voice walks its order list, and a note
// is loaded one tick before its DMA is switched on.
func TestTitleSongPlaysToTheEnd(t *testing.T) {
	m := loadMusic(t)
	now := 0
	rec := &recorder{now: &now}
	tr := NewTracker(m.song, m.title, m.ambient, rec)
	if !tr.Finished() {
		t.Fatal("a new tracker should be stopped until a song is started")
	}
	tr.StartSong()
	for now = 1; !tr.Finished() && now < 50*60*10; now++ {
		tr.Tick()
	}
	if !tr.Finished() {
		t.Fatal("the song never ended")
	}
	t.Logf("the title song ends after %d ticks = %.1f s, %d note events", now, float64(now)/50, len(rec.notes)/2)
	if now < 50*20 {
		t.Errorf("the song is only %d ticks long", now)
	}
	// Every note: loaded, then started on the next tick for the same channel.
	pending := map[int]int{}
	for _, n := range rec.notes {
		if !n.start {
			pending[n.ch] = n.tick
			if n.e.Volume > 64 || n.e.Period < 100 || n.e.Words == 0 {
				t.Fatalf("bad note on channel %d: period %d, volume %d, %d words", n.ch, n.e.Period, n.e.Volume, n.e.Words)
			}
		} else if loadedAt, ok := pending[n.ch]; !ok || n.tick != loadedAt+1 {
			t.Fatalf("channel %d started at %d, loaded at %d (%v)", n.ch, n.tick, loadedAt, ok)
		} else {
			delete(pending, n.ch)
		}
	}
	used := map[int]bool{}
	for _, n := range rec.notes {
		used[n.ch] = true
	}
	if len(used) != 4 {
		t.Errorf("only %d voices played", len(used))
	}
}

// The in-game track uses voices 2 and 3 only, and starts over when it ends.
func TestAmbientTrack(t *testing.T) {
	m := loadMusic(t)
	now := 0
	rec := &recorder{now: &now}
	tr := NewTracker(m.song, m.title, m.ambient, rec)
	tr.StartAmbient()
	for now = 1; now <= 50*60*5; now++ {
		tr.Tick()
	}
	if tr.Finished() {
		t.Fatal("the ambient track ended")
	}
	if len(rec.notes) == 0 {
		t.Fatal("the ambient track made no notes")
	}
	for _, n := range rec.notes {
		if n.ch < 2 {
			t.Fatalf("voice %d played in the ambient track", n.ch)
		}
		if !n.start && n.e.Words > 2500 {
			t.Fatalf("an ambient note longer than the IAZ sample: %d words", n.e.Words)
		}
	}
	// An enemy on screen stops it; it goes on when the enemy is gone.
	before := len(rec.notes)
	tr.SetStopped(true)
	for i := 0; i < 500; i++ {
		tr.Tick()
	}
	if len(rec.notes) != before {
		t.Error("notes were played while stopped")
	}
	tr.SetStopped(false)
	for i := 0; i < 500; i++ {
		tr.Tick()
	}
	if len(rec.notes) == before {
		t.Error("the track did not go on after being stopped")
	}
}
