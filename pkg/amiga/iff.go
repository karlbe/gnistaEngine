// Package amiga decodes the Amiga file formats used by the original game.
package amiga

import (
	"encoding/binary"
	"fmt"
)

// Chunk is one chunk of an IFF FORM.
type Chunk struct {
	ID   string
	Data []byte
}

// ParseForm splits an IFF FORM into its form type and chunks.
func ParseForm(b []byte) (formType string, chunks []Chunk, err error) {
	if len(b) < 12 || string(b[:4]) != "FORM" {
		return "", nil, fmt.Errorf("not an IFF FORM")
	}
	end := 8 + int(binary.BigEndian.Uint32(b[4:8]))
	if end > len(b) {
		end = len(b)
	}
	for i := 12; i+8 <= end; {
		n := int(binary.BigEndian.Uint32(b[i+4 : i+8]))
		if i+8+n > end {
			return "", nil, fmt.Errorf("chunk %q overruns form", b[i:i+4])
		}
		chunks = append(chunks, Chunk{string(b[i : i+4]), b[i+8 : i+8+n]})
		i += 8 + n + n&1
	}
	return string(b[8:12]), chunks, nil
}
