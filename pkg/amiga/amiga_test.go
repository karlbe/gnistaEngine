package amiga

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The original files are not in the repo; these tests skip when assets-local/ is absent.
func disk(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "assets-local", "adf", name))
	if err != nil {
		t.Skipf("original data not available: %v", err)
	}
	return b
}

func TestILBMScreens(t *testing.T) {
	for _, n := range []string{"NSILoader", "NSIMenu", "NSIRoom1", "NSIRoom15", "PF", "EX", "BB"} {
		img, err := DecodeILBM(disk(t, n))
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if img.W != 320 || img.H != 200 || len(img.Palette) != 32 || len(img.Ranges) != 4 {
			t.Errorf("%s: %dx%d, %d colours, %d ranges", n, img.W, img.H, len(img.Palette), len(img.Ranges))
		}
		for _, p := range img.Pix {
			if p >= 32 {
				t.Fatalf("%s: pixel index %d out of 5-plane range", n, p)
			}
		}
	}
}

func TestJBOBBanks(t *testing.T) {
	for _, c := range []struct {
		name        string
		entries, ok int
	}{{"NSIBobs", 649, 649}, {"NSIIcons", 362, 240}} {
		bank, err := DecodeJBOB(disk(t, c.name))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		valid := 0
		for _, s := range bank.Sprites {
			if s != nil {
				valid++
			}
		}
		if len(bank.Sprites) != c.entries || valid != c.ok {
			t.Errorf("%s: %d entries, %d valid; want %d, %d", c.name, len(bank.Sprites), valid, c.entries, c.ok)
		}
	}
}

func TestTilePlaneMask(t *testing.T) {
	// Synthetic bank: one entry whose tag byte 'D' ($44) routes the single stored plane to
	// screen plane 2, so a fully set stored plane becomes colour 4.
	hdr := make([]byte, 24)
	hdr[11] = 32                         // plane size
	hdr[15], hdr[17], hdr[19] = 4, 16, 1 // row bytes+2, height, width in words
	copy(hdr[20:], "DTRL")
	body := make([]byte, 32)
	for i := range body {
		body[i] = 0xFF
	}
	b := &Bank{hdr: hdr, body: body}
	tile, err := b.Tile(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range tile.Pix {
		if p != 4 {
			t.Fatalf("pixel %d, want 4", p)
		}
	}
}

func TestSamples(t *testing.T) {
	for n, size := range map[string]int{"DAS": 4000, "DBY": 2000, "IAZ": 5000, "NSISound": 82800, "NSIMusicSound": 140252} {
		s, err := DecodeSample(disk(t, n))
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if len(s.Data) != size {
			t.Errorf("%s: %d bytes, want %d", n, len(s.Data), size)
		}
	}
}

func TestFont(t *testing.T) {
	if _, err := DecodeFont(disk(t, "NSIAscii")); err != nil {
		t.Fatal(err)
	}
}

func TestHunks(t *testing.T) {
	hs, err := DecodeHunks(disk(t, "ns"))
	if err != nil {
		t.Fatal(err)
	}
	if len(hs) != 2 || hs[0].Type != hunkCode || len(hs[0].Data) != 47816 || hs[1].Type != hunkBSS {
		t.Fatalf("unexpected hunk layout")
	}
	if n := len(hs[0].Relocs[0]); n != 2024 {
		t.Errorf("%d relocs, want 2024", n)
	}
	if !strings.Contains(string(hs[0].Data), "df0:RoomData") {
		t.Error("filename table missing from code hunk")
	}
}

func TestByteRun1(t *testing.T) {
	// literal "ab", run of 3 'x', no-op, literal "c"
	got, err := unpackByteRun1([]byte{1, 'a', 'b', 0xFE, 'x', 0x80, 0, 'c'}, 6)
	if err != nil || string(got) != "abxxxc" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestRGBA(t *testing.T) {
	if c := RGBA(0x9CF); c.R != 0x99 || c.G != 0xCC || c.B != 0xFF {
		t.Errorf("RGBA(0x9CF) = %v", c)
	}
}

// Tiles whose plane mask has bit 4 carry a foreground plane. The railings and the floor
// edges have it; a plain background tile does not.
func TestTileForeground(t *testing.T) {
	bank, err := DecodeJBOB(disk(t, "NSIIcons"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for i := range bank.Sprites {
		if fg := bank.TileForeground(i); fg != nil {
			n++
			if len(fg) != 256 {
				t.Fatalf("tile %d: %d foreground pixels", i, len(fg))
			}
		}
	}
	if n < 20 || n > 89 {
		t.Errorf("%d tiles have foreground pixels, expected a few dozen", n)
	}
	if bank.TileForeground(0x73) != nil || bank.TileForeground(0x39) == nil {
		t.Errorf("tile $73 (open background) must have no foreground and the floor tile $39 must have some")
	}
	if bank.TileForeground(-1) != nil || bank.TileForeground(100000) != nil {
		t.Error("out of range tiles must have no foreground")
	}
}
