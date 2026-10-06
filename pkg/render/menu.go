package render

import (
	"image/color"
	"strings"
)

// Menu is the pause menu drawn over the game (Esc). Not in the original.
type Menu struct {
	Title  string
	Items  []string
	Cursor int
}

var (
	menuText   = color.RGBA{0xE0, 0xE0, 0xE0, 0xFF}
	menuActive = color.RGBA{0xFF, 0xE0, 0x30, 0xFF}
	menuBox    = color.RGBA{0x10, 0x14, 0x28, 0xFF}
)

const glyphW = 8

// fill draws an opaque rectangle on the whole screen.
func (r *Renderer) fill(x0, y0, w, h int, c color.RGBA) {
	for y := max(y0, 0); y < min(y0+h, ScreenHeight); y++ {
		for x := max(x0, 0); x < min(x0+w, ScreenWidth); x++ {
			o := (y*ScreenWidth + x) * 4
			r.fb[o], r.fb[o+1], r.fb[o+2], r.fb[o+3] = c.R, c.G, c.B, 255
		}
	}
}

// print draws upper-case text in the 8x8 font at screen position (x, y).
func (r *Renderer) print(x, y int, text string, c color.RGBA) {
	if r.a.Font == nil {
		return
	}
	for i, ch := range strings.ToUpper(text) {
		if ch > 127 {
			ch = '?'
		}
		for gy := 0; gy < 8; gy++ {
			for gx := 0; gx < 8; gx++ {
				if r.a.Font.Set(byte(ch), gx, gy) {
					r.fill(x+i*glyphW+gx, y+gy, 1, 1, c)
				}
			}
		}
	}
}

func (r *Renderer) drawMenu(m *Menu) {
	for i := 0; i < len(r.fb); i += 4 { // dim the game
		r.fb[i], r.fb[i+1], r.fb[i+2] = r.fb[i]/3, r.fb[i+1]/3, r.fb[i+2]/3
	}
	width := len(m.Title)
	for _, it := range m.Items {
		width = max(width, len(it)+2)
	}
	w, h := width*glyphW+24, (len(m.Items)+2)*12+16
	x0, y0 := (ScreenWidth-w)/2, (ScreenHeight-h)/2
	r.fill(x0-1, y0-1, w+2, h+2, miniBorder)
	r.fill(x0, y0, w, h, menuBox)
	r.print(x0+(w-len(m.Title)*glyphW)/2, y0+10, m.Title, menuText)
	for i, it := range m.Items {
		c, prefix := menuText, "  "
		if i == m.Cursor {
			c, prefix = menuActive, "> "
		}
		r.print(x0+12, y0+10+(i+2)*12, prefix+it, c)
	}
}
