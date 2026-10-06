package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/karlbe/gnistaEngine/pkg/audio"
	"github.com/karlbe/gnistaEngine/pkg/content"
)

// Where the original's room picture names and messages are kept (hunk-relative).
const (
	roomPictureNames, roomPictures = 0x842C, 11
	roomMessageTable, roomMessages = 0xAFA2, 14
)

// inventory prints what the original has, which is what a pack can replace. It needs the
// original's files and prints sizes and counts only.
func inventory(args []string) error {
	fs := flag.NewFlagSet("inventory", flag.ExitOnError)
	root := fs.String("assets", "assets-local", "asset root directory")
	fs.Parse(args)
	l := content.Loader{Root: *root}

	code, err := l.Code()
	if err != nil {
		return err
	}
	prog, err := l.Program()
	if err != nil {
		return err
	}
	cstr := func(a int) string {
		var b []byte
		for ; code[a] != 0 && code[a] != 1; a++ {
			b = append(b, code[a])
		}
		return string(b)
	}
	long := func(a int) int { return prog.Long(a) }

	fmt.Println("== Screens (screens/NAME.png, any indexed or RGB PNG, up to 256 colours)")
	for _, n := range []string{"NSILoader", "IT", "TA", "PF", "ST", "ET", "EX", "WT", "HC", "EN", "NSIMenu"} {
		img, err := l.ILBM(n)
		if err != nil {
			fmt.Printf("  %-10s missing: %v\n", n, err)
			continue
		}
		name := n
		if n == "NSIMenu" {
			name = "NSIMenu -> hud"
		}
		fmt.Printf("  %-14s %dx%d, %d colours\n", name, img.W, img.H, len(img.Palette))
	}

	fmt.Println("== Room pictures (rooms/N.png)")
	for i := 0; i < roomPictures; i++ {
		name := strings.TrimPrefix(cstr(long(roomPictureNames+4*i)), "df0:")
		img, err := l.ILBM(name)
		if err != nil {
			fmt.Printf("  %2d  %-6s (not on the disk)\n", i, name)
			continue
		}
		fmt.Printf("  %2d  %-6s %dx%d, %d colours\n", i, name, img.W, img.H, len(img.Palette))
	}

	fmt.Println("== Room messages (pack.json messages; one per picture number used)")
	fmt.Printf("  %d messages in the original\n", roomMessages)

	level, err := l.Level()
	if err != nil {
		return err
	}
	used := map[int]int{}
	for y := 0; y < level.HeightTiles(); y++ {
		for x := 0; x < level.WidthTiles(); x++ {
			used[level.Tile(x, y)]++
		}
	}
	var ids []int
	for id := range used {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	fmt.Println("== Tiles (tiles.png: 16x16 cells in id order, at most 256)")
	fmt.Printf("  the map is %dx%d tiles (%dx%d blocks of %dx%d) and uses %d different tile ids, highest %d\n",
		level.WidthTiles(), level.HeightTiles(), level.Cols, level.Rows, 20, 4, len(ids), ids[len(ids)-1])
	fmt.Printf("  %d different blocks of the 256 allowed\n", countBlocks(level.Cells))

	bobs, err := l.Bank("NSIBobs")
	if err != nil {
		return err
	}
	sizes := map[[2]int]int{}
	n := 0
	for _, s := range bobs.Sprites {
		if s != nil {
			sizes[[2]int{s.W, s.H}]++
			n++
		}
	}
	fmt.Println("== Sprites (sprites/N.png, N = frame number; the top left corner is the anchor)")
	fmt.Printf("  %d frames (numbered up to %d) in %d different sizes\n", n, len(bobs.Sprites)-1, len(sizes))

	font, err := l.Font()
	if err == nil {
		_ = font
		fmt.Println("== Font (font.png, 256x64: 128 glyphs of 16x8)")
	}

	table, err := audio.ParseTable(code)
	if err != nil {
		return err
	}
	fmt.Println("== Sound effects (sounds/N.wav)")
	for i, e := range table {
		if e.Words == 0 {
			continue
		}
		fmt.Printf("  %2d  %5d samples, %5.0f Hz, volume %d\n", i, e.Words*2, 3546895/float64(e.Period), e.Volume)
	}
	return nil
}

func countBlocks(cells []uint8) int {
	seen := map[uint8]bool{}
	for _, c := range cells {
		seen[c] = true
	}
	return len(seen)
}
