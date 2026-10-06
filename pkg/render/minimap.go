package render

import (
	"image/color"

	"github.com/karlbe/gnistaEngine/pkg/game"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
)

// The minimap is not in the original. It shows the level around the player, 2x2 pixels per
// tile, in the strip under the panel or in the top right corner of the play area. It is
// symbolic, not a scaled-down picture: walls, floors, stairs, ladders, doors and lifts each
// have their own colour and shape, and everything else is left empty. Tiles the player has
// not seen are left out (fog of war); opened room doors get a marker.
const (
	cornerW    = 96
	cornerH    = 40
	miniMargin = 3
	minBottomH = 16 // least free height under the panel for the bottom placement
)

var (
	miniFog     = color.RGBA{0x06, 0x08, 0x10, 0xFF}
	miniEmpty   = color.RGBA{0x16, 0x1A, 0x2C, 0xFF}
	miniBorder  = color.RGBA{0xC0, 0xC0, 0xC0, 0xFF}
	miniOutline = color.RGBA{0, 0, 0, 0xFF}
	miniVisited = color.RGBA{0x30, 0xFF, 0x40, 0xFF} // a door that was entered
	miniBlown   = color.RGBA{0xFF, 0x50, 0xC0, 0xFF} // a door that was blown open
	miniPlayerA = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	miniPlayerB = color.RGBA{0x40, 0xE0, 0xFF, 0xFF}

	miniWall   = color.RGBA{0x6A, 0x74, 0x90, 0xFF}
	miniFloor  = color.RGBA{0x48, 0x4E, 0x62, 0xFF}
	miniStairs = color.RGBA{0xF0, 0xF0, 0xF0, 0xFF}
	miniLadder = color.RGBA{0xD0, 0xA0, 0x50, 0xFF}
	miniDoor   = color.RGBA{0xFF, 0x8C, 0x20, 0xFF}
	miniLift   = color.RGBA{0xE0, 0x20, 0x20, 0xFF}
)

// tileKind is what a tile means to the minimap.
type tileKind uint8

const (
	kindNone tileKind = iota
	kindWall
	kindStairsUp   // rises to the right: "/"
	kindStairsDown // rises to the left: "\"
	kindLadder
	kindDoor
	kindLift
)

// classify sorts tile ids by the ranges the original's scripts test (op40/op41, op44):
// ids below 11 block movement (walls); $0B-$0F and $13-$15 are the stairs seen from the
// right, $16-$1A and $1B-$1D from the left; $21-$25 are ladders; $26-$28 doors (a charge
// goes on $26-$29); $29 and $2A are the lift, one tile per floor. $10-$12 are unused.
// The slope of each stair range was checked against the tile pictures.
func classify(id int) tileKind {
	switch {
	case id < 0x0B:
		return kindWall
	case id <= 0x0F, id >= 0x13 && id <= 0x15:
		return kindStairsUp
	case id >= 0x16 && id <= 0x1D:
		return kindStairsDown
	case id >= 0x21 && id <= 0x25:
		return kindLadder
	case id >= 0x26 && id <= 0x28:
		return kindDoor
	case id == 0x29 || id == 0x2A:
		return kindLift
	}
	return kindNone
}

// hasFloor reports whether a tile picture has a wide light grey band, which is how the
// floors and walkways are drawn ($13-$15, $1B-$20, $33-$3B, $43 and more).
func (r *Renderer) hasFloor(id int) bool {
	t := r.tile(id)
	if t == nil {
		return false
	}
	rows := 0
	for y := 0; y < t.H; y++ {
		n := 0
		for x := 0; x < t.W; x++ {
			c := r.a.Palette[t.At(x, y)]
			d := func(a, b uint8) int { return max(int(a), int(b)) - min(int(a), int(b)) }
			if d(c.R, c.G) < 24 && d(c.G, c.B) < 24 && c.R > 110 && c.R < 235 {
				n++
			}
		}
		if n >= t.W*7/8 {
			rows++
		}
	}
	return rows >= 7
}

