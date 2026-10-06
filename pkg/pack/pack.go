// Package pack loads a game pack: a directory of own assets in open formats (PNG, JSON, WAV,
// Tiled maps) that replaces the original's data piece by piece. The original's files stay
// the fallback for whatever a pack does not supply, so a pack can start small (a few
// screens) and grow until nothing of the original is left.
//
// Layout (everything is optional; see docs/packs.md):
//
//	pack.json          name, palettes, texts
//	tiles.png          map tiles, a grid of 16x16 cells, tile id = row-major index
//	tiles_fg.png       same grid; opaque pixels stay in front of the actors (railings)
//	sprites/N.png      actor frame N, any size, transparent where the actor is not
//	screens/NAME.png   full-screen pictures: title, intro, endings, hud (see docs)
//	rooms/N.png        the pictures shown behind doors, by picture number N
//	font.png           128 glyphs of 16x8 pixels, 16 per row
//	level.tmj          the map, made in Tiled (see docs/packs.md)
//	sounds/N.wav       sound effect N
package pack

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
)

// Manifest is pack.json.
type Manifest struct {
	Name string `json:"name"`

	// Palette is the colours of the game view (tiles, sprites, room pictures), "#rrggbb".
	// Tiles and sprites are matched to it; up to 256 colours.
	Palette []string `json:"palette"`
	// Flash is the palette of the explosion's flash.
	Flash []string `json:"flash"`
	// TitlePalettes recolour the title's silhouette picture step by step, by name.
	TitlePalettes map[string][]string `json:"title_palettes"`

	Credits  []string `json:"credits"`  // the lines that scroll over the title
	Messages []string `json:"messages"` // texts shown in rooms; "\n" starts a new line

	// Clock gives the font glyphs that make the upper and lower half of each clock digit.
	Clock *struct {
		Top    [10]uint8 `json:"top"`
		Bottom [10]uint8 `json:"bottom"`
	} `json:"clock"`

	// SoundVolumes gives effects a volume from 0 to 64, by effect number ("3": 40).
	SoundVolumes map[string]int `json:"sound_volumes"`

	// MusicVolume is the level of music/*.wav, 0-1 (default 0.6).
	MusicVolume float64 `json:"music_volume"`

	// EmptyTile is the tile an empty cell of the map gets (default 0x73, the original's sky).
	EmptyTile *int `json:"empty_tile"`
}

// Pack is an opened pack directory.
type Pack struct {
	Dir      string
	Manifest Manifest
	palette  []color.RGBA

	// Warnings are things that load but look wrong, found while loading the level.
	Warnings []string
}

// Open reads pack.json from dir. A pack without one is allowed (everything default).
func Open(dir string) (*Pack, error) {
	p := &Pack{Dir: dir}
	b, err := os.ReadFile(filepath.Join(dir, "pack.json"))
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, &p.Manifest); err != nil {
			return nil, fmt.Errorf("pack.json: %w", err)
		}
	}
	if p.palette, err = ParseColors(p.Manifest.Palette); err != nil {
		return nil, fmt.Errorf("pack.json palette: %w", err)
	}
	if len(p.palette) > 256 {
		return nil, fmt.Errorf("pack.json palette has %d colours, the most is 256", len(p.palette))
	}
	return p, nil
}

// ParseColors reads "#rrggbb" strings.
func ParseColors(list []string) ([]color.RGBA, error) {
	out := make([]color.RGBA, len(list))
	for i, s := range list {
		s = strings.TrimPrefix(strings.TrimSpace(s), "#")
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil || len(s) != 6 {
			return nil, fmt.Errorf("colour %d: %q is not #rrggbb", i, list[i])
		}
		out[i] = color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
	}
	return out, nil
}

// Palette returns the game view's palette, nil if the pack does not define one.
func (p *Pack) Palette() []color.RGBA { return p.palette }

// path joins a path in the pack.
func (p *Pack) path(parts ...string) string {
	return filepath.Join(append([]string{p.Dir}, parts...)...)
}

// Has reports whether a file exists in the pack.
func (p *Pack) Has(parts ...string) bool {
	_, err := os.Stat(p.path(parts...))
	return err == nil
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}

// opaque reports whether a pixel counts as drawn.
func opaque(c color.Color) bool {
	_, _, _, a := c.RGBA()
	return a >= 0x8000
}

