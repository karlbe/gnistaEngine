package world

import "testing"

func TestTileLookup(t *testing.T) {
	// Two blocks: block 0 all tile 7, block 1 numbered 0..79.
	blocks := make([]uint8, 2*BlockW*BlockH)
	for i := 0; i < BlockW*BlockH; i++ {
		blocks[i] = 7
		blocks[BlockW*BlockH+i] = uint8(i)
	}
	// 2x1 matrix: [0 1]
	l, err := NewLevel(2, 1, []uint8{0, 1}, blocks)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ x, y, want int }{
		{0, 0, 7}, {19, 3, 7}, {20, 0, 0}, {21, 0, 1}, {20, 1, 20}, {39, 3, 79},
		{-1, 0, -1}, {40, 0, -1}, {0, 4, -1},
	}
	for _, c := range cases {
		if got := l.Tile(c.x, c.y); got != c.want {
			t.Errorf("Tile(%d,%d) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
}
