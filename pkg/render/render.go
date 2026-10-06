// Package render draws the game state. It reads state but never changes it.
package render

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
	"github.com/karlbe/gnistaEngine/pkg/game"
	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
	"github.com/karlbe/gnistaEngine/pkg/input"
)

// Internal resolution: a PAL screen.
const (
	ScreenWidth  = 320
	ScreenHeight = 256
)

// The visible play area. Measured on the emulator's screen: the view position ($8F56,
// $8F58) is 16 pixels left of the visible area's left edge and at its top edge.
const (
	PlayWidth  = 320
	PlayHeight = 128
	viewMargin = 16
)

// Assets are the images the renderer needs, as palette indices.
type Assets struct {
	Tiles         TileSource // map tiles (NSIIcons); a pack supplies its own
	Bobs          BobSource  // actor frames (NSIBobs); a frame number is an index into it
	Palette       []color.RGBA
	HUD           *amiga.ILBM             // NSIMenu: the panel under the play area, with its own palette
	Font          *amiga.Font             // NSIAscii
	Clock         [2][10]uint8            // glyphs for the upper and lower half of each clock digit ($7846, $7850)
	Rooms         []*amiga.ILBM           // room pictures by index ($842C); nil where the file is missing
	Pictures      map[string]*amiga.ILBM  // full-screen pictures by file name (the endings and the title)
	TitlePalettes map[string][]color.RGBA // palette tables of the title ($7316, $7356, $7396), by address
	Credits       []string                // the scrolling credits of the title ($9D82)
	Flash         []color.RGBA            // palette $73D6, the explosion's flash
	Messages      [][]byte                // room messages ($AFA2): 0 starts a new line, 1 ends
}

// TileSource gives the map tiles and the pixels of each that stay in front of actors.
// *amiga.Bank is the original's; a pack (pkg/pack) has its own.
type TileSource interface {
	Tile(i int) (*amiga.Indexed, error)
	TileForeground(i int) []bool
}

// BobSource gives the actor frames, indexed by frame number.
type BobSource interface {
	Bob(i int) (*amiga.Sprite, error)
}

type Renderer struct {
	Debug         bool   // overlay engine state on the play area
	Extras        bool   // not in the original: the additions (minimap, indicators); off shows the original's screen
	Minimap       bool   // not in the original: the minimap in the corner of the play area
	MinimapBottom bool   // the minimap in the strip under the panel instead of the corner
	Menu          *Menu  // not in the original: drawn over the frame when set
	Toast         string // not in the original: a short message at the top of the play area

	a      Assets
	fgTile map[int][]bool // tile id -> its foreground pixels (nil: none), see amiga.Bank.TileForeground
	fg     []bool         // PlayWidth x PlayHeight: pixels of the drawn map that actors are not drawn over
	panelH int            // lines of the HUD picture that are not black, measured on first use
	tiles  map[int]*amiga.Indexed
	bobs   map[int]*amiga.Sprite
	fb     []byte
	img    *ebiten.Image

	miniCells map[[2]int][]color.RGBA // tile id, scale -> scale*scale cell colours
}

func New(a Assets) *Renderer {
	return &Renderer{a: a, tiles: map[int]*amiga.Indexed{}, bobs: map[int]*amiga.Sprite{}, fb: make([]byte, ScreenWidth*ScreenHeight*4)}
}

// Pixels returns the last drawn frame as RGBA.
func (r *Renderer) Pixels() []byte { return r.fb }

func (r *Renderer) Draw(dst *ebiten.Image, s *game.State, level *world.Level) {
	for i := range r.fb {
		r.fb[i] = 0
	}
	vm := s.Engine
	vx, vy := int(vm.G.ViewX)+viewMargin, int(vm.G.ViewY)
	if s.Room != nil {
		r.drawRoom(s.Room)
		r.drawHUD(s)
		r.present(dst)
		return
	}
	pal := r.a.Palette
	if s.Ending != nil {
		step := s.Ending.Current()
		if step == nil || !step.Flash {
			if step != nil {
				r.drawPicture(r.a.Pictures[step.Picture])
			}
			r.present(dst)
			return
		}
		pal = r.a.Flash
	}
	r.drawMap(level, vx, vy, pal)
	for _, sp := range s.Stamps {
		r.drawBobPal(sp, vx, vy, pal)
	}
	// Same order as the original's bob list: the lift cabin ($8128), then the player.
	for _, i := range []int{script.HelperSlot, 0, 1, 2, 3} {
		if a := vm.Slots[i].Actor; a != nil && a.State != 0 && a.State != 2 {
			r.drawBobPal(a.Upper, vx, vy, pal)
			if i != script.HelperSlot { // the cabin is a single bob
				r.drawBobPal(a.Lower, vx, vy, pal)
			}
		}
	}

	r.drawHUD(s)
	if r.Extras && r.Minimap && s.Ending == nil { // after the panel, which paints the strip below it black
		r.drawMinimap(s, level)
	}
	r.present(dst)
	if dst == nil || !r.Debug {
		return
	}
	p := vm.Slots[0]
	msg := fmt.Sprintf("tick %d  input %s\nview (%d,%d)  tile (%d,%d)\nscript $%04X  frames %d/%d\nammo %d+%dx%d  charges %d",
		s.Tick, describe(s.Last), vm.G.ViewX, vm.G.ViewY,
		int(vm.G.ViewX)/world.TileSize+p.TileX, int(vm.G.ViewY)/world.TileSize+p.TileY,
		p.PC, p.Actor.Upper.Frame, p.Actor.Lower.Frame,
		vm.G.Ammo[vm.G.Weapon].Rounds, vm.G.Ammo[vm.G.Weapon].Mags, vm.G.Ammo[vm.G.Weapon].MagSize, vm.G.Charges)
	msg += fmt.Sprintf("\nhits %d  enemies %d", vm.G.Lives, enemiesOn(vm))
	if s.Halted != nil {
		msg += "\nnot ported: " + s.Halted.Error()
	}
	ebitenutil.DebugPrintAt(dst, msg, 4, 16)
}

