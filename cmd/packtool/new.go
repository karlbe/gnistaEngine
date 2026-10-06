package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/karlbe/gnistaEngine/pkg/pack"
)

func newPack(dir string) error {
	if err := pack.WriteStarter(dir); err != nil {
		return err
	}
	if err := guide(dir); err != nil {
		return err
	}
	fmt.Printf("A starter pack is in %s:\n", dir)
	fmt.Println("  pack.json   palette and texts")
	fmt.Println("  tiles.png   placeholder tiles for the ids the game looks for (guides/tiles_guide.png shows the ids)")
	fmt.Println("  level.tmj   a corridor to walk in; open it in Tiled (tiles.png is its tile set)")
	fmt.Println("Try it:  go run ./cmd/game -pack", dir)
	fmt.Println("Check it: go run ./cmd/packtool check", dir)
	return nil
}

func check(dir string) error {
	return pack.Check(dir, func(format string, args ...any) { fmt.Printf(format+"\n", args...) })
}

// The hex digits as 3x5 bitmaps, for numbering the guide.
var digits = [16][5]uint8{
	{7, 5, 5, 5, 7}, {2, 6, 2, 2, 7}, {7, 1, 7, 4, 7}, {7, 1, 7, 1, 7},
	{5, 5, 7, 1, 1}, {7, 4, 7, 1, 7}, {7, 4, 7, 5, 7}, {7, 1, 1, 1, 1},
	{7, 5, 7, 5, 7}, {7, 5, 7, 1, 7}, {7, 5, 7, 5, 5}, {6, 5, 6, 5, 6},
	{7, 4, 4, 4, 7}, {6, 5, 5, 5, 6}, {7, 4, 7, 4, 7}, {7, 4, 7, 4, 4},
}

// tileClass says what the game does with a tile id (docs/packs.md, "Tile ids").
func tileClass(id int) (name string, c color.RGBA) {
	switch {
	case id < 0x0B:
		return "wall", color.RGBA{0x90, 0x40, 0x40, 255}
	case id <= 0x0F, id >= 0x13 && id <= 0x15:
		return "stairs /", color.RGBA{0xE0, 0xE0, 0xE0, 255}
	case id >= 0x16 && id <= 0x1D:
		return "stairs \\", color.RGBA{0xB0, 0xB0, 0xFF, 255}
	case id >= 0x21 && id <= 0x25:
		return "ladder", color.RGBA{0xD0, 0xA0, 0x50, 255}
	case id >= 0x26 && id <= 0x28:
		return "door", color.RGBA{0xFF, 0x8C, 0x20, 255}
	case id == 0x29 || id == 0x2A:
		return "lift", color.RGBA{0xE0, 0x30, 0x30, 255}
	}
	return "scenery", color.RGBA{0x30, 0x40, 0x50, 255}
}

// guide writes guides/tiles_guide.png: the 16x16 sheet with every cell numbered (hex) and
// coloured by what the game does with that id. Everything in it is drawn here; it contains
// nothing from the original.
func guide(dir string) error {
	const scale = 3
	img := image.NewRGBA(image.Rect(0, 0, 256*scale, 256*scale))
	for id := 0; id < 256; id++ {
		_, c := tileClass(id)
		x0, y0 := id%16*16*scale, id/16*16*scale
		for y := 0; y < 16*scale; y++ {
			for x := 0; x < 16*scale; x++ {
				edge := x == 0 || y == 0
				cc := c
				if edge {
					cc = color.RGBA{0, 0, 0, 255}
				}
				img.SetRGBA(x0+x, y0+y, cc)
			}
		}
		for k, d := range []int{id >> 4, id & 15} {
			for row := 0; row < 5; row++ {
				for col := 0; col < 3; col++ {
					if digits[d][row]>>(2-col)&1 != 0 {
						for sy := 0; sy < scale; sy++ {
							for sx := 0; sx < scale; sx++ {
								img.SetRGBA(x0+(3+k*4+col)*scale+sx, y0+(3+row)*scale+sy, color.RGBA{255, 255, 255, 255})
							}
						}
					}
				}
			}
		}
	}
	out := filepath.Join(dir, "guides")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(out, "tiles_guide.png"))
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
