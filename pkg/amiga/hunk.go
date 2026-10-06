package amiga

import (
	"encoding/binary"
	"fmt"
)

const (
	hunkHeader  = 0x3F3
	hunkCode    = 0x3E9
	hunkData    = 0x3EA
	hunkBSS     = 0x3EB
	hunkReloc32 = 0x3EC
	hunkEnd     = 0x3F2
)

// Hunk is one loadable segment of an AmigaOS executable.
type Hunk struct {
	Type   uint32 // hunkCode, hunkData or hunkBSS
	Data   []byte // nil for BSS
	Size   int
	Relocs map[int][]uint32 // target hunk -> offsets of 32-bit pointers to relocate
}

// DecodeHunks parses an AmigaOS hunk executable (no overlays, no symbols needed).
func DecodeHunks(b []byte) ([]*Hunk, error) {
	be := binary.BigEndian
	pos := 0
	u := func() (uint32, error) {
		if pos+4 > len(b) {
			return 0, fmt.Errorf("unexpected end at %d", pos)
		}
		v := be.Uint32(b[pos:])
		pos += 4
		return v, nil
	}
	if v, err := u(); err != nil || v != hunkHeader {
		return nil, fmt.Errorf("not a hunk executable")
	}
	for { // resident library names
		n, err := u()
		if err != nil {
			return nil, err
		}
		if n == 0 {
			break
		}
		pos += int(n) * 4
	}
	count, _ := u()
	first, _ := u()
	last, err := u()
	if err != nil || int(last-first+1) != int(count) {
		return nil, fmt.Errorf("bad hunk table")
	}
	hunks := make([]*Hunk, count)
	for i := range hunks {
		s, err := u()
		if err != nil {
			return nil, err
		}
		hunks[i] = &Hunk{Size: int(s&0x3FFFFFFF) * 4, Relocs: map[int][]uint32{}}
	}
	for i := 0; i < len(hunks) && pos < len(b); {
		t, err := u()
		if err != nil {
			return nil, err
		}
		h := hunks[i]
		switch t & 0x3FFFFFFF {
		case hunkCode, hunkData:
			n, err := u()
			if err != nil || pos+int(n)*4 > len(b) {
				return nil, fmt.Errorf("truncated hunk %d", i)
			}
			h.Type = t & 0x3FFFFFFF
			h.Data = b[pos : pos+int(n)*4]
			pos += int(n) * 4
		case hunkBSS:
			h.Type = hunkBSS
			pos += 4
		case hunkReloc32:
			for {
				n, err := u()
				if err != nil {
					return nil, err
				}
				if n == 0 {
					break
				}
				target, _ := u()
				for k := uint32(0); k < n; k++ {
					off, err := u()
					if err != nil {
						return nil, err
					}
					h.Relocs[int(target)] = append(h.Relocs[int(target)], off)
				}
			}
		case hunkEnd:
			i++
		default:
			return nil, fmt.Errorf("unsupported hunk type %#x", t)
		}
	}
	return hunks, nil
}