func (r *Renderer) set(x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= PlayWidth || y >= PlayHeight {
		return
	}
	o := (y*ScreenWidth + x) * 4
	r.fb[o], r.fb[o+1], r.fb[o+2], r.fb[o+3] = c.R, c.G, c.B, 255
}

func (r *Renderer) tile(id int) *amiga.Indexed {
	t, ok := r.tiles[id]
	if !ok {
		if r.a.Tiles != nil {
			t, _ = r.a.Tiles.Tile(id)
		}
		r.tiles[id] = t
	}
	return t
}

func (r *Renderer) drawMap(level *world.Level, vx, vy int, pal []color.RGBA) {
	ts := world.TileSize
	if r.fg == nil {
		r.fg = make([]bool, PlayWidth*PlayHeight)
		r.fgTile = map[int][]bool{}
	}
	for sy := 0; sy < PlayHeight; sy++ {
		wy := vy + sy
		for sx := 0; sx < PlayWidth; sx++ {
			wx := vx + sx
			id := level.Tile(floorDiv(wx, ts), floorDiv(wy, ts))
			r.fg[sy*PlayWidth+sx] = false
			if t := r.tile(id); t != nil {
				r.set(sx, sy, pal[t.At(mod(wx, ts), mod(wy, ts))])
				fg, ok := r.fgTile[id]
				if !ok {
					if r.a.Tiles != nil {
						fg = r.a.Tiles.TileForeground(id)
					}
					r.fgTile[id] = fg
				}
				if fg != nil {
					r.fg[sy*PlayWidth+sx] = fg[mod(wy, ts)*ts+mod(wx, ts)]
				}
			}
		}
	}
}

func (r *Renderer) drawBobPal(sp script.Sprite, vx, vy int, pal []color.RGBA) {
	b, ok := r.bobs[int(sp.Frame)]
	if !ok {
		if r.a.Bobs != nil {
			b, _ = r.a.Bobs.Bob(int(sp.Frame))
		}
		r.bobs[int(sp.Frame)] = b
	}
	if b == nil {
		return
	}
	x0, y0 := int(sp.X)-vx, int(sp.Y)-vy
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if i := y*b.W + x; b.Mask[i] && !r.foreground(x0+x, y0+y) {
				r.set(x0+x, y0+y, pal[b.Pix[i]])
			}
		}
	}
}

// foreground reports whether the map pixel at screen (x, y) is in front of actors.
func (r *Renderer) foreground(x, y int) bool {
	return r.fg != nil && x >= 0 && y >= 0 && x < PlayWidth && y < PlayHeight && r.fg[y*PlayWidth+x]
}

func floorDiv(a, b int) int {
	if a < 0 {
		return -((-a + b - 1) / b)
	}
	return a / b
}

func mod(a, b int) int { return a - floorDiv(a, b)*b }

func describe(a input.Actions) string {
	out := ""
	for _, k := range []struct {
		a input.Actions
		n string
	}{{input.Up, "U"}, {input.Down, "D"}, {input.Left, "L"}, {input.Right, "R"}, {input.Fire, "F"}} {
		if a.Has(k.a) {
			out += k.n
		} else {
			out += "-"
		}
	}
	return out
}

func enemiesOn(vm *script.VM) int {
	n := 0
	for k := 1; k <= 3; k++ {
		if vm.Slots[k].Actor != nil {
			n++
		}
	}
	return n
}

func (r *Renderer) present(dst *ebiten.Image) {
	if r.Toast != "" {
		w := len(r.Toast)*glyphW + 8
		r.fill((PlayWidth-w)/2, 4, w, 14, menuBox)
		r.print((PlayWidth-w)/2+4, 7, r.Toast, menuActive)
	}
	if r.Menu != nil {
		r.drawMenu(r.Menu)
	}
	if dst == nil { // drawn without a window: the pixels are in the buffer (see Pixels)
		return
	}
	if r.img == nil {
		r.img = ebiten.NewImage(ScreenWidth, ScreenHeight)
	}
	r.img.WritePixels(r.fb)
	dst.DrawImage(r.img, nil)
}

// drawPicture shows a full-screen picture with its own palette (the endings, $84E2).
func (r *Renderer) drawPicture(img *amiga.ILBM) {
	if img == nil {
		return
	}
	pal := img.Colors()
	for y := 0; y < ScreenHeight && y < img.H; y++ {
		for x := 0; x < ScreenWidth && x < img.W; x++ {
			c := pal[img.At(x, y)]
			o := (y*ScreenWidth + x) * 4
			r.fb[o], r.fb[o+1], r.fb[o+2], r.fb[o+3] = c.R, c.G, c.B, 255
		}
	}
}
