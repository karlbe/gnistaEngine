package amiga

import (
	"encoding/binary"
	"fmt"
)

// Sprite is one blitter object from a JBOB bank, decoded to chunky pixels.
type Sprite struct {
	Indexed
	Mask   []bool // true where the sprite is opaque
	Planes int
	Flags  uint16
	Tag    string
}

// Bank is a FORM JBOB sprite bank. Entries that do not describe a sprite (the unused
// template records at the end of NSIIcons) are nil.
type Bank struct {
	Sprites []*Sprite
	hdr     []byte
	body    []byte
}

// Tile decodes entry i the way the game draws map tiles ($8D66): an opaque 16x16 block on
// a 4-plane screen. The first tag byte is a plane mask: screen plane p receives the next
// stored 32-byte plane if bit p is set, otherwise it is cleared.
func (b *Bank) Tile(i int) (*Indexed, error) {
	const size, planeBytes, screenPlanes = 16, 32, 4
	if i < 0 || (i+1)*24 > len(b.hdr) {
		return nil, fmt.Errorf("tile %d out of range", i)
	}
	e := b.hdr[i*24:]
	off, mask := int(binary.BigEndian.Uint32(e)), e[20]
	planes := make([][]byte, screenPlanes)
	k := 0
	for p := range planes {
		if mask>>p&1 == 0 {
			continue
		}
		if off+(k+1)*planeBytes > len(b.body) {
			return nil, fmt.Errorf("tile %d data out of range", i)
		}
		planes[p] = b.body[off+k*planeBytes : off+(k+1)*planeBytes]
		k++
	}
	t := &Indexed{W: size, H: size, Pix: make([]uint8, size*size)}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var v uint8
			for p, pl := range planes {
				if pl != nil {
					v |= (pl[y*2+x>>3] >> (7 - x&7) & 1) << p
				}
			}
			t.Pix[y*size+x] = v
		}
	}
	return t, nil
}

// TileForeground returns the pixels of tile i that stay in front of actors, or nil when
// there are none. The game's screen has a fifth bitplane that the colours ignore: tiles
// whose plane mask has bit 4 store a fifth plane for it ($8D66, $8DC0), and the blitter
// that draws an actor first takes the actor's mask AND NOT that plane ($80EA, minterm
// $B50), so actors are never drawn over those pixels. This is how the railings and floor
// edges end up in front of the player.
func (b *Bank) TileForeground(i int) []bool {
	const size, planeBytes = 16, 32
	if i < 0 || (i+1)*24 > len(b.hdr) {
		return nil
	}
	e := b.hdr[i*24:]
	off, mask := int(binary.BigEndian.Uint32(e)), e[20]
	if mask&0x10 == 0 {
		return nil
	}
	k := 0 // the stored planes come in plane order, so the fifth follows the colour planes
	for p := 0; p < 4; p++ {
		k += int(mask >> p & 1)
	}
	if off+(k+1)*planeBytes > len(b.body) {
		return nil
	}
	pl := b.body[off+k*planeBytes : off+(k+1)*planeBytes]
	fg := make([]bool, size*size)
	any := false
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if pl[y*2+x>>3]>>(7-x&7)&1 != 0 {
				fg[y*size+x], any = true, true
			}
		}
	}
	if !any {
		return nil
	}
	return fg
}

// Bob decodes entry i the way the game draws actors ($803A): screen plane p (of 4)
// receives the next stored plane if bit p of the first tag byte is set, otherwise it is
// cleared under the mask. The planes are read on past the colour planes into the mask
// plane, so a 3-plane bob with tag $4F gets its mask as plane 3 (colours 8-15).
func (b *Bank) Bob(i int) (*Sprite, error) {
	const screenPlanes = 4
	if i < 0 || i >= len(b.Sprites) || b.Sprites[i] == nil {
		return nil, fmt.Errorf("bob %d does not exist", i)
	}
	s := b.Sprites[i]
	e := b.hdr[i*24:]
	off, psize, mask := int(binary.BigEndian.Uint32(e)), int(binary.BigEndian.Uint32(e[8:])), e[20]
	rb := s.W / 8
	out := &Sprite{Indexed: Indexed{W: s.W, H: s.H, Pix: make([]uint8, len(s.Pix))}, Mask: s.Mask, Planes: screenPlanes, Flags: s.Flags, Tag: s.Tag}
	k := 0
	for p := 0; p < screenPlanes; p++ {
		if mask>>p&1 == 0 {
			continue
		}
		pl := off + k*psize
		k++
		if pl+psize > len(b.body) {
			return nil, fmt.Errorf("bob %d plane %d out of range", i, p)
		}
		for y := 0; y < s.H; y++ {
			for x := 0; x < s.W; x++ {
				out.Pix[y*s.W+x] |= (b.body[pl+y*rb+x>>3] >> (7 - x&7) & 1) << p
			}
		}
	}
	return out, nil
}

// DecodeJBOB decodes a FORM JBOB bank. BHDR holds 24-byte entries:
//
//	u32 body offset, u32 mask-plane offset, u32 plane size, u16 flags,
//	u16 row bytes + 2, u16 height, u16 width in words, 4-byte tag
//
// Colour planes are stored one after another in BODY, followed by the mask plane.
func DecodeJBOB(b []byte) (*Bank, error) {
	ft, chunks, err := ParseForm(b)
	if err != nil {
		return nil, err
	}
	if ft != "JBOB" {
		return nil, fmt.Errorf("form type %q, want JBOB", ft)
	}
	var hdr, body []byte
	for _, c := range chunks {
		switch c.ID {
		case "BHDR":
			hdr = c.Data
		case "BODY":
			body = c.Data
		}
	}
	if hdr == nil || body == nil {
		return nil, fmt.Errorf("missing BHDR or BODY")
	}
	be := binary.BigEndian
	bank := &Bank{Sprites: make([]*Sprite, len(hdr)/24), hdr: hdr, body: body}
	for i := range bank.Sprites {
		e := hdr[i*24:]
		off, moff, psize := int(be.Uint32(e)), int(be.Uint32(e[4:])), int(be.Uint32(e[8:]))
		flags, rb2, h, ww := be.Uint16(e[12:]), int(be.Uint16(e[14:])), int(be.Uint16(e[16:])), int(be.Uint16(e[18:]))
		rb := ww * 2
		if rb*h != psize || psize == 0 || moff < off || (moff-off)%psize != 0 || moff+psize > len(body) || rb2 != rb+2 {
			continue
		}
		s := &Sprite{Planes: (moff - off) / psize, Flags: flags, Tag: string(e[20:24])}
		s.W, s.H = ww*16, h
		s.Pix = make([]uint8, s.W*s.H)
		s.Mask = make([]bool, s.W*s.H)
		for y := 0; y < h; y++ {
			for x := 0; x < s.W; x++ {
				o, bit := y*rb+x>>3, 7-x&7
				var v uint8
				for p := 0; p < s.Planes; p++ {
					v |= (body[off+p*psize+o] >> bit & 1) << p
				}
				s.Pix[y*s.W+x] = v
				s.Mask[y*s.W+x] = body[moff+o]>>bit&1 != 0
			}
		}
		bank.Sprites[i] = s
	}
	return bank, nil
}
