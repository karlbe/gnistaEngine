package render

import (
	"image/color"

	"github.com/karlbe/gnistaEngine/pkg/game"
	"github.com/karlbe/gnistaEngine/pkg/game/script"
)

// Room messages start at (20, 20) in the room's bitmap ($29FC): each glyph in colour 9
// over a black outline, 6 pixels apart, 8 pixels per line.
const (
	msgX, msgY  = 20, 20
	msgColour   = 9
	msgAdvance  = 6
	msgLine     = 8
	msgNewLine  = 0
	msgEnd      = 1
	roomPalSize = 32
)

// drawRoom shows a room picture in the play area with its own palette.
func (r *Renderer) drawRoom(v *game.Visit) {
	if v.Picture >= len(r.a.Rooms) || r.a.Rooms[v.Picture] == nil {
		return
	}
	img := r.a.Rooms[v.Picture]
	pal := make([]color.RGBA, roomPalSize)
	copy(pal, img.Colors())
	for y := 0; y < PlayHeight && y < img.H; y++ {
		for x := 0; x < PlayWidth && x < img.W; x++ {
			r.set(x, y, pal[img.At(x, y)])
		}
	}
	if v.Wires {
		// The cursor is a bob placed relative to the view; on screen that is fixed.
		r.drawBobPal(script.Sprite{X: game.WireX[v.Cursor], Y: game.CursorY, Frame: game.CursorFrame}, viewMargin, 0, pal)
		return
	}
	if !v.Showing() || v.Message >= len(r.a.Messages) || r.a.Font == nil {
		return
	}
	x, y := msgX, msgY
	for _, ch := range r.a.Messages[v.Message] {
		switch ch {
		case msgEnd:
			return
		case msgNewLine:
			x, y = msgX, y+msgLine
			continue
		}
		for _, d := range [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}} {
			r.glyph(x+d[0]-viewMargin, y+d[1], ch, pal[0])
		}
		r.glyph(x-viewMargin, y, ch, pal[msgColour])
		x += msgAdvance
	}
}

// glyph draws the set pixels of a font glyph in the play area.
func (r *Renderer) glyph(x, y int, ch byte, c color.RGBA) {
	for gy := 0; gy < 8; gy++ {
		for gx := 0; gx < 16; gx++ {
			if r.a.Font.Set(ch, gx, gy) {
				r.set(x+gx, y+gy, c)
			}
		}
	}
}
