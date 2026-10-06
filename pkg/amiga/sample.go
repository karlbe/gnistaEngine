package amiga

import (
	"encoding/binary"
	"fmt"
)

// Sample is a raw 8-bit signed sound as used by the game's sound files: a u32 length,
// a u16 parameter (probably a playback rate or period, not yet confirmed) and the data.
type Sample struct {
	Param uint16
	Data  []int8
}

func DecodeSample(b []byte) (*Sample, error) {
	if len(b) < 6 {
		return nil, fmt.Errorf("sample too short")
	}
	n := int(binary.BigEndian.Uint32(b))
	if 6+n != len(b) {
		return nil, fmt.Errorf("length field %d does not match file size %d", n, len(b))
	}
	s := &Sample{Param: binary.BigEndian.Uint16(b[4:]), Data: make([]int8, n)}
	for i, v := range b[6:] {
		s.Data[i] = int8(v)
	}
	return s, nil
}