func rgba(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.RGBA{}
	}
	// Un-premultiply: image/png hands back premultiplied colours.
	return color.RGBA{uint8((r * 0xFFFF / a) >> 8), uint8((g * 0xFFFF / a) >> 8), uint8((b * 0xFFFF / a) >> 8), uint8(a >> 8)}
}

// matcher maps colours to the nearest palette index.
type matcher struct {
	pal   []color.RGBA
	cache map[color.RGBA]uint8
}

func newMatcher(pal []color.RGBA) *matcher {
	return &matcher{pal: pal, cache: map[color.RGBA]uint8{}}
}

func (m *matcher) index(c color.RGBA) uint8 {
	c.A = 255
	if i, ok := m.cache[c]; ok {
		return i
	}
	best, bd := 0, 1<<30
	for i, p := range m.pal {
		dr, dg, db := int(c.R)-int(p.R), int(c.G)-int(p.G), int(c.B)-int(p.B)
		if d := dr*dr + dg*dg + db*db; d < bd {
			best, bd = i, d
		}
	}
	m.cache[c] = uint8(best)
	return uint8(best)
}

// indexed converts a region of an image to palette indices and a transparency mask.
func (m *matcher) indexed(img image.Image, r image.Rectangle) (*amiga.Indexed, []bool) {
	w, h := r.Dx(), r.Dy()
	out := &amiga.Indexed{W: w, H: h, Pix: make([]uint8, w*h)}
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.At(r.Min.X+x, r.Min.Y+y)
			if opaque(c) {
				mask[y*w+x] = true
				out.Pix[y*w+x] = m.index(rgba(c))
			}
		}
	}
	return out, mask
}

// Tiles is a tile set read from tiles.png (and tiles_fg.png). It satisfies render.TileSource.
type Tiles struct {
	tiles []*amiga.Indexed
	fg    [][]bool
}

// Count is the number of cells in the tile sheet.
func (t *Tiles) Count() int { return len(t.tiles) }

// Has reports whether the sheet draws tile i. A cell left completely transparent does not:
// a pack can replace tiles one at a time, and the game then uses the original's for the rest.
func (t *Tiles) Has(i int) bool { return i >= 0 && i < len(t.tiles) && t.tiles[i] != nil }

func (t *Tiles) Tile(i int) (*amiga.Indexed, error) {
	if !t.Has(i) {
		return nil, fmt.Errorf("the pack's tile set has no tile %d (%d cells)", i, len(t.tiles))
	}
	return t.tiles[i], nil
}

func (t *Tiles) TileForeground(i int) []bool {
	if i < 0 || i >= len(t.fg) {
		return nil
	}
	return t.fg[i]
}

const tileSize = 16

// Tiles loads tiles.png as a grid of 16x16 tiles, read left to right, top to bottom. It
// returns nil if the pack has none. The palette must be defined.
func (p *Pack) Tiles() (*Tiles, error) {
	if !p.Has("tiles.png") {
		return nil, nil
	}
	if len(p.palette) == 0 {
		return nil, fmt.Errorf("tiles.png needs a palette in pack.json")
	}
	img, err := readPNG(p.path("tiles.png"))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dx()%tileSize != 0 || b.Dy()%tileSize != 0 {
		return nil, fmt.Errorf("tiles.png is %dx%d, not a multiple of %d", b.Dx(), b.Dy(), tileSize)
	}
	var fgImg image.Image
	if p.Has("tiles_fg.png") {
		if fgImg, err = readPNG(p.path("tiles_fg.png")); err != nil {
			return nil, err
		}
	}
	m := newMatcher(p.palette)
	t := &Tiles{}
	for y := b.Min.Y; y < b.Max.Y; y += tileSize {
		for x := b.Min.X; x < b.Max.X; x += tileSize {
			r := image.Rect(x, y, x+tileSize, y+tileSize)
			tile, mask := m.indexed(img, r)
			drawn := false
			for _, on := range mask { // a tile is opaque; a hole becomes colour 0
				drawn = drawn || on
			}
			if !drawn {
				tile = nil
			}
			t.tiles = append(t.tiles, tile)
			var fg []bool
			if fgImg != nil && tile != nil && r.In(fgImg.Bounds()) {
				_, fm := m.indexed(fgImg, r)
				for _, on := range fm {
					if on {
						fg = fm
						break
					}
				}
			}
			t.fg = append(t.fg, fg)
		}
	}
	if len(t.tiles) > 256 {
		return nil, fmt.Errorf("tiles.png has %d tiles, the most is 256", len(t.tiles))
	}
	return t, nil
}

