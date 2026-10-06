package solve

import "github.com/karlbe/gnistaEngine/pkg/game/world"

func floorDiv(a, b int) int {
	if a < 0 {
		return -((-a + b - 1) / b)
	}
	return a / b
}

// DoorTile is where a room's door is: the tile the player stands on to use it.
type DoorTile struct{ X, Y int }

// RoomDoors maps every room to its door tile. The room behind a door is looked up by the
// level matrix cell of the view when the player stands there: the view starts 10 tiles left
// of and 6 above the player's tile, a block is 20 x 4 tiles, and the cell index of the start
// position is the first row's offset ($4EE starts at 662 = one row below the block row).
func RoomDoors(b *Bot) map[int]DoorTile {
	level := b.S.Level()
	out := map[int]DoorTile{}
	for ty := 0; ty < level.HeightTiles(); ty++ {
		for tx := 0; tx < level.WidthTiles(); tx++ {
			if id := b.S.Engine.Env.Tile(tx, ty); id < 0x26 || id > 0x28 {
				continue
			}
			out[int(level.T.Room(world.CellOf(tx, ty, level.Cols)))] = DoorTile{tx, ty}
		}
	}
	return out
}
