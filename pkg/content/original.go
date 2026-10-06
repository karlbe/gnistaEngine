package content

import (
	"encoding/binary"

	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
)

// Where the original keeps the level's tables in its code hunk (hunk-relative). This is the
// only place that knows; everything else reads world.Tables, which a pack fills from its own
// map file.
const (
	addrTriggers  = 0xA738 // spawner flags per matrix cell (the end marker is $FF)
	addrBlockAttr = 0xA628 // flags per block id
	addrRoomIndex = 0xA9F4 // room record number per matrix cell
	addrRooms     = 0xACAC // room records, ended by $FFFF
)

// OriginalTables reads the level's tables out of the original code hunk. The trigger table
// is read for the whole matrix, as the original's copy of it is; it runs on into the room
// index.
func OriginalTables(code []byte, e *Extracted) world.Tables {
	cells := e.LevelCols * e.LevelRows
	t := world.Tables{
		Triggers:   append([]uint8(nil), code[addrTriggers:addrTriggers+cells]...),
		RoomIndex:  append([]uint8(nil), code[addrRoomIndex:addrRoomIndex+cells]...),
		StartViewX: e.StartX, StartViewY: e.StartY,
	}
	copy(t.BlockAttr[:], code[addrBlockAttr:addrBlockAttr+256])
	for a := addrRooms; uint16(code[a])<<8|uint16(code[a+1]) != 0xFFFF; a += world.RoomRecLen {
		var r [world.RoomRecLen]uint8
		copy(r[:], code[a:])
		t.Rooms = append(t.Rooms, r)
	}
	// Lifts need the card of their third of the level: block columns 0-2, 3-5 and 6-8 ($16D8).
	t.LiftCards = make([]uint8, e.LevelCols)
	for c := 0; c < 9 && c < len(t.LiftCards); c++ {
		t.LiftCards[c] = 1 << (c / 3)
	}
	return t
}

// Where the original keeps the program's entry points and tables in its code hunk.
const (
	addrAmmo       = 0x4F6  // 4 bytes per weapon; byte 2 is the magazine size
	addrWaveOrder  = 0xC02  // which wave template comes next, ended by $FF
	addrTmplFirstR = 0x5EF8 // enemy templates, 24 bytes each: six script addresses
	addrTmplFirstL = 0x5EC8
	addrTmplWaveR  = 0x5E80
	addrTmplWaveL  = 0x5E38
)

// OriginalSpec builds the program's Spec from the original code hunk: the scripts the engine
// starts by itself and the tables the game reads.
func OriginalSpec(code []byte) script.Spec {
	long := func(a int) int { return int(binary.BigEndian.Uint32(code[a:])) }
	tmpl := func(a int) script.Template {
		t := script.Template{PC: long(a)}
		for i := range t.Scripts {
			t.Scripts[i] = long(a + 4*(i+1))
		}
		return t
	}
	sp := script.Spec{
		Player:      0x2E6A,
		Idle:        [2][3]int{{0x2E6A, 0x30F8, 0x33FC}, {0x2EE2, 0x3170, 0x3474}},
		LiftCall:    0x455E,
		LiftRecall:  0x46A6,
		Cabin:       [4]int{0x4943, 0x49B2, 0x4A21, 0x4A92},
		LiftHelper:  [2][2]int{{0x4B03, 0x46A9}, {0x4B62, 0x47F6}},
		LiftExitEnd: 0x455D, LiftExitFrames: 0x4556,
		Charge: 0x4E19, BlownUp: 0x4E5A,
		StepSounds: 0x20C2, StairSounds: 0x2114,
		ContactRight: 0x4E5A, ContactLeft: 0x4EC4,
		PlayerFrames: [2]uint16{0x14D, 0x14E},
	}
	for w := range sp.MagSize {
		sp.MagSize[w] = code[addrAmmo+4*w+2]
	}
	for i := 0; i < 2; i++ {
		sp.FirstRight[i], sp.FirstLeft[i] = tmpl(addrTmplFirstR+24*i), tmpl(addrTmplFirstL+24*i)
	}
	for i := 0; i < 3; i++ {
		sp.WaveRight[i], sp.WaveLeft[i] = tmpl(addrTmplWaveR+24*i), tmpl(addrTmplWaveL+24*i)
	}
	for a := addrWaveOrder; code[a] != 0xFF; a++ {
		sp.WaveOrder = append(sp.WaveOrder, code[a])
	}
	return sp
}
