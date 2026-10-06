// Command viewer browses the game data through the content loader: full-screen pictures
// (with colour cycling), the sprite banks and the whole level.
//
// Keys: Tab = next mode. Pictures: Left/Right. Banks: Up/Down = page. Level: arrows scroll
// (Shift = fast), mouse wheel or +/- zoom around the cursor, Home = start view. The mouse
// shows ids under the cursor.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
)

const (
	W, H     = 640, 400
	cellW    = 36
	cellH    = 42
	gridCols = W / cellW
	gridRows = (H - 16) / cellH
	perPage  = gridCols * gridRows
)

type mode int

const (
	modePictures mode = iota
	modeBobs
	modeIcons
	modeLevel
	modeCount
)

type viewer struct {
	mode     mode
	pictures []string
	images   map[string]*amiga.ILBM
	bobs     *amiga.Bank
	icons    *amiga.Bank
	level    *world.Level
	pal      []color.RGBA
	ext      *content.Extracted
	loader   content.Loader

	pic, page  [modeCount]int
	camX, camY int
	zoom       int
	fb         []byte
	screen     *ebiten.Image
	start      time.Time
	status     string
	shot       string // save the first frame here and exit
	shotDone   bool
	tiles      map[int]*amiga.Indexed
}

// tile returns map tile id decoded as the game draws it (opaque, plane-mask remapped).
func (v *viewer) tile(id int) *amiga.Indexed {
	if t, ok := v.tiles[id]; ok {
		return t
	}
	t, err := v.icons.Tile(id)
	if err != nil {
		t = nil
	}
	v.tiles[id] = t
	return t
}

func main() {
	root := flag.String("assets", "assets-local", "asset root directory")
	scale := flag.Int("scale", 2, "window scale")
	startMode := flag.String("mode", "level", "start mode: pictures, bobs, icons or level")
	pic := flag.Int("pic", 0, "picture index (pictures mode) or page (bobs/icons)")
	shot := flag.String("shot", "", "render one frame to this PNG file and exit")
	zoom := flag.Int("zoom", 0, "level zoom: 2^zoom world pixels per screen pixel (-2..5)")
	flag.Parse()

	v, err := load(content.Loader{Root: *root})
	if err != nil {
		log.Fatal(err)
	}
	modes := map[string]mode{"pictures": modePictures, "bobs": modeBobs, "icons": modeIcons, "level": modeLevel}
	m, ok := modes[*startMode]
	if !ok {
		log.Fatalf("unknown mode %q", *startMode)
	}
	v.mode, v.shot = m, *shot
	v.zoom = clamp(*zoom, minZoom, maxZoom)
	v.clampCamera()
	if m == modePictures {
		v.pic[m] = *pic % len(v.pictures)
	} else {
		v.page[m] = *pic
	}
	ebiten.SetWindowSize(W**scale, H**scale)
	ebiten.SetWindowTitle("GnistaEngine - viewer")
	if err := ebiten.RunGame(v); err != nil {
		log.Fatal(err)
	}
}

func load(l content.Loader) (*viewer, error) {
	v := &viewer{loader: l, images: map[string]*amiga.ILBM{}, tiles: map[int]*amiga.Indexed{},
		fb: make([]byte, W*H*4), start: time.Now(), mode: modeLevel}
	files, err := l.DiskFiles()
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if img, err := l.ILBM(f); err == nil {
			v.pictures = append(v.pictures, f)
			v.images[f] = img
		}
	}
	if v.bobs, err = l.Bank("NSIBobs"); err != nil {
		return nil, err
	}
	if v.icons, err = l.Bank("NSIIcons"); err != nil {
		return nil, err
	}
	if v.ext, err = l.Extracted(); err != nil {
		return nil, err
	}
	if v.level, err = l.Level(); err != nil {
		return nil, err
	}
	for _, c := range v.ext.Palettes[v.ext.Gameplay] {
		v.pal = append(v.pal, amiga.RGBA(c))
	}
	v.home()
	return v, nil
}

func (v *viewer) home() { v.camX, v.camY = v.ext.StartX, v.ext.StartY }

