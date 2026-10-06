package pack

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// StarterPalette is the palette of a new pack: 32 colours for the island, the sea and the
// limestone. It is our own.
var StarterPalette = []string{
	"#0b1020", "#1c2540", "#2e3a5c", "#46557f", "#6a7aa3", "#9aa8c8", "#d9e0ee", "#ffffff",
	"#3a2a1e", "#5b4230", "#8a6a48", "#b89868", "#d8c490", "#efe2b8", "#1f4d2c", "#2f7a42",
	"#66b050", "#a8d878", "#5a1414", "#a02828", "#e04040", "#ff8a70", "#7a4a00", "#d08a10",
	"#ffc820", "#fff080", "#0a4a6a", "#1e7aa0", "#4ab0d0", "#a0e0f0", "#6a2a7a", "#c070d0",
}

// Starter palette indices used by the placeholder tiles.
const (
	cInk       = 0
	cNight     = 1
	cSlate     = 2
	cSteel     = 3
	cSteelLt   = 5
	cWhite     = 7
	cBrown     = 9
	cBrownLt   = 10
	cLimestone = 11
	cLimeLt    = 12
	cGreen     = 15
	cRed       = 19
	cRedLt     = 20
	cGold      = 23
	cYellow    = 24
	cSea       = 27
	cSeaLt     = 28
)

// Tile ids of the starter's placeholder tiles. The ranges are what the game's scripts look
// for (docs/packs.md, "Tile ids").
const (
	TileWall       = 0    // ids 0-10 block movement
	TileWallLight  = 1    //
	TileStairsUp   = 0x0B // 0x0B-0x0F: stairs rising to the right, seen from the left
	TileStairsDown = 0x16 // 0x16-0x1A: stairs rising to the left
	TileLadder     = 0x21 // 0x21-0x25
	TileDoor       = 0x26 // 0x26-0x28
	TileLift       = 0x29 // 0x29 and 0x2A, one per floor
	TileFloor      = 0x30
	TileInterior   = 0x31
	TileSky        = 0x73
)

type canvas struct {
	img *image.RGBA
	pal []color.RGBA
}

// px sets a pixel at (x, y) in the tile at (cx, cy) of the sheet.
func (c *canvas) px(tile, x, y int, idx int) {
	c.img.SetRGBA(tile%16*16+x, tile/16*16+y, c.pal[idx])
}

func (c *canvas) rect(tile, x0, y0, w, h, idx int) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			c.px(tile, x, y, idx)
		}
	}
}

// brick draws limestone blocks.
func (c *canvas) brick(tile, base, mortar, shade int) {
	c.rect(tile, 0, 0, 16, 16, base)
	for row := 0; row < 4; row++ {
		y := row * 4
		c.rect(tile, 0, y, 16, 1, mortar)
		off := 0
		if row%2 == 1 {
			off = 4
		}
		for x := off; x < 16; x += 8 {
			c.rect(tile, x, y, 1, 4, mortar)
		}
		c.rect(tile, 0, y+3, 16, 1, shade)
	}
}

func (c *canvas) stairs(tile, step int, rising bool) {
	c.rect(tile, 0, 0, 16, 16, cNight)
	for x := 0; x < 16; x++ {
		h := (x + step*4) % 16
		if !rising {
			h = (15 - x + step*4) % 16
		}
		top := 15 - h
		c.rect(tile, x, top, 1, 16-top, cLimestone)
		c.px(tile, x, top, cLimeLt)
	}
}

func (c *canvas) ladder(tile int) {
	c.rect(tile, 0, 0, 16, 16, cNight)
	c.rect(tile, 3, 0, 2, 16, cBrownLt)
	c.rect(tile, 11, 0, 2, 16, cBrownLt)
	for y := 1; y < 16; y += 4 {
		c.rect(tile, 5, y, 6, 2, cBrown)
	}
}

func (c *canvas) door(tile, part int) {
	c.rect(tile, 0, 0, 16, 16, cNight)
	c.rect(tile, 2, 0, 12, 16, cBrownLt)
	c.rect(tile, 3, 1, 10, 15, cBrown)
	c.rect(tile, 5, 3, 6, 5, cGold)
	c.rect(tile, 11, 9, 2, 2, cYellow)
	if part == 1 {
		c.rect(tile, 2, 0, 12, 2, cRed)
	}
}

func (c *canvas) lift(tile int) {
	c.rect(tile, 0, 0, 16, 16, cSlate)
	c.rect(tile, 0, 0, 2, 16, cSteelLt)
	c.rect(tile, 14, 0, 2, 16, cSteelLt)
	c.rect(tile, 2, 7, 12, 2, cRed)
	c.rect(tile, 2, 8, 12, 1, cRedLt)
}

