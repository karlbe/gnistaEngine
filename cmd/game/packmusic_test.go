package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/audio"
	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/input"
	"github.com/karlbe/gnistaEngine/pkg/pack"
)

func track(ticks int) *audio.Track {
	return &audio.Track{Rate: 50, PCM: make([]int16, 2*ticks)}
}

func TestPackMusic(t *testing.T) {
	orig := audio.NewTracker(nil, nil, nil, audio.Silent{})
	p := &packMusic{orig: orig, title: track(30)}

	// Before anything starts, the title has nothing to wait for.
	if !p.Finished() {
		t.Error("nothing plays, so the music is finished")
	}
	p.StartSong()
	if p.Finished() {
		t.Fatal("the title track has just started")
	}
	for i := 0; i < 29; i++ {
		p.Tick()
	}
	if p.Finished() {
		t.Fatal("finished one tick early")
	}
	p.Tick()
	if !p.Finished() {
		t.Fatal("the title track is over")
	}

	p.game = track(100) // the game track needs no tracker: it loops by itself
	p.StartAmbient()
	if p.Finished() {
		t.Error("a looping game track does not finish")
	}
	for i := 0; i < 500; i++ {
		p.Tick()
	}
	if p.Finished() {
		t.Error("a looping game track does not finish")
	}
}

// A pack that brings everything it needs runs with no original game files at all. This one is
// as small as it gets: the starter pack's palette, tiles and map, and two scripts.
func TestPackStandsAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mini")
	if err := pack.WriteStarter(dir); err != nil {
		t.Fatal(err)
	}
	scripts := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "script idle\n" +
		"    controls facing right right=walk default=idle\n" +
		"    show legs=0 body=64\n" +
		"    wait\n" +
		"    goto idle\n" +
		"script walk\n" +
		"    camera follow right\n" +
		"    hold legs=1 body=64 for 32 ticks moving right\n" +
		"    camera stop\n" +
		"    resume controls right\n"
	if err := os.WriteFile(filepath.Join(scripts, "mini.gs"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	prog := `{"player":"idle","idle":[["idle","idle","idle"],["idle","idle","idle"]],"mag_size":[7,7,7],"wave_order":[0]}`
	if err := os.WriteFile(filepath.Join(dir, "program.json"), []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "no original here")
	a, err := load(content.Loader{Root: root}, dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		a.state.Step(input.Right)
	}
	if int(a.state.Engine.G.ViewX) < a.startX+64 {
		t.Errorf("the player did not walk: view x %d from %d", a.state.Engine.G.ViewX, a.startX)
	}
	// Drawing the game needs only what the pack has.
	a.render.Draw(nil, a.state, a.content.Level)
}
