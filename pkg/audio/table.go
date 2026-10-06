// Package audio plays the original's sound effects: the sample table, and a mixer that
// behaves like the four Paula channels the original drives from its scripts.
package audio

import (
	"encoding/binary"
	"fmt"
)

// Entry is one effect: 10 bytes at $6432 + 10*id in the original (data pointer, length in
// words, period, volume), filled in at startup by the code at $61B0.
type Entry struct {
	Offset int // byte offset into the NSISound data (after its 6-byte header)
	Words  int // length in 16-bit words
	Period int // Paula period: sample rate = 3546895 / Period (PAL)
	Volume int // 0-64

	// Gain is not in the original: a factor applied on top of Volume, 0 meaning 1. Used to
	// make effects that are barely audible in the original data (the stair steps) audible.
	Gain float64

	// Data is the sample data the entry points into; nil means the mixer's own (NSISound).
	// The music's instruments live in other files.
	Data []int8
}

const (
	tableAddr  = 0x6432 // first entry
	entrySize  = 10
	initStart  = 0x61B0 // table setup code
	initEnd    = 0x6430
	soundBase  = 0x64FA // pointer to the loaded NSISound file
	maxEntries = 32
)

// ParseTable reads the effect table out of the code hunk. The setup code at $61B0 is a
// straight list of
//
//	lea entry,a0 / move.l d0,(a0) / [addi.l #off,(a0)] / move.w #len,4(a0) /
//	move.w #period,6(a0) / move.b #vol,8(a0)
//
// with d0 = the sound file's data start, so it is decoded here instead of copied.
func ParseTable(code []byte) ([]Entry, error) {
	if len(code) < initEnd+2 {
		return nil, fmt.Errorf("code hunk too short")
	}
	w := func(a int) int { return int(binary.BigEndian.Uint16(code[a:])) }
	l := func(a int) int { return int(binary.BigEndian.Uint32(code[a:])) }
	// The routine starts with: move.l $64fa.l,d0 / addi.l #6,d0.
	if w(initStart) != 0x2039 || l(initStart+2) != soundBase || w(initStart+6) != 0x0680 || l(initStart+8) != 6 {
		return nil, fmt.Errorf("unexpected sound table setup at $%X", initStart)
	}
	table := make([]Entry, maxEntries)
	n := 0
	cur := -1
	for pc := initStart + 12; pc < initEnd; {
		switch op := w(pc); {
		case op == 0x41F9: // lea.l abs,a0
			a := l(pc + 2)
			if (a-tableAddr)%entrySize != 0 || a < tableAddr || (a-tableAddr)/entrySize >= maxEntries {
				return nil, fmt.Errorf("bad table address $%X at $%X", a, pc)
			}
			cur = (a - tableAddr) / entrySize
			if cur+1 > n {
				n = cur + 1
			}
			pc += 6
		case op == 0x2080: // move.l d0,(a0)
			pc += 2
		case op == 0x0690: // addi.l #off,(a0)
			table[cur].Offset = l(pc + 2)
			pc += 6
		case op == 0x317C: // move.w #imm,disp(a0)
			switch d := w(pc + 4); d {
			case 4:
				table[cur].Words = w(pc + 2)
			case 6:
				table[cur].Period = w(pc + 2)
			default:
				return nil, fmt.Errorf("unexpected displacement %d at $%X", d, pc)
			}
			pc += 6
		case op == 0x117C && w(pc+4) == 8: // move.b #vol,8(a0)
			table[cur].Volume = w(pc+2) & 0xFF
			pc += 6
		case op == 0x4E75: // rts
			return table[:n], nil
		default:
			return nil, fmt.Errorf("unexpected opcode $%04X at $%X", op, pc)
		}
	}
	return table[:n], nil
}
