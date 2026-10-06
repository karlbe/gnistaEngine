package main

import (
	"fmt"
	"image/color"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
	"github.com/karlbe/gnistaEngine/pkg/audio"
	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
	"github.com/karlbe/gnistaEngine/pkg/pack"
	"github.com/karlbe/gnistaEngine/pkg/render"
)

// tileMix draws a tile from the pack when its sheet has it and from the original otherwise.
type tileMix struct {
	own  *pack.Tiles
	orig render.TileSource
}

func (m tileMix) Tile(i int) (*amiga.Indexed, error) {
	if m.own != nil && m.own.Has(i) {
		return m.own.Tile(i)
	}
	if m.orig == nil {
		return nil, fmt.Errorf("no tile %d", i)
	}
	return m.orig.Tile(i)
}

func (m tileMix) TileForeground(i int) []bool {
	if m.own != nil && m.own.Has(i) {
		return m.own.TileForeground(i)
	}
	if m.orig == nil {
		return nil
	}
	return m.orig.TileForeground(i)
}

// bobMix does the same for actor frames.
type bobMix struct {
	own  *pack.Sprites
	orig render.BobSource
}

func (m bobMix) Bob(i int) (*amiga.Sprite, error) {
	if m.own != nil && m.own.Has(i) {
		return m.own.Bob(i)
	}
	if m.orig == nil {
		return nil, fmt.Errorf("no sprite %d", i)
	}
	return m.orig.Bob(i)
}

// remap carries the original's tiles and sprites over to a pack's palette: each of the
// original's colours becomes the nearest colour of the pack's.
type remap struct {
	tiles render.TileSource
	bobs  render.BobSource
	lut   [256]uint8
}

func newRemap(from, to []color.RGBA) *remap {
	r := &remap{}
	for i := range r.lut {
		r.lut[i] = uint8(i)
		if i >= len(from) {
			continue
		}
		best, bd := 0, 1<<30
		for j, c := range to {
			dr, dg, db := int(from[i].R)-int(c.R), int(from[i].G)-int(c.G), int(from[i].B)-int(c.B)
			if d := dr*dr + dg*dg + db*db; d < bd {
				best, bd = j, d
			}
		}
		r.lut[i] = uint8(best)
	}
	return r
}

func (r *remap) Tile(i int) (*amiga.Indexed, error) {
	t, err := r.tiles.Tile(i)
	if err != nil {
		return nil, err
	}
	out := &amiga.Indexed{W: t.W, H: t.H, Pix: make([]uint8, len(t.Pix))}
	for k, v := range t.Pix {
		out.Pix[k] = r.lut[v]
	}
	return out, nil
}

func (r *remap) TileForeground(i int) []bool { return r.tiles.TileForeground(i) }

func (r *remap) Bob(i int) (*amiga.Sprite, error) {
	s, err := r.bobs.Bob(i)
	if err != nil {
		return nil, err
	}
	out := *s
	out.Pix = make([]uint8, len(s.Pix))
	for k, v := range s.Pix {
		out.Pix[k] = r.lut[v]
	}
	return &out, nil
}

// packSummary says what a pack replaced, for the start-up message.
type packSummary struct {
	Tiles, Sprites, Screens, Rooms int
	Sounds                         int
	Font, Palette, Level, Texts    bool
	Music, Scripts                 bool
}

func (s packSummary) String() string {
	return fmt.Sprintf("%d tile cells, %d sprites, %d screens, %d room pictures, %d sounds, music %v, scripts %v, font %v, palette %v, level %v, texts %v",
		s.Tiles, s.Sprites, s.Screens, s.Rooms, s.Sounds, s.Music, s.Scripts, s.Font, s.Palette, s.Level, s.Texts)
}