func (v *viewer) Update() error {
	if v.shotDone {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		v.mode = (v.mode + 1) % modeCount
	}
	switch v.mode {
	case modePictures:
		n := len(v.pictures)
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			v.pic[v.mode] = (v.pic[v.mode] + 1) % n
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			v.pic[v.mode] = (v.pic[v.mode] + n - 1) % n
		}
	case modeBobs, modeIcons:
		pages := (len(v.bank().Sprites) + perPage - 1) / perPage
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
			v.page[v.mode] = (v.page[v.mode] + 1) % pages
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
			v.page[v.mode] = (v.page[v.mode] + pages - 1) % pages
		}
	case modeLevel:
		step := 4 * v.scale()
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			step *= 8
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
			v.camX -= int(step)
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
			v.camX += int(step)
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
			v.camY -= int(step)
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
			v.camY += int(step)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
			v.home()
		}
		_, wheel := ebiten.Wheel()
		switch {
		case wheel > 0 || inpututil.IsKeyJustPressed(ebiten.KeyEqual) || inpututil.IsKeyJustPressed(ebiten.KeyKPAdd):
			v.zoomAt(v.zoom - 1)
		case wheel < 0 || inpututil.IsKeyJustPressed(ebiten.KeyMinus) || inpututil.IsKeyJustPressed(ebiten.KeyKPSubtract):
			v.zoomAt(v.zoom + 1)
		}
		v.clampCamera()
	}
	return nil
}

// Zoom k: 2^k world pixels per screen pixel (negative = zoomed in).
const minZoom, maxZoom = -2, 5

func (v *viewer) scale() float64 { return math.Pow(2, float64(v.zoom)) }

// zoomAt changes the zoom level, keeping the world point under the mouse in place.
func (v *viewer) zoomAt(k int) {
	if k < minZoom || k > maxZoom {
		return
	}
	mx, my := ebiten.CursorPosition()
	wx, wy := v.toWorld(mx, my)
	v.zoom = k
	v.camX = wx - int(float64(mx)*v.scale())
	v.camY = wy - int(float64(my)*v.scale())
}

func (v *viewer) toWorld(sx, sy int) (int, int) {
	s := v.scale()
	return v.camX + int(math.Floor(float64(sx)*s)), v.camY + int(math.Floor(float64(sy)*s))
}

// clampCamera keeps the view inside the level, or centres the level when it is smaller.
func (v *viewer) clampCamera() {
	lw, lh := v.level.WidthTiles()*world.TileSize, v.level.HeightTiles()*world.TileSize
	vw, vh := int(W*v.scale()), int(H*v.scale())
	if lw <= vw {
		v.camX = (lw - vw) / 2
	} else {
		v.camX = clamp(v.camX, 0, lw-vw)
	}
	if lh <= vh {
		v.camY = (lh - vh) / 2
	} else {
		v.camY = clamp(v.camY, 0, lh-vh)
	}
}

func (v *viewer) bank() *amiga.Bank {
	if v.mode == modeIcons {
		return v.icons
	}
	return v.bobs
}

func (v *viewer) Draw(screen *ebiten.Image) {
	for i := range v.fb {
		v.fb[i] = 0
	}
	switch v.mode {
	case modePictures:
		v.drawPicture()
	case modeBobs, modeIcons:
		v.drawBank()
	case modeLevel:
		v.drawLevel()
	}
	if v.screen == nil {
		v.screen = ebiten.NewImage(W, H)
	}
	v.screen.WritePixels(v.fb)
	screen.DrawImage(v.screen, nil)
	ebitenutil.DebugPrint(screen, v.status)
	if v.shot != "" && !v.shotDone {
		v.shotDone = true
		if err := savePNG(v.shot, v.fb); err != nil {
			log.Print(err)
		}
	}
}

