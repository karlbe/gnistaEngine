// Command tiles prints the level's tile IDs (hex) in a rectangle, for reverse engineering.
// Coordinates are in tiles; the player's start tile is at view (7984,1360)/16 + (10,6).
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/karlbe/gnistaEngine/pkg/content"
)

func main() {
	root := flag.String("assets", "assets-local", "asset root directory")
	x := flag.Int("x", 499, "left tile column")
	y := flag.Int("y", 85, "top tile row")
	w := flag.Int("w", 40, "width in tiles")
	h := flag.Int("h", 16, "height in tiles")
	find := flag.Int("find", -1, "instead list every position of this tile ID")
	flag.Parse()
	level, err := (content.Loader{Root: *root}).Level()
	if err != nil {
		log.Fatal(err)
	}
	if *find >= 0 {
		for ty := 0; ty < level.HeightTiles(); ty++ {
			for tx := 0; tx < level.WidthTiles(); tx++ {
				if level.Tile(tx, ty) == *find {
					fmt.Println(tx, ty)
				}
			}
		}
		return
	}
	fmt.Print("      ")
	for tx := *x; tx < *x+*w; tx++ {
		fmt.Printf("%3d", tx%1000)
	}
	fmt.Println()
	for ty := *y; ty < *y+*h; ty++ {
		var b strings.Builder
		for tx := *x; tx < *x+*w; tx++ {
			fmt.Fprintf(&b, " %02X", level.Tile(tx, ty))
		}
		fmt.Printf("%4d: %s\n", ty, b.String())
	}
}
