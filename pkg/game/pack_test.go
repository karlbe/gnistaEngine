package game

import (
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/input"
	"github.com/karlbe/gnistaEngine/pkg/pack"
)

// A map made from a pack runs on the original's scripts: the player walks, the door at the
// map's first door tile opens the room the map's object asked for, and the enemy trigger
// brings an enemy.
func TestOwnMap(t *testing.T) {
	c, _ := load(t)
	dir := t.TempDir()
	if err := pack.WriteStarter(dir); err != nil {
		t.Fatal(err)
	}
	p, err := pack.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	level, err := p.Level()
	if err != nil || level == nil {
		t.Fatalf("level: %v %v", level, err)
	}
	c.Level = level
	start := func() *State { return New(c, level.T.StartViewX, level.T.StartViewY) }
	s := start()
	if got, want := s.Engine.Slots[0].X, int16(14*16+8); got < want-16 || got > want+16 {
		t.Fatalf("the player starts at x=%d, want about %d", got, want)
	}

	// A step is a whole tile, and the player coasts on after the key is let go. Find the point
	// to let go at which the player ends up on the door tile.
	var onDoor *State
	for release := int16(330); release < 400 && onDoor == nil; release += 2 {
		try := start()
		for n := 0; try.Engine.Slots[0].X < release; n++ {
			try.Step(input.Right)
		}
		play(t, try, "none:60")
		if try.DoorHere().Door {
			onDoor = try
		}
	}
	if onDoor == nil {
		t.Fatal("no way to stop on the door tile")
	}
	s = onDoor
	play(t, s, "up:5")
	if s.Room == nil {
		t.Fatalf("up at the door did not open a room: %+v", s.DoorHere())
	}
	// The first door of the starter map is room 0: picture 1, message 1.
	if s.Room.Picture != 1 || s.Room.Message != 1 || s.Room.Index != 0 {
		t.Errorf("room %+v, want picture 1, message 1, record 0", *s.Room)
	}
	play(t, s, "none:100,down:1")
	if s.Room != nil {
		t.Fatal("still in the room after down")
	}

	// Walk on to the trigger's cell: an enemy appears.
	seen := false
	for n := 0; n < 600 && !seen; n++ {
		s.Step(input.Right)
		for k := 1; k <= 3; k++ {
			seen = seen || s.Engine.Slots[k].Actor != nil
		}
	}
	if !seen {
		t.Fatal("the enemy trigger did not bring an enemy")
	}
}
