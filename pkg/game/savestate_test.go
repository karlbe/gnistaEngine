package game

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/input"
)

// route is a fixed input sequence that walks, shoots, rolls and climbs, and meets enemies.
func route(i int) input.Actions {
	seq := []input.Actions{input.Right, input.Right | input.Down, input.Right, input.Fire, input.Up, 0, input.Left, input.Left | input.Fire, input.Down, input.Right | input.Fire}
	return seq[i/25%len(seq)]
}

// Saving in the middle of a game and loading gives a state that goes on exactly like the
// original: both then save to the same bytes.
func TestSaveStateRoundTrip(t *testing.T) {
	c, e := load(t)
	a := New(c, e.StartX, e.StartY)
	for i := 0; i < 700; i++ {
		a.Step(route(i))
	}
	data, err := a.Save()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("save state: %d bytes, tick %d", len(data), a.Tick)
	b, err := LoadState(c, data)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := b.Save(); !bytes.Equal(again, data) {
		t.Fatal("a loaded state does not save to the same bytes")
	}
	for i := 700; i < 1700; i++ {
		a.Step(route(i))
		b.Step(route(i))
		if a.Tick%100 == 0 {
			da, _ := a.Save()
			db, _ := b.Save()
			if !bytes.Equal(da, db) {
				t.Fatalf("the states differ at tick %d", a.Tick)
			}
		}
	}
	da, _ := a.Save()
	db, _ := b.Save()
	if !bytes.Equal(da, db) {
		t.Fatalf("the states differ after the run")
	}
}

// The actor links survive: a state saved with enemies and the lift in use still draws
// and moves them.
func TestSaveStateKeepsActors(t *testing.T) {
	c, e := load(t)
	a := New(c, e.StartX, e.StartY)
	a.Step(0)
	a.firstEnemy(a.freeSlot())
	for i := 0; i < 5; i++ {
		a.Step(0)
	}
	data, err := a.Save()
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadState(c, data)
	if err != nil {
		t.Fatal(err)
	}
	if b.Engine.Slots[1].Actor != &b.env.enemies[0] || b.Engine.Slots[0].Actor == nil {
		t.Fatal("the slots do not point at the loaded actors")
	}
	if *b.Engine.Slots[1].Actor != *a.Engine.Slots[1].Actor {
		t.Fatalf("enemy actor differs: %+v vs %+v", *b.Engine.Slots[1].Actor, *a.Engine.Slots[1].Actor)
	}
}

func TestLoadRejectsGarbage(t *testing.T) {
	c, _ := load(t)
	for _, d := range [][]byte{nil, []byte("hello"), []byte(saveMagic), []byte(saveMagic + "not gzip")} {
		if _, err := LoadState(c, d); err == nil {
			t.Errorf("LoadState(%q) did not fail", d)
		}
	}
}

// stepsOf expands a route like the one TestFirstDoor plays into one input per tick.
func stepsOf(route string) []input.Actions {
	names := map[string]input.Actions{"none": 0, "up": input.Up, "down": input.Down, "left": input.Left, "right": input.Right, "fire": input.Fire}
	var out []input.Actions
	for _, part := range strings.Split(route, ",") {
		name, n, _ := strings.Cut(part, ":")
		ticks, _ := strconv.Atoi(n)
		for i := 0; i < ticks; i++ {
			out = append(out, names[name])
		}
	}
	return out
}

// The walkthrough to the first door passes an enemy fight, stairs, the lift and a room.
// A save made at any point along it continues exactly like the game it was taken from.
func TestSaveStateAlongWalkthrough(t *testing.T) {
	c, e := load(t)
	in := stepsOf("right:125,none:50,left:8,none:50,up:150,left:10,none:75,fire:300,none:50,left:350,none:50,up:600,fire:20,none:200,right:150,none:50,up:20,none:60,down:1,none:30")
	main := New(c, e.StartX, e.StartY)
	seen := map[string]bool{}
	for i, a := range in {
		main.Step(a)
		seen["enemy"] = seen["enemy"] || main.Engine.Slots[1].Actor != nil
		seen["lift"] = seen["lift"] || main.Engine.Slots[script.HelperSlot].Actor != nil
		seen["room"] = seen["room"] || main.Room != nil
		seen["corpse"] = seen["corpse"] || len(main.Stamps) > 0
		if i%97 != 0 { // a prime step, so the saves fall on every phase of the cycles
			continue
		}
		data, err := main.Save()
		if err != nil {
			t.Fatal(err)
		}
		clone, err := LoadState(c, data)
		if err != nil {
			t.Fatalf("tick %d: %v", main.Tick, err)
		}
		end := min(i+1+120, len(in))
		for _, a := range in[i+1 : end] {
			clone.Step(a)
		}
		got, _ := clone.Save()
		want, _ := replay(c, e, in[:end]).Save()
		if !bytes.Equal(got, want) {
			t.Fatalf("a save at tick %d does not continue like the game it came from", main.Tick)
		}
	}
	for _, what := range []string{"enemy", "lift", "room", "corpse"} {
		if !seen[what] {
			t.Errorf("the walkthrough never had a %s, so saves were not tried with one", what)
		}
	}
}

// replay plays inputs on a fresh game.
func replay(c Content, e *content.Extracted, in []input.Actions) *State {
	s := New(c, e.StartX, e.StartY)
	for _, a := range in {
		s.Step(a)
	}
	return s
}

// After the clock runs out the ending sequence can be saved and loaded too.
func TestSaveStateInEnding(t *testing.T) {
	c, e := load(t)
	s := New(c, e.StartX, e.StartY)
	s.Clock = Clock{Frames: 49, Sec: 9, Sec10: 5, Min: 9, Min10: 5}
	for i := 0; i < 400; i++ {
		s.Step(0)
	}
	if s.Ending == nil {
		t.Fatal("the clock did not end the game")
	}
	data, err := s.Save()
	if err != nil {
		t.Fatal(err)
	}
	r, err := LoadState(c, data)
	if err != nil {
		t.Fatal(err)
	}
	if r.Ending == nil || r.Ending.Step != s.Ending.Step {
		t.Fatalf("the ending was not restored: %+v", r.Ending)
	}
	for i := 0; i < 600; i++ {
		s.Step(0)
		r.Step(0)
	}
	a, _ := s.Save()
	b, _ := r.Save()
	if !bytes.Equal(a, b) {
		t.Fatal("the ending continues differently after loading")
	}
}

// A clone is independent of the state it came from and continues exactly like it.
func TestCloneIsIndependent(t *testing.T) {
	c, e := load(t)
	a := New(c, e.StartX, e.StartY)
	for i := 0; i < 400; i++ {
		a.Step(route(i))
	}
	b := a.Clone()
	da, _ := a.Save()
	db, _ := b.Save()
	if !bytes.Equal(da, db) {
		t.Fatal("a clone does not save like its original")
	}
	for i := 400; i < 900; i++ {
		b.Step(route(i)) // only the clone moves on
	}
	if again, _ := a.Save(); !bytes.Equal(again, da) {
		t.Fatal("stepping the clone changed the original")
	}
	for i := 400; i < 900; i++ {
		a.Step(route(i))
	}
	da, _ = a.Save()
	db, _ = b.Save()
	if !bytes.Equal(da, db) {
		t.Fatal("original and clone diverged")
	}
}