// packResult is what a pack supplies besides the render assets.
type packResult struct {
	Level   *world.Level // nil: the original's
	Credits int          // number of credit lines, -1: the original's
	Sounds  map[int]audio.Entry
	Program *script.Program // nil: the original scripts
	// TitleTrack and GameTrack are recorded music that replaces the tracker's songs.
	TitleTrack, GameTrack *audio.Track
	Summary               packSummary
}

// applyPack replaces the original's assets with what the pack supplies.
func applyPack(p *pack.Pack, as *render.Assets) (res packResult, err error) {
	sum := &res.Summary
	res.Credits = -1
	origTiles, origBobs := as.Tiles, as.Bobs
	if pal := p.Palette(); len(pal) > 0 {
		if origTiles != nil { // the original's tiles and sprites are recoloured to the pack's palette
			rm := newRemap(as.Palette, pal)
			rm.tiles, rm.bobs = origTiles, origBobs
			as.Tiles, as.Bobs = rm, rm
		}
		as.Palette, sum.Palette = pal, true
	}
	if m := p.Manifest; len(m.Flash) > 0 {
		if as.Flash, err = pack.ParseColors(m.Flash); err != nil {
			return res, fmt.Errorf("pack.json flash: %w", err)
		}
	}
	tiles, err := p.Tiles()
	if err != nil {
		return res, err
	}
	if tiles != nil {
		as.Tiles, sum.Tiles = tileMix{tiles, as.Tiles}, tiles.Count()
	}
	sprites, err := p.Sprites()
	if err != nil {
		return res, err
	}
	if sprites != nil {
		as.Bobs, sum.Sprites = bobMix{sprites, as.Bobs}, sprites.Count()
	}
	if as.Pictures == nil {
		as.Pictures = map[string]*amiga.ILBM{}
	}
	for _, name := range p.Screens() {
		img, err := p.Screen(name)
		if err != nil {
			return res, err
		}
		sum.Screens++
		switch name {
		case "hud":
			as.HUD = img
		default:
			as.Pictures[name] = img
		}
		if name == "TA" { // the original's palette tables recolour the original's silhouette
			as.TitlePalettes = map[string][]color.RGBA{}
		}
	}
	for name, list := range p.Manifest.TitlePalettes {
		cols, err := pack.ParseColors(list)
		if err != nil {
			return res, fmt.Errorf("pack.json title_palettes %s: %w", name, err)
		}
		if as.TitlePalettes == nil {
			as.TitlePalettes = map[string][]color.RGBA{}
		}
		as.TitlePalettes[name] = cols
	}
	for _, n := range p.Rooms() {
		img, err := p.Room(n)
		if err != nil {
			return res, err
		}
		for len(as.Rooms) <= n {
			as.Rooms = append(as.Rooms, nil)
		}
		as.Rooms[n] = img
		sum.Rooms++
	}
	if f, err := p.Font(); err != nil {
		return res, err
	} else if f != nil {
		as.Font, sum.Font = f, true
	}
	if m := p.Manifest; len(m.Credits) > 0 {
		as.Credits, res.Credits, sum.Texts = m.Credits, len(m.Credits), true
	}
	if m := p.Manifest; len(m.Messages) > 0 {
		as.Messages = nil
		for _, s := range m.Messages {
			as.Messages = append(as.Messages, pack.MessageBytes(s))
		}
		sum.Texts = true
	}
	if c := p.Manifest.Clock; c != nil {
		as.Clock[0], as.Clock[1] = c.Top, c.Bottom
	}
	if res.Level, err = p.Level(); err != nil {
		return res, err
	}
	sum.Level = res.Level != nil
	if res.Program, err = p.Program(); err != nil {
		return res, err
	}
	sum.Scripts = res.Program != nil
	if res.Sounds, err = p.Sounds(); err != nil {
		return res, err
	}
	sum.Sounds = len(res.Sounds)
	if res.TitleTrack, res.GameTrack, err = p.Music(); err != nil {
		return res, err
	}
	sum.Music = res.TitleTrack != nil || res.GameTrack != nil
	return res, nil
}
