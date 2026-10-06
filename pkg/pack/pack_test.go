package pack

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/game/world"
)

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

var (
	red   = color.RGBA{255, 0, 0, 255}
	green = color.RGBA{0, 255, 0, 255}
	blue  = color.RGBA{0, 0, 255, 255}
)

const manifest = `{"name":"test","palette":["#000000","#ff0000","#00ff00","#0000ff"],
 "messages":["HELLO\nWORLD"],"credits":["A","B"]}`

func newPack(t *testing.T) *Pack {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pack.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTiles(t *testing.T) {
	p := newPack(t)
	img := image.NewRGBA(image.Rect(0, 0, 32, 16))
	fill(img, image.Rect(0, 0, 16, 16), red)
	fill(img, image.Rect(16, 0, 32, 16), green)
	writePNG(t, filepath.Join(p.Dir, "tiles.png"), img)
	fg := image.NewRGBA(image.Rect(0, 0, 32, 16))
	fill(fg, image.Rect(16, 0, 20, 2), blue) // the top left corner of tile 1
	writePNG(t, filepath.Join(p.Dir, "tiles_fg.png"), fg)

	ts, err := p.Tiles()
	if err != nil {
		t.Fatal(err)
	}
	if ts.Count() != 2 {
		t.Fatalf("%d tiles, want 2", ts.Count())
	}
	if a, _ := ts.Tile(0); a.At(3, 3) != 1 {
		t.Errorf("tile 0 pixel = %d, want palette index 1 (red)", a.At(3, 3))
	}
	if a, _ := ts.Tile(1); a.At(15, 15) != 2 {
		t.Errorf("tile 1 pixel = %d, want palette index 2 (green)", a.At(15, 15))
	}
	if ts.TileForeground(0) != nil {
		t.Error("tile 0 has no foreground")
	}
	f := ts.TileForeground(1)
	if f == nil || !f[0] || !f[3] || f[4] || f[2*16] {
		t.Errorf("tile 1 foreground is wrong: %v", f)
	}
	if _, err := ts.Tile(2); err == nil {
		t.Error("tile 2 should not exist")
	}
}

func TestNearestColour(t *testing.T) {
	p := newPack(t)
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	fill(img, img.Bounds(), color.RGBA{250, 10, 5, 255}) // close to red
	writePNG(t, filepath.Join(p.Dir, "tiles.png"), img)
	ts, err := p.Tiles()
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := ts.Tile(0); a.At(0, 0) != 1 {
		t.Errorf("nearest colour = %d, want 1", a.At(0, 0))
	}
}

func TestSprites(t *testing.T) {
	p := newPack(t)
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	fill(img, image.Rect(0, 0, 4, 4), blue) // the right half stays transparent
	writePNG(t, filepath.Join(p.Dir, "sprites", "333.png"), img)
	writePNG(t, filepath.Join(p.Dir, "sprites", "notes.png"), img) // ignored: not a number
	s, err := p.Sprites()
	if err != nil {
		t.Fatal(err)
	}
	if s.Count() != 1 || !s.Has(333) {
		t.Fatalf("sprites: %d, has 333: %v", s.Count(), s.Has(333))
	}
	b, err := s.Bob(333)
	if err != nil {
		t.Fatal(err)
	}
	if b.W != 8 || b.H != 4 || !b.Mask[0] || b.Mask[7] || b.Pix[0] != 3 {
		t.Errorf("sprite 333: %dx%d mask %v pix %d", b.W, b.H, b.Mask[:8], b.Pix[0])
	}
	if _, err := s.Bob(1); err == nil {
		t.Error("sprite 1 should not exist")
	}
}

func TestScreens(t *testing.T) {
	p := newPack(t)
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{10, 20, 30, 255}, color.RGBA{200, 100, 50, 255}}
	pi := image.NewPaletted(image.Rect(0, 0, 4, 2), pal)
	pi.SetColorIndex(1, 0, 2)
	writePNG(t, filepath.Join(p.Dir, "screens", "IT.png"), pi)
	rgb := image.NewRGBA(image.Rect(0, 0, 2, 1))
	rgb.SetRGBA(0, 0, red)
	rgb.SetRGBA(1, 0, blue)
	writePNG(t, filepath.Join(p.Dir, "rooms", "8.png"), rgb)

	s, err := p.Screen("IT")
	if err != nil || s == nil {
		t.Fatalf("screen: %v %v", s, err)
	}
	if s.W != 4 || s.H != 2 || s.At(1, 0) != 2 || s.Colors()[2] != (color.RGBA{200, 100, 50, 255}) {
		t.Errorf("IT: %dx%d, pixel %d, colour %v", s.W, s.H, s.At(1, 0), s.Colors()[2])
	}
	if len(s.Colors()) < 32 {
		t.Errorf("palette has %d entries, want at least 32", len(s.Colors()))
	}
	if x, _ := p.Screen("XX"); x != nil {
		t.Error("XX should not exist")
	}
	r, err := p.Room(8)
	if err != nil || r == nil || r.At(0, 0) == r.At(1, 0) {
		t.Fatalf("room 8: %v %v", r, err)
	}
	if got := p.Rooms(); len(got) != 1 || got[0] != 8 {
		t.Errorf("rooms = %v", got)
	}
}