// Sprites are the actor frames of a pack, by frame number. They satisfy render.BobSource.
type Sprites struct {
	frames map[int]*amiga.Sprite
}

// Count is the number of frames.
func (s *Sprites) Count() int { return len(s.frames) }

// Bob returns frame i. A frame the pack does not have is an error, and the caller then
// draws nothing (or falls back to the original's frame).
func (s *Sprites) Bob(i int) (*amiga.Sprite, error) {
	if f := s.frames[i]; f != nil {
		return f, nil
	}
	return nil, fmt.Errorf("the pack has no sprite %d", i)
}

// Has reports whether the pack has frame i.
func (s *Sprites) Has(i int) bool { return s.frames[i] != nil }

// Sprites loads sprites/N.png for every frame number N. It returns nil if there are none.
func (p *Pack) Sprites() (*Sprites, error) {
	es, err := os.ReadDir(p.path("sprites"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(p.palette) == 0 {
		return nil, fmt.Errorf("sprites need a palette in pack.json")
	}
	m := newMatcher(p.palette)
	s := &Sprites{frames: map[int]*amiga.Sprite{}}
	for _, e := range es {
		name, ok := strings.CutSuffix(e.Name(), ".png")
		n, err := strconv.Atoi(name)
		if !ok || err != nil || n < 0 {
			continue
		}
		img, err := readPNG(p.path("sprites", e.Name()))
		if err != nil {
			return nil, err
		}
		pix, mask := m.indexed(img, img.Bounds())
		s.frames[n] = &amiga.Sprite{Indexed: *pix, Mask: mask, Planes: 4}
	}
	if err := p.sameAs(s); err != nil {
		return nil, err
	}
	if len(s.frames) == 0 {
		return nil, nil
	}
	return s, nil
}

// sameAs applies sprites/map.json, which lets a frame reuse another, optionally mirrored:
//
//	{"335": {"same": 333, "flip": true}, "336": {"same": 333}}
//
// The original numbers a pose once for each direction and weapon; with this a pack draws
// each pose once. A frame can refer to one that is itself a reference.
func (p *Pack) sameAs(s *Sprites) error {
	b, err := os.ReadFile(p.path("sprites", "map.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var m map[string]struct {
		Same int  `json:"same"`
		Flip bool `json:"flip"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("sprites/map.json: %w", err)
	}
	pending := map[int]int{} // frame -> frame it copies
	for k, v := range m {
		n, err := strconv.Atoi(k)
		if err != nil || n < 0 {
			return fmt.Errorf("sprites/map.json: %q is not a frame number", k)
		}
		pending[n] = v.Same
	}
	for len(pending) > 0 {
		progress := false
		for n, from := range pending {
			if _, waiting := pending[from]; waiting {
				continue // its source is not ready yet
			}
			src := s.frames[from]
			if src == nil {
				return fmt.Errorf("sprites/map.json: frame %d copies frame %d, which the pack does not have", n, from)
			}
			cp := *src
			if m[strconv.Itoa(n)].Flip {
				cp.Pix, cp.Mask = append([]uint8(nil), src.Pix...), append([]bool(nil), src.Mask...)
				for y := 0; y < cp.H; y++ {
					for x := 0; x < cp.W; x++ {
						cp.Pix[y*cp.W+x] = src.Pix[y*cp.W+cp.W-1-x]
						cp.Mask[y*cp.W+x] = src.Mask[y*cp.W+cp.W-1-x]
					}
				}
			}
			s.frames[n] = &cp
			delete(pending, n)
			progress = true
		}
		if !progress {
			return fmt.Errorf("sprites/map.json: frames copy each other in a circle")
		}
	}
	return nil
}

// picture reads a full-colour picture with its own palette: an indexed PNG keeps its
// indices, any other PNG gets a palette made from its colours (256 at most).
func picture(path string) (*amiga.ILBM, error) {
	img, err := readPNG(path)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	out := &amiga.ILBM{Indexed: amiga.Indexed{W: b.Dx(), H: b.Dy(), Pix: make([]uint8, b.Dx()*b.Dy())}, Transparent: -1}
	if pi, ok := img.(*image.Paletted); ok {
		for _, c := range pi.Palette {
			out.RGB = append(out.RGB, rgba(c))
		}
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				out.Pix[y*b.Dx()+x] = pi.ColorIndexAt(b.Min.X+x, b.Min.Y+y)
			}
		}
		out.RGB = padColours(out.RGB)
		return out, nil
	}
	seen := map[color.RGBA]uint8{}
	var colours []color.RGBA
	for y := 0; y < b.Dy() && len(seen) <= 256; y++ {
		for x := 0; x < b.Dx() && len(seen) <= 256; x++ {
			c := rgba(img.At(b.Min.X+x, b.Min.Y+y))
			c.A = 255
			if _, ok := seen[c]; !ok {
				seen[c] = uint8(len(colours))
				colours = append(colours, c)
			}
		}
	}
	if len(seen) > 256 { // too many colours for an index: reduce them
		pal, idx := quantize(img, 256)
		out.RGB, out.Pix = padColours(pal), idx
		return out, nil
	}
	out.RGB = colours
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := rgba(img.At(b.Min.X+x, b.Min.Y+y))
			c.A = 255
			out.Pix[y*b.Dx()+x] = seen[c]
		}
	}
	out.RGB = padColours(out.RGB)
	return out, nil
}

// padColours makes the palette at least 32 entries, so index lookups never run off it.
func padColours(c []color.RGBA) []color.RGBA {
	for i := range c {
		c[i].A = 255
	}
	for len(c) < 32 {
		c = append(c, color.RGBA{0, 0, 0, 255})
	}
	return c
}

// Screen returns the full-screen picture screens/NAME.png, or nil if the pack has none.
func (p *Pack) Screen(name string) (*amiga.ILBM, error) {
	if !p.Has("screens", name+".png") {
		return nil, nil
	}
	img, err := picture(p.path("screens", name+".png"))
	if err == nil && name == "hud" {
		for _, v := range img.Pix {
			if v > 15 {
				p.Warnings = append(p.Warnings, "screens/hud.png uses more than 16 colours; the panel only shows colours 0-15 (indexed PNG, indices 0-15)")
				break
			}
		}
	}
	return img, err
}

// Screens lists the names of the pack's screens.
func (p *Pack) Screens() []string {
	return p.names("screens")
}

// Room returns the picture shown behind doors with picture number n, or nil.
func (p *Pack) Room(n int) (*amiga.ILBM, error) {
	name := strconv.Itoa(n) + ".png"
	if !p.Has("rooms", name) {
		return nil, nil
	}
	return picture(p.path("rooms", name))
}

// Rooms lists the picture numbers the pack has, ascending.
func (p *Pack) Rooms() []int {
	var out []int
	for _, n := range p.names("rooms") {
		if v, err := strconv.Atoi(n); err == nil {
			out = append(out, v)
		}
	}
	sort.Ints(out)
	return out
}

func (p *Pack) names(dir string) []string {
	es, _ := os.ReadDir(p.path(dir))
	var out []string
	for _, e := range es {
		if n, ok := strings.CutSuffix(e.Name(), ".png"); ok {
			out = append(out, n)
		}
	}
	return out
}

// Font loads font.png: 128 glyphs of 16x8 pixels, 16 per row (a 256x64 picture). A pixel
// that is drawn and not black is set. It returns nil if the pack has none.
func (p *Pack) Font() (*amiga.Font, error) {
	if !p.Has("font.png") {
		return nil, nil
	}
	img, err := readPNG(p.path("font.png"))
	if err != nil {
		return nil, err
	}
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 64 {
		return nil, fmt.Errorf("font.png is %dx%d, want 256x64 (128 glyphs of 16x8, 16 per row)", b.Dx(), b.Dy())
	}
	var f amiga.Font
	b := img.Bounds()
	for g := 0; g < 128; g++ {
		for y := 0; y < 8; y++ {
			for x := 0; x < 16; x++ {
				c := img.At(b.Min.X+g%16*16+x, b.Min.Y+g/16*8+y)
				if r := rgba(c); opaque(c) && (r.R|r.G|r.B) != 0 {
					f[g][y] |= 1 << (15 - x)
				}
			}
		}
	}
	return &f, nil
}

// MessageBytes encodes a room message the way the renderer reads it: 0 starts a new line
// and 1 ends the text.
func MessageBytes(s string) []byte {
	var out []byte
	for _, line := range strings.Split(s, "\n") {
		if len(out) > 0 {
			out = append(out, 0)
		}
		out = append(out, line...)
	}
	return append(out, 1)
}