// WriteStarter writes a new pack into dir: the palette and texts, placeholder tiles for the
// ids the engine cares about, and a small map to walk around in. Existing files are kept.
func WriteStarter(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pal, err := ParseColors(StarterPalette)
	if err != nil {
		return err
	}
	c := &canvas{img: image.NewRGBA(image.Rect(0, 0, 256, 256)), pal: pal}
	c.brick(TileWall, cLimestone, cBrownLt, cBrown)
	c.brick(TileWallLight, cLimeLt, cLimestone, cBrownLt)
	for i := 0; i < 5; i++ {
		c.stairs(TileStairsUp+i, i, true)
		c.stairs(TileStairsDown+i, i, false)
		c.ladder(TileLadder + i)
	}
	for i := 0; i < 3; i++ {
		c.door(TileDoor+i, i%2)
	}
	c.lift(TileLift)
	c.lift(TileLift + 1)
	c.brick(TileFloor, cSteel, cSlate, cNight)
	c.rect(TileFloor, 0, 0, 16, 2, cSteelLt)
	c.rect(TileInterior, 0, 0, 16, 16, cNight)
	for y := 0; y < 16; y += 8 {
		c.rect(TileInterior, 0, y, 16, 1, cSlate)
	}
	c.rect(TileSky, 0, 0, 16, 16, cSea) // the sea, for cells the map leaves empty
	c.rect(TileSky, 0, 12, 16, 4, cSeaLt)
	if err := writeFile(filepath.Join(dir, "tiles.png"), func(f *os.File) error { return png.Encode(f, c.img) }); err != nil {
		return err
	}

	m := Manifest{
		Name:    "Starter pack",
		Palette: StarterPalette,
		Credits: []string{"GNISTAENGINE", "", "A STARTER PACK", "REPLACE THESE TEXTS", "IN PACK.JSON"},
		Messages: []string{
			"", "NOBODY HERE", "THERE IS A KEY CARD\nIN THE DRAWER",
		},
	}
	if err := writeJSON(filepath.Join(dir, "pack.json"), m); err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, "level.tmj"), starterMap())
}

func writeFile(path string, f func(*os.File) error) error {
	if _, err := os.Stat(path); err == nil {
		return nil // keep what the artist has made
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	return f(out)
}

func writeJSON(path string, v any) error {
	return writeFile(path, func(f *os.File) error {
		e := json.NewEncoder(f)
		e.SetIndent("", "  ")
		return e.Encode(v)
	})
}

// starterMap is a corridor of 100 x 12 tiles with two doors and two enemy triggers, as a Tiled map.
func starterMap() map[string]any {
	const w, h, walk = 100, 12, 8 // the player walks along row 8
	data := make([]int, w*h)
	set := func(x, y, id int) { data[y*w+x] = id + 1 } // Tiled counts from 1; 0 is an empty cell
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			set(x, y, TileSky)
		}
	}
	for x := 0; x < w; x++ {
		for y := 4; y <= 5; y++ {
			set(x, y, TileWall)
		}
		for y := 6; y < walk+1; y++ {
			set(x, y, TileInterior)
		}
		set(x, walk+1, TileFloor)
	}
	for y := 4; y <= walk+1; y++ { // the walls at both ends stop the player
		for _, x := range []int{0, 1, 2, w - 3, w - 2, w - 1} {
			set(x, y, TileWallLight)
		}
	}
	// The player only stops on every second tile counted from the start (a walk cycle is two
	// tiles), so doors stand on tiles with the same parity as the start tile.
	set(26, walk, TileDoor)
	set(66, walk, TileDoor)
	obj := func(id int, class string, x, y int, props ...map[string]any) map[string]any {
		return map[string]any{"id": id, "name": class, "type": class, "class": class,
			"x": x * 16, "y": y * 16, "width": 0, "height": 0, "point": true, "visible": true, "properties": props}
	}
	p := func(name string, v any) map[string]any {
		t := "int"
		if _, ok := v.(string); ok {
			t = "string"
		}
		return map[string]any{"name": name, "type": t, "value": v}
	}
	zone := obj(7, "wave_zone", 0, 0)
	zone["point"], zone["width"], zone["height"] = false, w*16, h*16
	return map[string]any{
		"type": "map", "version": "1.10", "tiledversion": "1.10.2", "orientation": "orthogonal",
		"renderorder": "right-down", "infinite": false, "width": w, "height": h, "tilewidth": 16, "tileheight": 16,
		"nextlayerid": 3, "nextobjectid": 8,
		"tilesets": []map[string]any{{
			"firstgid": 1, "name": "tiles", "image": "tiles.png", "imagewidth": 256, "imageheight": 256,
			"tilewidth": 16, "tileheight": 16, "tilecount": 256, "columns": 16, "margin": 0, "spacing": 0,
		}},
		"properties": []map[string]any{{"name": "lift_cards", "type": "string", "value": ""}},
		"layers": []map[string]any{
			{"id": 1, "name": "tiles", "type": "tilelayer", "width": w, "height": h, "x": 0, "y": 0, "visible": true, "opacity": 1, "data": data},
			{"id": 2, "name": "objects", "type": "objectgroup", "draworder": "topdown", "visible": true, "opacity": 1, "x": 0, "y": 0,
				"objects": []map[string]any{
					obj(1, "start", 14, walk),
					obj(2, "door", 26, walk, p("picture", 1), p("message", 1)),
					obj(3, "door", 66, walk, p("picture", 2), p("message", 2), p("card", "1")),
					obj(4, "spawn", 40, walk, p("wave", 1)),
					obj(5, "spawn", 85, walk, p("wave", 0)),
					zone,
				}},
		},
	}
}

