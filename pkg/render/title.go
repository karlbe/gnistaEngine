package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
	"github.com/karlbe/gnistaEngine/pkg/title"
)

// The credits are printed one line at a time at (creditX, creditY), just below the 200 lines
// of the picture, and the text layer moves up one pixel per pass ($6852: x $1E, y $C9). The
// glyphs are 6 pixels apart ($6A1E).
const (
	creditX, creditY = 0x1E, 0xC9
	creditAdvance    = 6
)

// DrawTitle draws the title sequence.
func (r *Renderer) DrawTitle(dst *ebiten.Image, t *title.Title) {
	for i := range r.fb {
		r.fb[i] = 0
	}
	if cur := t.Current(); cur != nil && cur.Picture != "" {
		if img := r.a.Pictures[cur.Picture]; img != nil {
			pal := r.titlePalette(cur, img)
			r.drawPictureWith(img, pal)
			if cur.Credits {
				r.drawCredits(img, pal, t.ScrollPixels())
			}
		}
	}
	r.present(dst)
}

// titlePalette is the palette a step shows its picture with: a table from the code, or the
// picture's own.
func (r *Renderer) titlePalette(s *title.Step, img *amiga.ILBM) []color.RGBA {
	if p := r.a.TitlePalettes[s.Palette]; s.Palette != "" && len(p) >= 32 {
		return p
	}
	return img.Colors()
}

func (r *Renderer) drawPictureWith(img *amiga.ILBM, pal []color.RGBA) {
	for y := 0; y < ScreenHeight && y < img.H; y++ {
		for x := 0; x < ScreenWidth && x < img.W; x++ {
			c := pal[img.At(x, y)]
			o := (y*ScreenWidth + x) * 4
			r.fb[o], r.fb[o+1], r.fb[o+2], r.fb[o+3] = c.R, c.G, c.B, 255
		}
	}
}

// drawCredits draws the credit lines that have been printed after scroll pixels. The text
// is a layer in the bitplane the silhouette picture leaves free (it only uses even colour
// indices): a text pixel turns the picture's index into the odd one next to it, which the
// palette tables colour white.
func (r *Renderer) drawCredits(img *amiga.ILBM, pal []color.RGBA, scroll int) {
	if r.a.Font == nil {
		return
	}
	for n, line := range r.a.Credits {
		printed := title.LinePixels * n // the scroll position at which this line enters
		if scroll < printed {
			break
		}
		y0 := creditY - (scroll - printed)
		if y0 <= -8 || y0 >= img.H {
			continue
		}
		x := creditX
		for _, ch := range []byte(line) {
			for gy := 0; gy < 8; gy++ {
				y := y0 + gy
				if y < 0 || y >= img.H {
					continue
				}
				for gx := 0; gx < 8; gx++ {
					px := x + gx
					if px < 0 || px >= ScreenWidth || !r.a.Font.Set(ch, gx, gy) {
						continue
					}
					c := pal[img.At(px, y)|1]
					o := (y*ScreenWidth + px) * 4
					r.fb[o], r.fb[o+1], r.fb[o+2], r.fb[o+3] = c.R, c.G, c.B, 255
				}
			}
			x += creditAdvance
		}
	}
}
