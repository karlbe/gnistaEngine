package world

// RoomRecLen is the size of a room record.
const RoomRecLen = 12

// Tables is the level data that is not tile pictures: where enemies are triggered, what is
// behind each door, and where the player starts. The original keeps these as tables in its
// code (pkg/content/original.go reads them); a pack (pkg/pack) builds them from its
// map file. They are static: the game copies what it changes.
type Tables struct {
	// Triggers holds one byte per matrix cell for the enemy spawner: bit 0 marks a trigger,
	// bits 2-6 choose what follows the first enemy (see pkg/game/enemies.go).
	Triggers []uint8
	// RoomIndex is the room record number behind the door in each matrix cell.
	RoomIndex []uint8
	// Rooms are the room records: lock, flags, magazines, charges, picture, message.
	Rooms [][RoomRecLen]uint8
	// BlockAttr is one byte per block id; bit 0 lets wave enemies appear to the left of it.
	BlockAttr [256]uint8
	// LiftCards is the card bits needed to call a lift, per block column (0: none).
	LiftCards []uint8
	// StartViewX and StartViewY are the first view position in pixels. The player stands
	// 10 tiles right of and 6 tiles below the view's corner.
	StartViewX, StartViewY int
}

// Trigger returns the spawner flags of cell i, or 0 outside the table.
func (t *Tables) Trigger(i int) uint8 {
	if i < 0 || i >= len(t.Triggers) {
		return 0
	}
	return t.Triggers[i]
}

// Room returns the room record number of cell i, or 0 outside the table.
func (t *Tables) Room(i int) uint8 {
	if i < 0 || i >= len(t.RoomIndex) {
		return 0
	}
	return t.RoomIndex[i]
}

// LiftCard returns the card bits needed in block column c.
func (t *Tables) LiftCard(c int) uint8 {
	if c < 0 || c >= len(t.LiftCards) {
		return 0
	}
	return t.LiftCards[c]
}

// StartCell is the matrix cell of the start view: the cell index the original calls $4EE.
// The view's block row is one above the cell's, because a view is 8 tiles tall and a block 4.
func (t *Tables) StartCell(cols int) int {
	return (t.StartViewY/TileSize/BlockH+1)*cols + t.StartViewX/TileSize/BlockW
}

// CellOf returns the matrix cell that tile (tx, ty) belongs to when the player stands there:
// the view starts 10 tiles to the left of and 6 above the player's tile.
func CellOf(tx, ty, cols int) int {
	return (floorDiv(ty-6, BlockH)+1)*cols + floorDiv(tx-10, BlockW)
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
