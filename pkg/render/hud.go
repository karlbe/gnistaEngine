package render

import (
	"image/color"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
	"github.com/karlbe/gnistaEngine/pkg/game"
)

// The panel is shown from screen line 128 (raster line 172, where the copper list switches
// planes and palette). Text coordinates are in the panel's bitmap, which, like the play
// area, starts 16 pixels left of the visible edge.
const hudTop = PlayHeight

// HUD colours set by the game in the panel palette ($7296).
const (
	hudSelected = 0xF00 // border of the selected weapon (colours 13-15)
	hudOther    = 0xFFF
)

func (r *Renderer) drawHUD(s *game.State) {
	if r.a.HUD == nil {
		return
	}
	g := s.Engine.G
	pal := make([]color.RGBA, 16)
	copy(pal, r.a.HUD.Colors())
	// F1-F3 colour the weapon borders ($13AC); picking up a weapon shows its icon ($2690, $26BA).
	for w := 0; w < 3; w++ {
		c := uint16(hudOther)
		if int(g.Weapon) == w {
			c = hudSelected
		}
		pal[13+w] = amiga.RGBA(c)
	}
	if g.WeaponsOwned&2 != 0 {
		pal[7], pal[8], pal[9] = amiga.RGBA(0xBBB), amiga.RGBA(0x777), amiga.RGBA(0)
	}
	if g.WeaponsOwned&4 != 0 {
		pal[4], pal[5], pal[6] = amiga.RGBA(0xBBB), amiga.RGBA(0x777), amiga.RGBA(0)
	}
	img := r.a.HUD
	for y := 0; y < ScreenHeight-hudTop && y < img.H; y++ {
		for x := 0; x < ScreenWidth && x < img.W; x++ {
			r.setHUD(x, y, pal[img.At(x, y)&15])
		}
	}
	// The values the game prints into the panel (redrawn in the original when they change).
	r.text(pal, 0x13E, 0x09, '0'+byte(g.Charges), 0, 0xA) // $78B6
	for w, x := range []int{0x28, 0x52, 0xBE} {           // $7974
		r.text(pal, x, 0x25, '0'+g.Ammo[w].Mags, 0, 0xA)
		// Not in the original: rounds left in the magazine, on the line above.
		if r.Extras && g.WeaponsOwned&(1<<w) != 0 {
			r.text(pal, x-6, 0x1D, '(', 0, 0xA)
			r.number(pal, x, 0x1D, int(g.Ammo[w].Rounds), 0, 0xA)
			r.text(pal, x+12, 0x1D, ')', 0, 0xA)
		}
	}
	for k, fg := range []uint8{1, 2, 2, 3, 0xB} { // cards, $26E4-$27CC
		if g.Cards&(1<<k) != 0 {
			r.text(pal, 0xFA+8*k, 0x22, 0x1F, fg, 0xA)
		}
	}
	r.number(pal, 0x10E, 0x32, 0x15-floorDiv(floorDiv(int(g.ViewY), 16), 4), 0, 3) // floor, $78F8
	r.number(pal, 0x10E, 0x3F, 9-int(g.Lives), 0, 3)                               // hits, $7936
	// Not in the original: the damage counter ($1C05) in brackets after the hits. It starts
	// at 0 (the original never sets it), so the first hit always costs a hit.
	if r.Extras {
		r.text(pal, 0x11C, 0x3F, '(', 0, 3)
		r.text(pal, 0x11C+6, 0x3F, '0'+byte(max(g.Health, 0)), 0, 3)
		r.text(pal, 0x11C+12, 0x3F, ')', 0, 3)
	}
	// Not in the original: cheat indicators on the left of the clock.
	if r.Extras && s.Invulnerable {
		r.text(pal, 0x24, 0x33, 'I', 0xC, 0)
		r.text(pal, 0x2C, 0x33, 'N', 0xC, 0)
		r.text(pal, 0x34, 0x33, 'V', 0xC, 0)
	}
	if r.Extras && s.Paused {
		for i, ch := range "PAUSED" {
			r.text(pal, 0xB0+8*i, 0x37, uint8(ch), 0xC, 0)
		}
	}
	if r.Extras && s.Fast {
		r.text(pal, 0x24, 0x3B, 'x', 0xC, 0)
		r.text(pal, 0x2C, 0x3B, '2', 0xC, 0)
	}
	c := s.Clock // $7706-$77FC
	for _, d := range []struct{ x, v int16 }{{0x79, c.Min10}, {0x83, c.Min}, {0x8E, c.Sec10}, {0x98, c.Sec}} {
		r.text(pal, int(d.x), 0x33, r.a.Clock[0][d.v], 0xC, 0)
		r.text(pal, int(d.x), 0x3B, r.a.Clock[1][d.v], 0xC, 0)
	}
}

// number prints the last two decimal digits of v, 6 pixels apart ($69E4).
func (r *Renderer) number(pal []color.RGBA, x, y, v int, fg, bg uint8) {
	r.text(pal, x, y, '0'+byte(v/10%10), fg, bg)
	r.text(pal, x+6, y, '0'+byte(v%10), fg, bg)
}

// text draws glyph ch as an opaque 8x8 cell: glyph pixels in colour fg, the rest in bg
// (the print routine $6A2C in the mode the panel ends up using).
func (r *Renderer) text(pal []color.RGBA, x, y int, ch, fg, bg uint8) {
	if r.a.Font == nil {
		return
	}
	for gy := 0; gy < 8; gy++ {
		for gx := 0; gx < 8; gx++ {
			c := pal[bg]
			if r.a.Font.Set(ch, gx, gy) {
				c = pal[fg]
			}
			r.setHUD(x-viewMargin+gx, y+gy, c)
		}
	}
}

func (r *Renderer) setHUD(x, y int, c color.RGBA) {
	y += hudTop
	if x < 0 || y < hudTop || x >= ScreenWidth || y >= ScreenHeight {
		return
	}
	o := (y*ScreenWidth + x) * 4
	r.fb[o], r.fb[o+1], r.fb[o+2], r.fb[o+3] = c.R, c.G, c.B, 255
}