// tileCells draws a tile as scale x scale cells, row-major.
func (r *Renderer) tileCells(id, scale int) []color.RGBA {
	key := [2]int{id, scale}
	if c, ok := r.miniCells[key]; ok {
		return c
	}
	cells := make([]color.RGBA, scale*scale)
	for i := range cells {
		cells[i] = miniEmpty
	}
	set := func(c color.RGBA, f func(x, y int) bool) {
		for y := 0; y < scale; y++ {
			for x := 0; x < scale; x++ {
				if f(x, y) {
					cells[y*scale+x] = c
				}
			}
		}
	}
	all := func(int, int) bool { return true }
	if r.hasFloor(id) {
		set(miniFloor, func(_, y int) bool { return y == 0 })
	}
	switch classify(id) {
	case kindWall:
		set(miniWall, all)
	case kindStairsUp:
		set(miniStairs, func(x, y int) bool { return x == scale-1-y })
	case kindStairsDown:
		set(miniStairs, func(x, y int) bool { return x == y })
	case kindLadder:
		set(miniLadder, func(x, _ int) bool { return x == scale/2 })
	case kindDoor:
		set(miniDoor, all)
	case kindLift:
		set(miniLift, all)
	}
	if r.miniCells == nil {
		r.miniCells = map[[2]int][]color.RGBA{}
	}
	r.miniCells[key] = cells
	return cells
}

// panelHeight is the number of lines the HUD panel uses. The NSIMenu picture is 200 lines
// tall, but everything below the panel's last drawn line is black and free.
func (r *Renderer) panelHeight() int {
	if r.panelH == 0 {
		img := r.a.HUD
		for y := img.H - 1; y >= 0 && r.panelH == 0; y-- {
			for x := 0; x < img.W; x++ {
				if img.At(x, y)&15 != 0 {
					r.panelH = y + 1
					break
				}
			}
		}
	}
	return r.panelH
}

// miniRect is the minimap's inner rectangle on the screen. At the bottom it fills the whole
// black strip under the panel; if that strip is too small it falls back to the corner.
func (r *Renderer) miniRect() (x0, y0, w, h, scale int) {
	if r.MinimapBottom && r.a.HUD != nil {
		if top := hudTop + r.panelHeight(); ScreenHeight-top >= minBottomH {
			return 0, top, ScreenWidth, ScreenHeight - top, 2
		}
	}
	return PlayWidth - cornerW - miniMargin, miniMargin, cornerW, cornerH, 2
}

func (r *Renderer) drawMinimap(s *game.State, level *world.Level) {
	m := &s.Map
	x0, y0, pw, ph, scale := r.miniRect()
	tw, th := pw/scale, ph/scale // size in tiles
	px, py := s.PlayerTile()
	ox := min(max(px-tw/2, 0), max(m.W-tw, 0))
	oy := min(max(py-th/2, 0), max(m.H-th, 0))
	if x0 > 0 { // the bottom placement fills the whole strip and needs no border
		for x := -1; x <= pw; x++ {
			r.px(x0+x, y0-1, miniBorder)
			r.px(x0+x, y0+ph, miniBorder)
		}
		for y := 0; y < ph; y++ {
			r.px(x0-1, y0+y, miniBorder)
			r.px(x0+pw, y0+y, miniBorder)
		}
	}
	r.fill(x0, y0, pw, ph, miniFog)
	for ty := 0; ty < th; ty++ {
		for tx := 0; tx < tw; tx++ {
			if !m.Discovered(ox+tx, oy+ty) {
				continue
			}
			cells := r.tileCells(level.Tile(ox+tx, oy+ty), scale)
			for cy := 0; cy < scale; cy++ {
				for cx := 0; cx < scale; cx++ {
					r.px(x0+tx*scale+cx, y0+ty*scale+cy, cells[cy*scale+cx])
				}
			}
		}
	}
	// A marker is the size of its tile with a one pixel outline.
	marker := func(tx, ty int, c color.RGBA) {
		if tx < ox-1 || ty < oy-1 || tx >= ox+tw+1 || ty >= oy+th+1 {
			return
		}
		bx, by := x0+(tx-ox)*scale, y0+(ty-oy)*scale
		for dy := -1; dy <= scale; dy++ {
			for dx := -1; dx <= scale; dx++ {
				if x, y := bx+dx, by+dy; x >= x0 && y >= y0 && x < x0+pw && y < y0+ph {
					if dx >= 0 && dx < scale && dy >= 0 && dy < scale {
						r.px(x, y, c)
					} else {
						r.px(x, y, miniOutline)
					}
				}
			}
		}
	}
	for _, d := range m.Doors {
		c := miniVisited
		if d.Blown {
			c = miniBlown
		}
		marker(d.X, d.Y, c)
	}
	c := miniPlayerA
	if s.Tick/12%2 == 1 {
		c = miniPlayerB
	}
	marker(px, py, c)
}

func (r *Renderer) px(x, y int, c color.RGBA) { r.fill(x, y, 1, 1, c) }
