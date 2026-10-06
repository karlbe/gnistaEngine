package game

import "github.com/karlbe/gnistaEngine/pkg/game/world"

// The visible area in tiles: the view position is one tile (16 pixels) left of the visible
// left edge (see render.viewMargin); the play area is 320 x 128 pixels.
const (
	viewLeftTiles = 1
	viewTilesW    = 20
	viewTilesH    = 8
)

// Door is a room door that has been opened, at the tile where the player stood.
type Door struct {
	Room  int
	X, Y  int
	Blown bool // opened with a charge rather than entered
}

// MapMemory is what the player has seen of the level. Not in the original: it feeds the
// minimap and does not affect the simulation.
type MapMemory struct {
	W, H  int
	Seen  []bool // per tile, row-major
	Doors []Door
}

func newMapMemory(l *world.Level) MapMemory {
	w, h := l.WidthTiles(), l.HeightTiles()
	return MapMemory{W: w, H: h, Seen: make([]bool, w*h)}
}

// Discovered reports whether tile (tx, ty) has been on screen.
func (m *MapMemory) Discovered(tx, ty int) bool {
	return tx >= 0 && ty >= 0 && tx < m.W && ty < m.H && m.Seen[ty*m.W+tx]
}

// reveal marks the tiles of a w x h rectangle at (tx, ty) as seen.
func (m *MapMemory) reveal(tx, ty, w, h int) {
	for y := max(ty, 0); y < min(ty+h, m.H); y++ {
		for x := max(tx, 0); x < min(tx+w, m.W); x++ {
			m.Seen[y*m.W+x] = true
		}
	}
}

// open records a room's door once; the first position and kind stay.
func (m *MapMemory) open(room, x, y int, blown bool) {
	for _, d := range m.Doors {
		if d.Room == room {
			return
		}
	}
	m.Doors = append(m.Doors, Door{Room: room, X: x, Y: y, Blown: blown})
}

// PlayerTile is the tile the player stands on, in level tile coordinates.
func (s *State) PlayerTile() (int, int) {
	vm := s.Engine
	p := vm.Slots[0]
	return floorDiv(int(vm.G.ViewX), world.TileSize) + p.TileX, floorDiv(int(vm.G.ViewY), world.TileSize) + p.TileY
}

// discover marks what the view shows.
func (s *State) discover() {
	vm := s.Engine
	s.Map.reveal(floorDiv(int(vm.G.ViewX), world.TileSize)+viewLeftTiles, floorDiv(int(vm.G.ViewY), world.TileSize), viewTilesW, viewTilesH)
}

// noteDoor records the door at the player's tile as opened.
func (s *State) noteDoor(room int, blown bool) {
	x, y := s.PlayerTile()
	s.Map.open(room, x, y, blown)
}

func floorDiv(a, b int) int {
	if a < 0 {
		return -((-a + b - 1) / b)
	}
	return a / b
}
