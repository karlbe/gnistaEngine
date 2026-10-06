package pack

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/karlbe/gnistaEngine/pkg/game/world"
)

// The map is a Tiled map (https://www.mapeditor.org/) saved as JSON (level.tmj). Tile layer
// "tiles" holds the tiles; the object layer "objects" holds what the game needs to know
// about places. docs/packs.md describes the objects; in short:
//
//	start      where the player begins
//	door       a room behind a door tile: lock, card, weapon, mags1-3, charges, picture, message
//	spawn      an enemy trigger: wave 0 (one enemy) or 1-4 (that many more follow)
//	wave_zone  a rectangle; waves may appear in front of the player in blocks inside it
//
// Map property lift_cards lists the cards a lift needs per block column ("1,1,1,2,2,2,4").

type tiledMap struct {
	Width, Height         int
	TileWidth, TileHeight int
	Layers                []tiledLayer
	Tilesets              []struct{ FirstGID int }
	Properties            []tiledProp
}

type tiledLayer struct {
	Name       string
	Type       string
	Data       json.RawMessage
	Objects    []tiledObject
	Properties []tiledProp
}

type tiledObject struct {
	ID            int
	Name          string
	Type          string
	Class         string
	X, Y          float64
	Width, Height float64
	Properties    []tiledProp
}

type tiledProp struct {
	Name  string
	Value any
}

func (o *tiledObject) kind() string {
	if o.Class != "" {
		return o.Class
	}
	return o.Type
}

// prop finds a property by name.
func prop(ps []tiledProp, name string) (any, bool) {
	for _, p := range ps {
		if p.Name == name {
			return p.Value, true
		}
	}
	return nil, false
}

// number reads a numeric property; a missing one is def.
func number(ps []tiledProp, name string, def int) (int, error) {
	v, ok := prop(ps, name)
	if !ok {
		return def, nil
	}
	switch x := v.(type) {
	case float64:
		return int(x), nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0, fmt.Errorf("property %s: %q is not a number", name, x)
		}
		return n, nil
	}
	return 0, fmt.Errorf("property %s has an unusable value", name)
}

// cards reads a card list property: a bit mask (number) or card numbers "1,3" (1-5).
func cards(ps []tiledProp, name string) (uint8, error) {
	v, ok := prop(ps, name)
	if !ok {
		return 0, nil
	}
	switch x := v.(type) {
	case float64:
		return uint8(x), nil
	case string:
		var m uint8
		for _, f := range strings.FieldsFunc(x, func(r rune) bool { return r == ',' || r == ' ' }) {
			n, err := strconv.Atoi(f)
			if err != nil || n < 1 || n > 5 {
				return 0, fmt.Errorf("property %s: card %q is not 1-5", name, f)
			}
			m |= 1 << (n - 1)
		}
		return m, nil
	}
	return 0, fmt.Errorf("property %s has an unusable value", name)
}