func TestFont(t *testing.T) {
	p := newPack(t)
	img := image.NewRGBA(image.Rect(0, 0, 256, 64))
	// glyph 65 ('A') is the 2nd row (65/16 = 4, 65%16 = 1): set its top left pixel.
	img.SetRGBA(1*16, 4*8, color.RGBA{255, 255, 255, 255})
	writePNG(t, filepath.Join(p.Dir, "font.png"), img)
	f, err := p.Font()
	if err != nil {
		t.Fatal(err)
	}
	if !f.Set('A', 0, 0) || f.Set('A', 1, 0) || f.Set('B', 0, 0) {
		t.Error("glyph A should have exactly its first pixel set")
	}
}

func TestMessageBytes(t *testing.T) {
	got := MessageBytes("AB\nC")
	want := []byte{'A', 'B', 0, 'C', 1}
	if string(got) != string(want) {
		t.Errorf("MessageBytes = %v, want %v", got, want)
	}
}

// tmj builds a Tiled map of w x h tiles, all gid 0 except the given cells.
func tmj(w, h int, cells map[[2]int]int, objects []map[string]any, props []map[string]any) []byte {
	data := make([]int, w*h)
	for p, gid := range cells {
		data[p[1]*w+p[0]] = gid
	}
	m := map[string]any{
		"width": w, "height": h, "tilewidth": 16, "tileheight": 16,
		"tilesets": []map[string]any{{"firstgid": 1}},
		"layers": []map[string]any{
			{"type": "tilelayer", "name": "tiles", "data": data},
			{"type": "objectgroup", "name": "objects", "objects": objects},
		},
		"properties": props,
	}
	b, _ := json.Marshal(m)
	return b
}

func point(id int, kind string, tx, ty int, props ...map[string]any) map[string]any {
	return map[string]any{"id": id, "type": kind, "x": tx * 16, "y": ty * 16, "properties": props}
}

func prop2(name string, v any) map[string]any { return map[string]any{"name": name, "value": v} }

func level(t *testing.T, p *Pack, doc []byte) (*world.Level, error) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(p.Dir, "level.tmj"), doc, 0o644); err != nil {
		t.Fatal(err)
	}
	return p.Level()
}

func TestLevel(t *testing.T) {
	p := newPack(t)
	// A map two blocks wide and two high (40 x 8 tiles). Tiled's gid 3 is tile 2 (firstgid 1).
	cells := map[[2]int]int{{12, 5}: 3, {25, 5}: 0x27 + 1}
	doc := tmj(40, 8, cells, []map[string]any{
		point(1, "start", 12, 6),
		point(2, "door", 25, 5, prop2("lock", "1,3"), prop2("card", "2"), prop2("weapon", 3), prop2("mags2", 4), prop2("charges", 2), prop2("picture", 7), prop2("message", 6)),
		point(3, "spawn", 14, 6, prop2("wave", 2)),
		point(4, "spawn", 30, 6),
	}, []map[string]any{prop2("lift_cards", "1,2")})
	lv, err := level(t, p, doc)
	if err != nil {
		t.Fatal(err)
	}
	if lv.Cols != 2 || lv.Rows != 2 {
		t.Fatalf("matrix %dx%d, want 2x2", lv.Cols, lv.Rows)
	}
	if got := lv.Tile(12, 5); got != 2 {
		t.Errorf("tile (12,5) = %d, want 2", got)
	}
	if got := lv.Tile(25, 5); got != 0x27 {
		t.Errorf("tile (25,5) = %#x, want 0x27", got)
	}
	if got := lv.Tile(0, 0); got != 0x73 {
		t.Errorf("an empty cell = %#x, want the default sky 0x73", got)
	}
	T := lv.T
	if T.StartViewX != 32 || T.StartViewY != 0 {
		t.Errorf("start view = (%d,%d), want (32,0)", T.StartViewX, T.StartViewY)
	}
	cell := world.CellOf(25, 5, lv.Cols)
	if n := int(T.Room(cell)); n >= len(T.Rooms) {
		t.Fatalf("door cell %d has room %d of %d", cell, n, len(T.Rooms))
	}
	r := T.Rooms[T.Room(cell)]
	want := [world.RoomRecLen]uint8{0b101, 0b10 | 0x80, 0, 4, 0, 2, 7, 6, 0}
	if r != want {
		t.Errorf("room record = %v, want %v", r, want)
	}
	if got := T.Triggers[world.CellOf(14, 6, lv.Cols)]; got != 1|1<<3 {
		t.Errorf("wave 2 trigger = %#b, want bit 0 and bit 3", got)
	}
	if got := T.Triggers[world.CellOf(30, 6, lv.Cols)]; got != 1|1<<6 {
		t.Errorf("single trigger = %#b, want bit 0 and bit 6", got)
	}
	if len(T.LiftCards) != 2 || T.LiftCards[0] != 1 || T.LiftCards[1] != 2 {
		t.Errorf("lift cards = %v", T.LiftCards)
	}
}

