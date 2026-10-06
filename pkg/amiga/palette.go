package amiga

import (
	"encoding/binary"
	"image/color"
)

// Palette holds colours as the Amiga stores them: 12-bit $0RGB words.
type Palette []uint16

// RGBA expands a 12-bit colour the way the Amiga hardware does (4 bits per channel, x17).
func RGBA(c uint16) color.RGBA {
	return color.RGBA{uint8(c>>8&15) * 17, uint8(c>>4&15) * 17, uint8(c&15) * 17, 255}
}

// PaletteFromWords reads n big-endian 12-bit colour words.
func PaletteFromWords(b []byte, n int) Palette {
	p := make(Palette, n)
	for i := range p {
		p[i] = binary.BigEndian.Uint16(b[i*2:]) & 0x0FFF
	}
	return p
}

// paletteFromCMAP converts 8-bit RGB triplets (as written by Deluxe Paint) to 12 bits.
func paletteFromCMAP(b []byte) Palette {
	p := make(Palette, len(b)/3)
	for i := range p {
		r, g, bl := b[i*3], b[i*3+1], b[i*3+2]
		p[i] = uint16(r>>4)<<8 | uint16(g>>4)<<4 | uint16(bl>>4)
	}
	return p
}
