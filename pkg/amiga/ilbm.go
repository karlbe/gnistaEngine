package amiga

import (
	"encoding/binary"
	"fmt"
	"image/color"
)

// Indexed is a chunky (one byte per pixel) palette-indexed image.
type Indexed struct {
	W, H int
	Pix  []uint8
}

func (m *Indexed) At(x, y int) uint8 { return m.Pix[y*m.W+x] }

// ColorRange is a Deluxe Paint colour cycling range (CRNG).
type ColorRange struct {
	Rate    int // 16384 = 60 steps per second
	Active  bool
	Reverse bool
	Low, Hi int
}

// StepsPerSecond returns how many times per second the range rotates by one.
func (r ColorRange) StepsPerSecond() float64 { return float64(r.Rate) * 60 / 16384 }

// ILBM is a decoded IFF ILBM picture.
type ILBM struct {
	Indexed
	Palette     Palette
	RGB         []color.RGBA // full-colour palette; set by a pack, and then used instead of Palette
	Transparent int          // transparent colour index (BMHD), -1 if none
	Ranges      []ColorRange
}

// Colors returns the picture's palette as RGBA: the pack's colours if it has them, else the
// Amiga's 12-bit palette expanded.
func (m *ILBM) Colors() []color.RGBA {
	if m.RGB != nil {
		return m.RGB
	}
	out := make([]color.RGBA, len(m.Palette))
	for i, c := range m.Palette {
		out[i] = RGBA(c)
	}
	return out
}

// DecodeILBM decodes an IFF ILBM (uncompressed or ByteRun1, up to 8 bitplanes).
func DecodeILBM(b []byte) (*ILBM, error) {
	ft, chunks, err := ParseForm(b)
	if err != nil {
		return nil, err
	}
	if ft != "ILBM" {
		return nil, fmt.Errorf("form type %q, want ILBM", ft)
	}
	img := &ILBM{Transparent: -1}
	var body []byte
	var planes, masking, compression int
	for _, c := range chunks {
		switch c.ID {
		case "BMHD":
			if len(c.Data) < 20 {
				return nil, fmt.Errorf("short BMHD")
			}
			img.W = int(binary.BigEndian.Uint16(c.Data[0:]))
			img.H = int(binary.BigEndian.Uint16(c.Data[2:]))
			planes, masking, compression = int(c.Data[8]), int(c.Data[9]), int(c.Data[10])
			if masking == 2 {
				img.Transparent = int(binary.BigEndian.Uint16(c.Data[12:]))
			}
		case "CMAP":
			img.Palette = paletteFromCMAP(c.Data)
		case "CRNG":
			if len(c.Data) >= 8 {
				flags := binary.BigEndian.Uint16(c.Data[4:])
				img.Ranges = append(img.Ranges, ColorRange{
					Rate: int(binary.BigEndian.Uint16(c.Data[2:])), Active: flags&1 != 0,
					Reverse: flags&2 != 0, Low: int(c.Data[6]), Hi: int(c.Data[7]),
				})
			}
		case "BODY":
			body = c.Data
		}
	}
	if img.W == 0 || body == nil {
		return nil, fmt.Errorf("missing BMHD or BODY")
	}
	if planes > 8 {
		return nil, fmt.Errorf("%d planes not supported", planes)
	}
	rowBytes := (img.W + 15) / 16 * 2
	stored := planes
	if masking == 1 {
		stored++
	}
	raw := body
	if compression == 1 {
		if raw, err = unpackByteRun1(body, rowBytes*stored*img.H); err != nil {
			return nil, err
		}
	}
	if len(raw) < rowBytes*stored*img.H {
		return nil, fmt.Errorf("BODY too short")
	}
	img.Pix = make([]uint8, img.W*img.H)
	for y := 0; y < img.H; y++ {
		row := raw[y*rowBytes*stored:]
		for x := 0; x < img.W; x++ {
			var v uint8
			for p := 0; p < planes; p++ {
				v |= (row[p*rowBytes+x>>3] >> (7 - x&7) & 1) << p
			}
			img.Pix[y*img.W+x] = v
		}
	}
	return img, nil
}

func unpackByteRun1(src []byte, size int) ([]byte, error) {
	out := make([]byte, 0, size)
	for i := 0; len(out) < size; {
		if i >= len(src) {
			return nil, fmt.Errorf("ByteRun1 data ends early")
		}
		c := int(int8(src[i]))
		i++
		switch {
		case c >= 0:
			if i+c+1 > len(src) {
				return nil, fmt.Errorf("ByteRun1 literal overruns")
			}
			out = append(out, src[i:i+c+1]...)
			i += c + 1
		case c != -128:
			if i >= len(src) {
				return nil, fmt.Errorf("ByteRun1 run overruns")
			}
			for k := 0; k < 1-c; k++ {
				out = append(out, src[i])
			}
			i++
		}
	}
	return out[:size], nil
}