func TestLevelBlocksShare(t *testing.T) {
	p := newPack(t)
	// Four equal blocks (40 x 8 tiles of the same tile) are one library entry; one
	// different tile makes a second.
	doc := tmj(40, 8, map[[2]int]int{{0, 0}: 2}, []map[string]any{point(1, "start", 12, 6)}, nil)
	lv, err := level(t, p, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(lv.Blocks) / (world.BlockW * world.BlockH); got != 2 {
		t.Errorf("%d blocks in the library, want 2", got)
	}
	if lv.Cells[0] == lv.Cells[1] || lv.Cells[1] != lv.Cells[2] || lv.Cells[2] != lv.Cells[3] {
		t.Errorf("cells = %v", lv.Cells)
	}
}

func TestWaveZoneSplitsBlocks(t *testing.T) {
	p := newPack(t)
	zone := map[string]any{"id": 9, "type": "wave_zone", "x": 20 * 16, "y": 0, "width": 20 * 16, "height": 4 * 16}
	doc := tmj(40, 8, nil, []map[string]any{point(1, "start", 12, 6), zone}, nil)
	lv, err := level(t, p, doc)
	if err != nil {
		t.Fatal(err)
	}
	if lv.Cells[0] == lv.Cells[1] {
		t.Fatal("equal blocks inside and outside a wave zone must differ")
	}
	if lv.T.BlockAttr[lv.Cells[0]] != 0 || lv.T.BlockAttr[lv.Cells[1]] != 1 {
		t.Errorf("attributes %d, %d", lv.T.BlockAttr[lv.Cells[0]], lv.T.BlockAttr[lv.Cells[1]])
	}
}

func TestLevelErrors(t *testing.T) {
	cases := []struct {
		name string
		doc  []byte
	}{
		{"no start", tmj(20, 4, nil, nil, nil)},
		{"start too close to the edge", tmj(20, 4, nil, []map[string]any{point(1, "start", 3, 2)}, nil)},
		{"two doors in a cell", tmj(40, 8, nil, []map[string]any{point(1, "start", 12, 6), point(2, "door", 25, 5), point(3, "door", 26, 5)}, nil)},
		{"bad wave", tmj(40, 8, nil, []map[string]any{point(1, "start", 12, 6), point(2, "spawn", 25, 5, prop2("wave", 9))}, nil)},
		{"bad card", tmj(40, 8, nil, []map[string]any{point(1, "start", 12, 6), point(2, "door", 25, 5, prop2("lock", "9"))}, nil)},
		{"tile id too big", tmj(20, 4, map[[2]int]int{{0, 0}: 300}, []map[string]any{point(1, "start", 12, 6)}, nil)},
	}
	for _, c := range cases {
		p := newPack(t)
		if _, err := level(t, p, c.doc); err == nil {
			t.Errorf("%s: want an error", c.name)
		} else {
			t.Logf("%s: %v", c.name, err)
		}
	}
}

func TestTooManyBlocks(t *testing.T) {
	p := newPack(t)
	// 300 blocks side by side, each different (the first tile counts up).
	cells := map[[2]int]int{}
	for i := 0; i < 300; i++ {
		cells[[2]int{i * world.BlockW, 0}] = 1 + i%200
		cells[[2]int{i*world.BlockW + 1, 0}] = 1 + i/200
	}
	doc := tmj(300*world.BlockW, 4, cells, []map[string]any{point(1, "start", 12, 6)}, nil)
	if _, err := level(t, p, doc); err == nil {
		t.Fatal("want an error for more than 256 different blocks")
	} else {
		t.Log(err)
	}
	_ = fmt.Sprint
}

// wav16 makes a mono 16-bit PCM WAV file of the samples.
func wav16(rate int, samples []int16) []byte {
	var b []byte
	put32 := func(v int) { b = append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16 := func(v int) { b = append(b, byte(v), byte(v>>8)) }
	b = append(b, "RIFF"...)
	put32(36 + 2*len(samples))
	b = append(b, "WAVEfmt "...)
	put32(16)
	put16(1)
	put16(1)
	put32(rate)
	put32(rate * 2)
	put16(2)
	put16(16)
	b = append(b, "data"...)
	put32(2 * len(samples))
	for _, s := range samples {
		put16(int(uint16(s)))
	}
	return b
}

func TestSounds(t *testing.T) {
	p := newPack(t)
	p.Manifest.SoundVolumes = map[string]int{"4": 30}
	dir := filepath.Join(p.Dir, "sounds")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	samples := make([]int16, 101) // an odd count is padded to whole words
	samples[0], samples[1] = 32767, -32768
	for _, n := range []string{"4.wav", "5.wav"} {
		if err := os.WriteFile(filepath.Join(dir, n), wav16(22050, samples), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := p.Sounds()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("%d sounds, want 2", len(m))
	}
	e := m[4]
	if e.Words != 51 || len(e.Data) != 102 || e.Data[0] != 127 || e.Data[1] != -128 {
		t.Errorf("effect 4: %d words, %d samples, first %d %d", e.Words, len(e.Data), e.Data[0], e.Data[1])
	}
	if e.Period != 161 { // 3546895 / 22050 = 160.86
		t.Errorf("period %d, want 161", e.Period)
	}
	if e.Volume != 30 || m[5].Volume != 64 {
		t.Errorf("volumes %d, %d, want 30 and 64", e.Volume, m[5].Volume)
	}
	if err := os.WriteFile(filepath.Join(dir, "6.wav"), []byte("not a wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Sounds(); err == nil {
		t.Error("a broken WAV file should be an error")
	}
}

func TestScreenWithManyColours(t *testing.T) {
	p := newPack(t)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100)) // 10000 different colours
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 2), uint8(y * 2), uint8((x + y) % 256), 255})
		}
	}
	writePNG(t, filepath.Join(p.Dir, "screens", "ST.png"), img)
	s, err := p.Screen("ST")
	if err != nil || s == nil {
		t.Fatalf("screen: %v %v", s, err)
	}
	if len(s.RGB) > 256 {
		t.Fatalf("%d colours", len(s.RGB))
	}
	// Every pixel is close to its original colour.
	worst := 0
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			want, got := img.RGBAAt(x, y), s.RGB[s.At(x, y)]
			d := abs(int(want.R)-int(got.R)) + abs(int(want.G)-int(got.G)) + abs(int(want.B)-int(got.B))
			worst = max(worst, d)
		}
	}
	if worst > 80 {
		t.Errorf("the worst pixel is off by %d (sum over the channels)", worst)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestSpriteMap(t *testing.T) {
	p := newPack(t)
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	fill(img, image.Rect(0, 0, 1, 2), blue) // the left column only
	writePNG(t, filepath.Join(p.Dir, "sprites", "10.png"), img)
	mapping := `{"11": {"same": 10, "flip": true}, "12": {"same": 11}, "13": {"same": 12, "flip": true}}`
	if err := os.WriteFile(filepath.Join(p.Dir, "sprites", "map.json"), []byte(mapping), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := p.Sprites()
	if err != nil {
		t.Fatal(err)
	}
	if s.Count() != 4 {
		t.Fatalf("%d frames, want 4", s.Count())
	}
	for frame, wantLeft := range map[int]bool{10: true, 11: false, 12: false, 13: true} {
		b, err := s.Bob(frame)
		if err != nil {
			t.Fatal(err)
		}
		if b.Mask[0] != wantLeft || b.Mask[3] == wantLeft {
			t.Errorf("frame %d: mask row %v, want the drawn column on the %s", frame, b.Mask[:4], map[bool]string{true: "left", false: "right"}[wantLeft])
		}
	}
	// A frame copying one that does not exist, or a circle, is an error.
	for _, bad := range []string{`{"11": {"same": 99}}`, `{"11": {"same": 12}, "12": {"same": 11}}`} {
		if err := os.WriteFile(filepath.Join(p.Dir, "sprites", "map.json"), []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Sprites(); err == nil {
			t.Errorf("%s: want an error", bad)
		}
	}
}