// Level builds the level from level.tmj. It returns nil if the pack has none.
func (p *Pack) Level() (*world.Level, error) {
	b, err := os.ReadFile(p.path("level.tmj"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m tiledMap
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("level.tmj: %w", err)
	}
	lv, err := p.buildLevel(&m)
	if err != nil {
		return nil, fmt.Errorf("level.tmj: %w", err)
	}
	return lv, nil
}

func (p *Pack) buildLevel(m *tiledMap) (*world.Level, error) {
	if m.TileWidth != tileSize || m.TileHeight != tileSize {
		return nil, fmt.Errorf("tiles are %dx%d, the game's are %dx%d", m.TileWidth, m.TileHeight, tileSize, tileSize)
	}
	first := 1
	if len(m.Tilesets) > 0 {
		first = m.Tilesets[0].FirstGID
	}
	empty := 0x73
	if p.Manifest.EmptyTile != nil {
		empty = *p.Manifest.EmptyTile
	}
	if empty < 0 || empty > 255 {
		return nil, fmt.Errorf("empty_tile %d is not a tile id", empty)
	}

	var tiles *tiledLayer
	var objs []tiledObject
	for i := range m.Layers {
		l := &m.Layers[i]
		switch l.Type {
		case "tilelayer":
			if tiles == nil || l.Name == "tiles" {
				tiles = l
			}
		case "objectgroup":
			objs = append(objs, l.Objects...)
		}
	}
	if tiles == nil {
		return nil, fmt.Errorf("no tile layer (name it \"tiles\")")
	}
	var data []uint32
	if err := json.Unmarshal(tiles.Data, &data); err != nil {
		return nil, fmt.Errorf("layer %q: set the tile layer format to CSV in the map properties (%v)", tiles.Name, err)
	}
	if len(data) != m.Width*m.Height {
		return nil, fmt.Errorf("layer %q has %d cells, the map is %dx%d", tiles.Name, len(data), m.Width, m.Height)
	}

	cols := (m.Width + world.BlockW - 1) / world.BlockW
	rows := (m.Height + world.BlockH - 1) / world.BlockH
	w, h := cols*world.BlockW, rows*world.BlockH
	grid := make([]uint8, w*h)
	for i := range grid {
		grid[i] = uint8(empty)
	}
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			gid := data[y*m.Width+x] &^ 0xF0000000 // the flip flags
			if gid == 0 {
				continue
			}
			id := int(gid) - first
			if id < 0 || id > 255 {
				return nil, fmt.Errorf("tile (%d,%d) is %d: tile ids go from 0 to 255", x, y, id)
			}
			grid[y*w+x] = uint8(id)
		}
	}

	t := world.Tables{
		Triggers:  make([]uint8, cols*rows),
		RoomIndex: make([]uint8, cols*rows),
	}
	tileOf := func(o *tiledObject) (int, int) {
		return int(math.Floor(o.X / tileSize)), int(math.Floor(o.Y / tileSize))
	}
	wave := map[[2]int]bool{} // blocks inside a wave zone
	doorCell := map[int]bool{}
	haveStart := false
	for i := range objs {
		o := &objs[i]
		tx, ty := tileOf(o)
		where := fmt.Sprintf("%s %d at tile (%d,%d)", o.kind(), o.ID, tx, ty)
		cell := world.CellOf(tx, ty, cols)
		switch o.kind() {
		case "start", "door", "spawn":
			if tx < 0 || ty < 0 || tx >= w || ty >= h {
				return nil, fmt.Errorf("%s: outside the map", where)
			}
		}
		switch o.kind() {
		case "start":
			if tx < 10 || ty < 6 {
				return nil, fmt.Errorf("%s: the player needs 10 tiles to the left of and 6 above the start (the view's size)", where)
			}
			t.StartViewX, t.StartViewY = (tx-10)*tileSize, (ty-6)*tileSize
			if grid[ty*w+tx] < 11 {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s is inside a wall (tile ids below 11 are solid)", where))
			}
			haveStart = true
		case "door":
			if cell < 0 || cell >= len(t.RoomIndex) {
				return nil, fmt.Errorf("%s: outside the map", where)
			}
			if doorCell[cell] {
				return nil, fmt.Errorf("%s: another door is in the same cell; a cell holds one door", where)
			}
			rec, err := roomRecord(o.Properties)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", where, err)
			}
			if len(t.Rooms) >= 255 {
				return nil, fmt.Errorf("more than 255 doors")
			}
			if id := grid[ty*w+tx]; id < 0x26 || id > 0x28 {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s is not on a door tile (0x26-0x28): the game never opens it", where))
			}
			doorCell[cell] = true
			t.Rooms = append(t.Rooms, rec)
			t.RoomIndex[cell] = uint8(len(t.Rooms) - 1)
		case "spawn":
			if cell < 0 || cell >= len(t.Triggers) {
				return nil, fmt.Errorf("%s: outside the map", where)
			}
			n, err := number(o.Properties, "wave", 0)
			if err != nil || n < 0 || n > 4 {
				return nil, fmt.Errorf("%s: wave must be 0-4", where)
			}
			flags := uint8(1)
			if n == 0 {
				flags |= 1 << 6
			} else {
				flags |= 1 << (n + 1)
			}
			t.Triggers[cell] |= flags
		case "wave_zone":
			for by := int(o.Y) / tileSize / world.BlockH; by <= int(o.Y+o.Height-1)/tileSize/world.BlockH && by < rows; by++ {
				for bx := int(o.X) / tileSize / world.BlockW; bx <= int(o.X+o.Width-1)/tileSize/world.BlockW && bx < cols; bx++ {
					wave[[2]int{bx, by}] = true
				}
			}
		}
	}
	if !haveStart {
		return nil, fmt.Errorf("no start object: place a point object of class \"start\" where the player begins")
	}
	// The block library: identical blocks (with identical attributes) share an entry.
	lib := map[string]uint8{}
	var blocks []uint8
	cells := make([]uint8, cols*rows)
	for by := 0; by < rows; by++ {
		for bx := 0; bx < cols; bx++ {
			blk := make([]uint8, 0, world.BlockW*world.BlockH)
			for y := 0; y < world.BlockH; y++ {
				blk = append(blk, grid[(by*world.BlockH+y)*w+bx*world.BlockW:][:world.BlockW]...)
			}
			attr := uint8(0)
			if wave[[2]int{bx, by}] {
				attr = 1
			}
			key := string(blk) + string(rune(attr))
			id, ok := lib[key]
			if !ok {
				if len(lib) == 256 {
					return nil, fmt.Errorf("more than 256 different blocks: a block is %dx%d tiles, and the map may use 256 different ones; repeat more", world.BlockW, world.BlockH)
				}
				id = uint8(len(lib))
				lib[key] = id
				blocks = append(blocks, blk...)
				t.BlockAttr[id] = attr
			}
			cells[by*cols+bx] = id
		}
	}

	if v, ok := prop(m.Properties, "lift_cards"); ok {
		s, _ := v.(string)
		t.LiftCards = make([]uint8, cols)
		for i, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
			n, err := strconv.Atoi(f)
			if err != nil || i >= cols {
				return nil, fmt.Errorf("lift_cards %q: one number per block column (%d columns)", s, cols)
			}
			t.LiftCards[i] = uint8(n)
		}
	}
	lv, err := world.NewLevel(cols, rows, cells, blocks)
	if err != nil {
		return nil, err
	}
	lv.T = t
	return lv, nil
}

// roomRecord builds a room record from a door object's properties.
func roomRecord(ps []tiledProp) ([world.RoomRecLen]uint8, error) {
	var r [world.RoomRecLen]uint8
	var err error
	if r[0], err = cards(ps, "lock"); err != nil {
		return r, err
	}
	card, err := cards(ps, "card")
	if err != nil {
		return r, err
	}
	r[1] = card
	if w, _ := number(ps, "weapon", 0); w == 2 {
		r[1] |= 0x40
	} else if w == 3 {
		r[1] |= 0x80
	}
	for i := 0; i < 3; i++ {
		n, err := number(ps, fmt.Sprintf("mags%d", i+1), 0)
		if err != nil || n < 0 || n > 9 {
			return r, fmt.Errorf("mags%d must be 0-9", i+1)
		}
		r[2+i] = uint8(n)
	}
	for i, name := range []string{"charges", "picture", "message"} {
		n, err := number(ps, name, 0)
		if err != nil || n < 0 || n > 255 {
			return r, fmt.Errorf("%s must be 0-255", name)
		}
		r[5+i] = uint8(n)
	}
	return r, nil
}
