package amiga

import "fmt"

// Font is the game's one-bitplane font (NSIAscii): 128 glyphs of 8 rows, each row a
// 16-bit word, as the print routine ($6A2C) reads it (16 bytes per glyph). Glyphs use the
// left 8 pixels.
type Font [128][8]uint16

func DecodeFont(b []byte) (*Font, error) {
	if len(b) != 128*16 {
		return nil, fmt.Errorf("font size %d, want 2048", len(b))
	}
	var f Font
	for i := range f {
		for r := range f[i] {
			f[i][r] = uint16(b[i*16+r*2])<<8 | uint16(b[i*16+r*2+1])
		}
	}
	return &f, nil
}

// Set reports whether pixel (x, y) of glyph c is set (x 0-15, y 0-7).
func (f *Font) Set(c byte, x, y int) bool { return f[c&127][y]>>(15-x)&1 != 0 }