func savePNG(path string, rgba []byte) error {
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	copy(img.Pix, rgba)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func (v *viewer) Layout(int, int) (int, int) { return W, H }

func (v *viewer) set(x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= W || y >= H {
		return
	}
	o := (y*W + x) * 4
	v.fb[o], v.fb[o+1], v.fb[o+2], v.fb[o+3] = c.R, c.G, c.B, 255
}

func (v *viewer) drawPicture() {
	name := v.pictures[v.pic[modePictures]]
	img := v.images[name]
	pal := cycled(img, time.Since(v.start).Seconds())
	for y := 0; y < img.H && y*2 < H; y++ {
		for x := 0; x < img.W && x*2 < W; x++ {
			c := pal[img.At(x, y)]
			v.set(x*2, y*2, c)
			v.set(x*2+1, y*2, c)
			v.set(x*2, y*2+1, c)
			v.set(x*2+1, y*2+1, c)
		}
	}
	v.status = fmt.Sprintf("Tab: mode | Left/Right: picture | %s (%d/%d)", name, v.pic[modePictures]+1, len(v.pictures))
}

// cycled returns the picture's palette with its active CRNG ranges rotated for time t.
func cycled(img *amiga.ILBM, t float64) []color.RGBA {
	pal := make([]color.RGBA, len(img.Palette))
	for i, c := range img.Palette {
		pal[i] = amiga.RGBA(c)
	}
	for _, r := range img.Ranges {
		n := r.Hi - r.Low + 1
		if !r.Active || r.Rate == 0 || n < 2 || r.Hi >= len(pal) {
			continue
		}
		shift := int(t*r.StepsPerSecond()) % n
		if r.Reverse {
			shift = n - shift
		}
		orig := append([]color.RGBA(nil), pal[r.Low:r.Hi+1]...)
		for i := range orig {
			pal[r.Low+(i+shift)%n] = orig[i]
		}
	}
	return pal
}

func (v *viewer) drawBank() {
	bank := v.bank()
	first := v.page[v.mode] * perPage
	mx, my := ebiten.CursorPosition()
	hover := ""
	for k := 0; k < perPage && first+k < len(bank.Sprites); k++ {
		cx, cy := k%gridCols*cellW+2, 16+k/gridCols*cellH
		s := bank.Sprites[first+k]
		if s == nil {
			continue
		}
		for y := 0; y < s.H; y++ {
			for x := 0; x < s.W; x++ {
				c := color.RGBA{40, 40, 48, 255}
				if (x/4+y/4)%2 == 0 {
					c = color.RGBA{56, 56, 64, 255}
				}
				if s.Mask[y*s.W+x] {
					c = v.pal[s.Pix[y*s.W+x]]
				}
				v.set(cx+x, cy+y, c)
			}
		}
		if mx >= cx && mx < cx+cellW-2 && my >= cy && my < cy+cellH {
			hover = fmt.Sprintf(" | #%d %dx%d %d planes flags %04X tag %q", first+k, s.W, s.H, s.Planes, s.Flags, s.Tag)
		}
	}
	name := map[mode]string{modeBobs: "NSIBobs", modeIcons: "NSIIcons"}[v.mode]
	pages := (len(bank.Sprites) + perPage - 1) / perPage
	v.status = fmt.Sprintf("%s page %d/%d%s", name, v.page[v.mode]+1, pages, hover)
}

func (v *viewer) drawLevel() {
	ts := world.TileSize
	// Sample every screen pixel; works for any zoom level.
	wxs := make([]int, W)
	for sx := range wxs {
		wxs[sx], _ = v.toWorld(sx, 0)
	}
	for sy := 0; sy < H; sy++ {
		_, wy := v.toWorld(0, sy)
		if wy < 0 {
			continue
		}
		for sx, wx := range wxs {
			if wx < 0 {
				continue
			}
			id := v.level.Tile(wx/ts, wy/ts)
			if id < 0 {
				continue
			}
			if t := v.tile(id); t != nil {
				v.set(sx, sy, v.pal[t.At(wx%ts, wy%ts)])
			}
		}
	}
	mx, my := ebiten.CursorPosition()
	wx, wy := v.toWorld(mx, my)
	tx, ty := floorDiv(wx, ts), floorDiv(wy, ts)
	bc, br := floorDiv(tx, world.BlockW), floorDiv(ty, world.BlockH)
	block := -1
	if bc >= 0 && br >= 0 && bc < v.level.Cols && br < v.level.Rows {
		block = int(v.level.Cells[br*v.level.Cols+bc])
	}
	var zoom string
	if v.zoom > 0 {
		zoom = fmt.Sprintf("1:%d", 1<<v.zoom)
	} else {
		zoom = fmt.Sprintf("%dx", 1<<-v.zoom)
	}
	v.status = fmt.Sprintf("Level %s | view (%d,%d) | mouse px (%d,%d) tile (%d,%d) id %d | block (%d,%d) id %d\nArrows scroll, Shift fast, wheel or +/- zoom, Home start view",
		zoom, v.camX, v.camY, wx, wy, tx, ty, v.level.Tile(tx, ty), bc, br, block)
}

func floorDiv(a, b int) int {
	if a < 0 {
		return -((-a + b - 1) / b)
	}
	return a / b
}

func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
