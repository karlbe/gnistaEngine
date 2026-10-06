package game

import (
	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
	"github.com/karlbe/gnistaEngine/pkg/input"
)

// Original addresses for the rooms behind doors ($23CC).
const (
	roomRecLen = world.RoomRecLen
)

// A room record: the cards that open the door, what the room gives, its picture and
// message. The game changes Flags (visited) and Blown in place.
const (
	recLock    = 0 // cards ($505 bits), any of which opens the door
	recFlags   = 1 // bits 0-4: a card, 5: visited, 6: weapon 2, 7: weapon 3
	recMags    = 2 // +2..+4: magazines for weapons 1-3
	recCharges = 5
	recPicture = 6 // index into the room pictures ($842C)
	recMessage = 7 // index into the room messages ($AFA2)
	recBlown   = 8 // set by op74 when a charge has blown the door
)

// Room pictures with a meaning of their own.
const (
	pictureEmpty     = 0 // shown with message 0 when a room has been visited
	pictureFirstAid  = 5 // restores the hits; never marked as visited
	pictureBombRoom  = 8 // the bomb: fire leaves, Space starts cutting wires
	pictureBomb      = 9 // close-up of the bomb's wires (BB)
	messageEmpty     = 0
	roomMessageDelay = 38 // frames: the busy loop at $2432 (200000 iterations, scaled as measured for the ending)
)

// Wire cutting ($2864-$29EA): a cursor on one of four wires, moved or used once a second.
const (
	wireCount   = 4
	wireGood    = 3 // the fourth wire defuses the bomb
	wirePoll    = 50
	CursorFrame = 0x288 // bob for the wire cursor
	CursorY     = 0x5B  // view-relative cursor position
)

// WireX are the cursor's view-relative x positions over the four wires.
var WireX = [wireCount]int16{0x92, 0xA5, 0xC0, 0xD7}

// Visit is a room on screen. The game world and the clock stand still meanwhile: the
// original runs the whole visit inside the player's control opcode.
type Visit struct {
	Index   int // the room record
	Picture int
	Message int
	Wait    int  // frames until the message is shown
	Bomb    bool // the bomb room
	Wires   bool // cutting wires
	Cursor  int  // the wire under the cursor
	Poll    int  // frames until the joystick is read again
}

// Showing reports whether the message has appeared.
func (v *Visit) Showing() bool { return v.Wait == 0 }

func loadRooms(s *State) {
	s.Rooms = append(s.Rooms, s.content.Level.T.Rooms...)
}

// enterRoom is $23CC: up at a door. A visited room shows as empty; a locked door needs one
// of its cards or to have been blown open; otherwise the room's picture and message are
// shown and its contents taken.
func (s *State) enterRoom() {
	g := &s.Engine.G
	n := int(s.content.Level.T.Room(g.Cell))
	if n >= len(s.Rooms) {
		return
	}
	r := &s.Rooms[n]
	switch {
	case r[recFlags]&0x20 != 0:
		s.noteDoor(n, false)
		s.Room = &Visit{Index: n, Picture: pictureEmpty, Message: messageEmpty}
		return
	case r[recBlown] == 0 && r[recLock] != 0 && g.Cards&r[recLock] == 0:
		return
	}
	s.noteDoor(n, false)
	s.Room = &Visit{Index: n, Picture: int(r[recPicture]), Message: int(r[recMessage]), Wait: roomMessageDelay}
	switch r[recPicture] {
	case pictureBombRoom:
		s.Room.Bomb = true
		return
	case pictureFirstAid:
		g.Lives = startLives
		g.HUDDirty |= 4
	default:
		r[recFlags] |= 0x20
	}
	switch f := r[recFlags]; {
	case f&0x40 != 0:
		g.WeaponsOwned |= 2
	case f&0x80 != 0:
		g.WeaponsOwned |= 4
	}
	for k := 0; k < 5; k++ { // at most one card
		if r[recFlags]&(1<<k) != 0 {
			g.Cards |= 1 << k
			break
		}
	}
	for w := 0; w < 3; w++ {
		for n := r[recMags+w]; n > 0 && g.Ammo[w].Mags != 9; n-- {
			g.Ammo[w].Mags++
		}
	}
	for n := r[recCharges]; n > 0 && g.Charges != 9; n-- {
		g.Charges++
	}
}

// stepRoom advances a room visit by one frame.
func (s *State) stepRoom(in input.Actions) {
	v := s.Room
	if v.Wait > 0 {
		v.Wait--
		return
	}
	switch {
	case v.Wires:
		if v.Poll > 0 {
			v.Poll--
			return
		}
		v.Poll = wirePoll
		switch j := joystick(in); {
		case j&script.JoyFire != 0:
			s.Engine.G.Outcome = 2
			if v.Cursor == wireGood {
				s.Engine.G.Outcome = 3
			}
			s.Room = nil
		case j&script.JoyRight != 0 && v.Cursor < wireCount-1:
			v.Cursor++
		case j&script.JoyLeft != 0 && v.Cursor > 0:
			v.Cursor--
		}
	case v.Bomb:
		switch {
		case in.Has(input.Charge):
			v.Picture, v.Wires, v.Poll = pictureBomb, true, wirePoll
		case in.Has(input.Fire):
			s.Room = nil
		}
	case in.Has(input.Down):
		// The original goes straight back to the actor script, which would read the down
		// still held as a duck. Not in the original: wait until it is released.
		s.Room, s.IgnoreDown = nil, s.Engine.Improve
	}
}

// DoorInfo describes the door the player stands at.
type DoorInfo struct {
	Door    bool // the player stands on a door tile
	Room    int  // the room record behind it
	Locked  bool // closed to the player: it needs a card the player lacks, and has not been blown open
	Visited bool
	Bomb    bool // the room with the bomb
}

// DoorHere looks up the door at the player's tile, the way $23CC does. A query for tools and
// tests; it changes nothing.
func (s *State) DoorHere() DoorInfo {
	t := s.Engine.Slots[0].Sensors[script.SensHere]
	if t < 0x26 || t > 0x28 {
		return DoorInfo{}
	}
	n := int(s.content.Level.T.Room(s.Engine.G.Cell))
	if n >= len(s.Rooms) {
		return DoorInfo{Door: true, Room: n}
	}
	rec := &s.Rooms[n]
	return DoorInfo{
		Door: true, Room: n,
		Locked:  rec[recBlown] == 0 && rec[recLock] != 0 && s.Engine.G.Cards&rec[recLock] == 0,
		Visited: rec[recFlags]&0x20 != 0,
		Bomb:    rec[recPicture] == pictureBombRoom,
	}
}

// BlownDoors is the number of doors that charges have blown open. A query for tools.
func (s *State) BlownDoors() int {
	n := 0
	for i := range s.Rooms {
		if s.Rooms[i][recBlown] != 0 {
			n++
		}
	}
	return n
}

// doorBlown is the end of op74: the room record for the player's cell is opened.
func (s *State) doorBlown() {
	if n := int(s.content.Level.T.Room(s.Engine.G.Cell)); n < len(s.Rooms) {
		s.Rooms[n][recBlown] |= 1
		s.noteDoor(n, true)
	}
}

// VisitedRooms is the number of rooms the player has been in. A query for tools and tests.
func (s *State) VisitedRooms() int {
	n := 0
	for i := range s.Rooms {
		if s.Rooms[i][recFlags]&0x20 != 0 {
			n++
		}
	}
	return n
}
