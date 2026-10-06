package game

import (
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/game/world"
)

func TestMapMemory(t *testing.T) {
	l, err := world.NewLevel(2, 2, []uint8{0, 0, 0, 0}, make([]uint8, world.BlockW*world.BlockH))
	if err != nil {
		t.Fatal(err)
	}
	m := newMapMemory(l)
	if m.W != 40 || m.H != 8 {
		t.Fatalf("size %dx%d, want 40x8", m.W, m.H)
	}
	m.reveal(-3, 6, 10, 10) // partly outside the level
	for _, c := range []struct {
		x, y int
		want bool
	}{{0, 6, true}, {6, 7, true}, {7, 6, false}, {0, 5, false}, {-1, 6, false}, {0, 8, false}} {
		if got := m.Discovered(c.x, c.y); got != c.want {
			t.Errorf("Discovered(%d,%d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
	m.open(3, 5, 6, true)
	m.open(3, 9, 9, false) // same room: first record stays
	m.open(4, 1, 1, false)
	if len(m.Doors) != 2 || m.Doors[0] != (Door{Room: 3, X: 5, Y: 6, Blown: true}) {
		t.Fatalf("doors = %+v", m.Doors)
	}
}