// Check loads everything a pack contains the way the game does and reports what it found
// and what looks wrong. It is the engine of packtool check.
func Check(dir string, report func(format string, args ...any)) error {
	p, err := Open(dir)
	if err != nil {
		return err
	}
	report("pack %q in %s", p.Manifest.Name, dir)
	if len(p.palette) > 0 {
		report("palette: %d colours", len(p.palette))
	} else {
		report("palette: none (tiles and sprites cannot be loaded without one)")
	}
	tiles, err := p.Tiles()
	if err != nil {
		return err
	}
	if tiles != nil {
		n := 0
		for i := 0; i < tiles.Count(); i++ {
			if tiles.Has(i) {
				n++
			}
		}
		report("tiles: %d of %d cells drawn", n, tiles.Count())
	}
	sprites, err := p.Sprites()
	if err != nil {
		return err
	}
	if sprites != nil {
		report("sprites: %d frames", sprites.Count())
	}
	for _, name := range p.Screens() {
		s, err := p.Screen(name)
		if err != nil {
			return err
		}
		warn := ""
		if name != "hud" && (s.W != 320 || s.H != 200) {
			warn = "  (the original's are 320x200)"
		}
		report("screen %s: %dx%d, %d colours%s", name, s.W, s.H, len(s.RGB), warn)
	}
	for _, n := range p.Rooms() {
		s, err := p.Room(n)
		if err != nil {
			return err
		}
		report("room picture %d: %dx%d", n, s.W, s.H)
	}
	if f, err := p.Font(); err != nil {
		return err
	} else if f != nil {
		report("font: 128 glyphs")
	}
	prog, err := p.Program()
	if err != nil {
		return err
	}
	if prog != nil {
		report("scripts: assembled, player starts at %#x", prog.Spec.Player)
	}
	sounds, err := p.Sounds()
	if err != nil {
		return err
	}
	if sounds != nil {
		report("sounds: effects %v", SoundIDs(sounds))
	}
	title, game, err := p.Music()
	if err != nil {
		return err
	}
	if title != nil {
		report("music: title track %.1f s", float64(title.Frames())/float64(title.Rate))
	}
	if game != nil {
		report("music: game track %.1f s", float64(game.Frames())/float64(game.Rate))
	}
	lv, err := p.Level()
	if err != nil {
		return err
	}
	if lv == nil {
		for _, w := range p.Warnings {
			report("warning: %s", w)
		}
		return nil
	}
	report("level: %dx%d tiles (%dx%d blocks), %d different blocks, %d doors, start view (%d,%d)",
		lv.WidthTiles(), lv.HeightTiles(), lv.Cols, lv.Rows, len(lv.Blocks)/(20*4), len(lv.T.Rooms), lv.T.StartViewX, lv.T.StartViewY)
	triggers := 0
	for _, t := range lv.T.Triggers {
		if t&1 != 0 {
			triggers++
		}
	}
	report("level: %d enemy triggers", triggers)

	// The tiles the map uses must exist, and doors must sit on door tiles.
	used := map[int]bool{}
	for y := 0; y < lv.HeightTiles(); y++ {
		for x := 0; x < lv.WidthTiles(); x++ {
			used[lv.Tile(x, y)] = true
		}
	}
	if tiles != nil {
		var missing []int
		for id := range used {
			if !tiles.Has(id) {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			report("note: the map uses %d tile ids that tiles.png does not draw (the original's are used instead): %v", len(missing), sorted(missing))
		}
	}
	for _, w := range p.Warnings {
		report("warning: %s", w)
	}
	return nil
}

func sorted(v []int) []int {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
	return v
}
