package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"

	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/input"
)

// A save state is the whole simulation state as plain data. It holds no assets, only what
// the game has changed; the level and the program come from Content when it is loaded.
// Saves made by one build of the game load in another only if saveVersion matches.
const saveVersion = 2

const saveMagic = "GGISAVE"

// Actor references in a saved slot: which of the actors the slot points at.
const (
	refNone   = -1
	refPlayer = 0
	refCabin  = 1
	refEnemy0 = 2 // refEnemy0 + k for enemy k (0-2)
	numActors = refEnemy0 + 3
)

// savedSlot is a slot without its actor pointer, plus which actor it pointed to.
type savedSlot struct {
	Slot  script.Slot
	Actor int
}

// saved is what gets encoded.
type saved struct {
	Version int
	Tick    uint64
	Last    input.Actions
	G       script.Globals
	Slots   [script.NumSlots]savedSlot
	Actors  [numActors]script.Actor
	Roll    int

	Spawner      Spawner
	Clock        Clock
	Invulnerable bool
	Fast         bool
	Paused       bool
	IgnoreDown   bool
	Rooms        [][roomRecLen]uint8
	Room         *Visit
	Ending       *Ending
	Stamps       []script.Sprite
	Map          MapMemory
	LevelCols    int // checked on load: the save must be for the same level
	LevelRows    int
}

// actors returns the actors that slots can point to, in the order of the ref constants.
func (s *State) actors() [numActors]*script.Actor {
	var a [numActors]*script.Actor
	a[refPlayer] = s.Engine.Slots[0].Actor
	a[refCabin] = &s.env.cabin
	for k := range s.env.enemies {
		a[refEnemy0+k] = &s.env.enemies[k]
	}
	return a
}

// snapshot copies the state into plain data. The result shares nothing with the state.
func (s *State) snapshot() (*saved, error) {
	d := &saved{
		Version: saveVersion, Tick: s.Tick, Last: s.Last, G: s.Engine.G, Roll: s.env.roll,
		Spawner: s.Spawner, Clock: s.Clock, Invulnerable: s.Invulnerable, Fast: s.Fast, Paused: s.Paused, IgnoreDown: s.IgnoreDown,
		Rooms: append([][roomRecLen]uint8(nil), s.Rooms...), Stamps: append([]script.Sprite(nil), s.Stamps...), Map: s.Map,
		LevelCols: s.content.Level.WidthTiles(), LevelRows: s.content.Level.HeightTiles(),
	}
	d.Spawner.Cells = append([]uint8(nil), s.Spawner.Cells...)
	d.Map.Seen = append([]bool(nil), s.Map.Seen...)
	d.Map.Doors = append([]Door(nil), s.Map.Doors...)
	if s.Room != nil {
		v := *s.Room
		d.Room = &v
	}
	if s.Ending != nil {
		e := *s.Ending
		e.Steps = append([]EndingStep(nil), s.Ending.Steps...)
		d.Ending = &e
	}
	actors := s.actors()
	for i := range actors {
		if actors[i] != nil {
			d.Actors[i] = *actors[i]
		}
	}
	for i, sl := range s.Engine.Slots {
		ref := refNone
		if sl.Actor != nil {
			ref = -2
			for j, a := range actors {
				if a == sl.Actor {
					ref = j
				}
			}
			if ref == -2 {
				return nil, fmt.Errorf("slot %d points at an actor the save state does not know", i)
			}
		}
		sl.Actor = nil
		d.Slots[i] = savedSlot{Slot: sl, Actor: ref}
	}
	return d, nil
}

// Clone returns an independent copy of the state, for searching ahead.
func (s *State) Clone() *State {
	d, err := s.snapshot()
	if err != nil {
		panic(err)
	}
	content := s.content
	content.Improvements = s.Engine.Improve // a clone plays by the same rules
	c, err := restore(content, d)
	if err != nil {
		panic(err)
	}
	return c
}

// Save encodes the state. The result is compressed and can be loaded with LoadState.
func (s *State) Save() ([]byte, error) {
	d, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(saveMagic)
	zw := gzip.NewWriter(&buf)
	if err := gob.NewEncoder(zw).Encode(d); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// LoadState makes a state from data written by Save, for the same Content.
func LoadState(c Content, data []byte) (*State, error) {
	if len(data) < len(saveMagic) || string(data[:len(saveMagic)]) != saveMagic {
		return nil, fmt.Errorf("not a save state")
	}
	zr, err := gzip.NewReader(bytes.NewReader(data[len(saveMagic):]))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	var d saved
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&d); err != nil {
		return nil, err
	}
	return restore(c, &d)
}

// restore makes a state from a snapshot, for the same Content.
func restore(c Content, d *saved) (*State, error) {
	if d.Version != saveVersion {
		return nil, fmt.Errorf("save state version %d, this build reads %d", d.Version, saveVersion)
	}
	if d.LevelCols != c.Level.WidthTiles() || d.LevelRows != c.Level.HeightTiles() {
		return nil, fmt.Errorf("the save state is for a %dx%d level, not %dx%d",
			d.LevelCols, d.LevelRows, c.Level.WidthTiles(), c.Level.HeightTiles())
	}
	// New sets up the parts that are not saved (the environment, the callbacks); then the
	// saved state replaces everything else.
	s := New(c, 0, 0)
	actors := s.actors()
	for i, a := range actors {
		*a = d.Actors[i]
	}
	for i, ss := range d.Slots {
		sl := ss.Slot
		switch {
		case ss.Actor == refNone:
			sl.Actor = nil
		case ss.Actor >= 0 && ss.Actor < numActors:
			sl.Actor = actors[ss.Actor]
		default:
			return nil, fmt.Errorf("slot %d refers to actor %d", i, ss.Actor)
		}
		s.Engine.Slots[i] = sl
	}
	s.Engine.G = d.G
	s.env.roll = d.Roll
	s.Tick, s.Last = d.Tick, d.Last
	s.Spawner, s.Clock = d.Spawner, d.Clock
	s.Invulnerable, s.Fast, s.Paused, s.IgnoreDown = d.Invulnerable, d.Fast, d.Paused, d.IgnoreDown
	s.Rooms, s.Room, s.Ending, s.Stamps, s.Map = d.Rooms, d.Room, d.Ending, d.Stamps, d.Map
	return s, nil
}
