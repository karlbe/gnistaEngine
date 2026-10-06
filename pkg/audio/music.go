package audio

import (
	"encoding/binary"
	"fmt"
)

// The original's music player runs in the vertical blank interrupt ($7566 calls $96DE) as a
// four-voice tracker over the song file NSIA. See docs/re/music.md.

// Layout of NSIA, from the setup code at $9B5C and $9BA0.
const (
	eventSize    = 6      // flags, instrument, period (u16), duration, volume
	eventsLen    = 0x2328 // the events come first
	patternCount = 100    // the patterns are runs of events; the one with flags == 2 ends a run
	orderOffset  = 0x24B8 // then four order lists of orderLen bytes (pattern numbers, $FF ends), one per voice
	orderLen     = 0x9C
	voices       = 4
	endOfPattern = 2 // flag bit 1 of an event: the last event of its pattern
	endOfSong    = 0xFF
)

// Song is a parsed NSIA.
type Song struct {
	data     []byte
	patterns [patternCount]int // byte offset of each pattern's first event
	orders   [voices][]byte
}

// ParseSong splits NSIA into patterns and order lists the way $9B5C does: a pattern runs up
// to and including the first event whose flags byte is exactly 2.
func ParseSong(nsia []byte) (*Song, error) {
	if len(nsia) < orderOffset+voices*orderLen {
		return nil, fmt.Errorf("NSIA is %d bytes, too short", len(nsia))
	}
	s := &Song{data: nsia}
	pc, start := 0, 0
	for n := 0; n < patternCount; n++ {
		for pc+eventSize <= eventsLen && nsia[pc] != 2 {
			pc += eventSize
		}
		if pc+eventSize > eventsLen {
			return nil, fmt.Errorf("pattern %d has no end", n)
		}
		s.patterns[n] = start
		pc += eventSize
		start = pc
	}
	for v := range s.orders {
		s.orders[v] = nsia[orderOffset+v*orderLen : orderOffset+(v+1)*orderLen]
	}
	return s, nil
}

// ParseMusicInstruments reads the instrument table that $9C52 builds for NSIMusicSound:
//
//	move.l d0,(a0) / [addi.l #off,(a0)] / move.w #words,4(a0) / adda.l #10,a0
//
// with d0 the start of the sample data, so an instrument is an offset and a length in words.
func ParseMusicInstruments(code []byte) ([]Entry, error) {
	const start, end = 0x9C7C, 0x9D4A
	if len(code) < end {
		return nil, fmt.Errorf("code hunk too short")
	}
	w := func(a int) int { return int(binary.BigEndian.Uint16(code[a:])) }
	l := func(a int) int { return int(binary.BigEndian.Uint32(code[a:])) }
	var out []Entry
	cur := Entry{}
	for pc := start; pc < end; {
		switch op := w(pc); op {
		case 0x41F9, 0x2039, 0x0680, 0xD1FC: // lea table,a0 / move.l base,d0 / addi.l #6,d0 / adda.l #10,a0
			if op == 0xD1FC {
				out = append(out, cur)
				cur = Entry{}
			}
			pc += 6
		case 0x2080: // move.l d0,(a0)
			pc += 2
		case 0x0690: // addi.l #off,(a0)
			cur.Offset = l(pc + 2)
			pc += 6
		case 0x317C: // move.w #words,4(a0)
			cur.Words = w(pc + 2)
			pc += 6
		case 0x4E75: // rts
			return append(out, cur), nil
		default:
			return nil, fmt.Errorf("unexpected opcode $%04X at $%X", op, pc)
		}
	}
	return append(out, cur), nil
}

// ParseAmbientLengths reads the lengths in words of the three samples $95EA puts in the
// instrument table for the in-game track (IAZ, DAS, DBY).
func ParseAmbientLengths(code []byte) ([]int, error) {
	const start, end = 0x9698, 0x96DE
	if len(code) < end {
		return nil, fmt.Errorf("code hunk too short")
	}
	var out []int
	for pc := start; pc < end; pc += 2 {
		if binary.BigEndian.Uint16(code[pc:]) == 0x317C && pc+6 <= end { // move.w #n,disp(a0)
			out = append(out, int(binary.BigEndian.Uint16(code[pc+2:])))
			pc += 4
		}
	}
	if len(out) != 3 {
		return nil, fmt.Errorf("found %d ambient samples, want 3", len(out))
	}
	return out, nil
}

// Voices is what the tracker drives: the four Paula channels, set up in two steps like the
// original's registers (see Mixer.Load and Mixer.Start).
type Voices interface {
	LoadEntry(ch int, e Entry) // registers set, DMA off
	Start(ch int, once bool)   // DMA on
}

// Silent is Voices that does nothing, for running the music's timing without sound.
type Silent struct{}

func (Silent) LoadEntry(int, Entry) {}
func (Silent) Start(int, bool)      {}

// Tracker is the music player. The song plays one row every five ticks of the 50 Hz
// vertical blank; a voice holds its note for the duration of the event.
type Tracker struct {
	V Voices

	song    *Song
	title   []Entry // the instruments of the title song (NSIMusicSound)
	ambient []Entry // the three samples of the in-game track
	inst    []Entry // the table in use ($9ABC)

	count    [voices]int // $9A82: rows left of the current note; 1 = start the next one
	event    [voices]int // $9A9C...: byte offset of the next event
	order    [voices]int // $9A8C...: next index into the voice's order list
	started  uint8       // $9A86: voices that began a note, waiting for their DMA to be switched on
	speed    int         // $9A87: ticks until the next row
	mute     uint8       // $9A89: voices the player leaves alone
	stopped  bool        // $9A88 bit 0: the song has ended, or the game stops it
	ambientM bool        // $9A8A: playing the in-game track, which starts over when it ends
}

// NewTracker makes a player for a parsed song, the title instruments and the ambient ones.
func NewTracker(song *Song, title, ambient []Entry, v Voices) *Tracker {
	return &Tracker{V: v, song: song, title: title, ambient: ambient, stopped: true}
}

// Finished reports $9A88 bit 0: the song has ended (or been stopped), and no ticks run.
func (t *Tracker) Finished() bool { return t.stopped }

// SetStopped sets or clears $9A88 bit 0. The game sets it while an enemy is on screen.
func (t *Tracker) SetStopped(s bool) { t.stopped = s }

// StartSong is $94EA: the title song from the start of every voice's order list.
func (t *Tracker) StartSong() {
	t.inst = t.title
	for v := 0; v < voices; v++ {
		t.event[v] = t.song.patterns[t.song.orders[v][0]]
		t.order[v] = 1
		t.count[v] = 1
	}
	t.started, t.speed, t.stopped, t.mute, t.ambientM = 0, 0, false, 0, false
}

// ambientStart is where $95EA enters the song: the order list position 18.
const ambientStart = 0x12

// StartAmbient is $95EA: the in-game track. Voices 0 and 1 are muted, voices 2 and 3 play
// from position 18 of their order lists, with the IAZ, DAS and DBY samples as instruments.
// It starts over by itself when it ends.
func (t *Tracker) StartAmbient() {
	t.inst = t.ambient
	for v := 2; v < voices; v++ {
		t.event[v] = t.song.patterns[t.song.orders[v][ambientStart]]
		t.order[v] = ambientStart + 1
	}
	for v := 0; v < voices; v++ {
		t.count[v] = 1
	}
	t.started, t.speed, t.stopped, t.mute, t.ambientM = 0, 0, false, 3, true
}

// Tick is one vertical blank ($96DE): voices that started a note last time get their DMA
// switched on (they play the sample once), then every fifth tick the song moves one row.
func (t *Tracker) Tick() {
	if t.stopped {
		return
	}
	for v := 0; v < voices; v++ {
		if t.started&(1<<v) != 0 {
			t.V.Start(v, true)
			t.started &^= 1 << v
		}
	}
	if t.speed != 0 {
		t.speed--
		return
	}
	t.speed = 4
	t.row()
}

// row is $976E: each voice that is not muted counts down its note, or starts the next.
func (t *Tracker) row() {
	for v := 0; v < voices; v++ {
		if t.mute&(1<<v) != 0 {
			continue
		}
		if t.count[v] != 1 {
			t.count[v]--
			continue
		}
		if !t.note(v) {
			return
		}
	}
}

// note starts voice v's next event ($980C, $98B0, $994A, $99E4) and moves it on to the next
// pattern when this was the last event of one. It reports false when the song ended, which
// the original does with a return that skips the voices after it.
func (t *Tracker) note(v int) bool {
	d := t.song.data
	e := d[t.event[v] : t.event[v]+eventSize]
	flags, instrument := e[0], int(e[1])
	period := int(binary.BigEndian.Uint16(e[2:]))
	vol := int(e[5])
	if vol == 4 {
		vol = 0
	}
	if instrument < len(t.inst) {
		ent := t.inst[instrument]
		ent.Period, ent.Volume = period, vol
		t.V.LoadEntry(v, ent)
	}
	t.count[v] = int(e[4])
	t.event[v] += eventSize
	t.started |= 1 << v
	if flags&endOfPattern == 0 {
		return true
	}
	idx := t.song.orders[v][t.order[v]]
	if idx == endOfSong && (v == 0 || v == 3) || int(idx) >= patternCount { // only the outer voices look for the end
		t.songEnd()
		return false
	}
	t.order[v]++
	t.event[v] = t.song.patterns[idx]
	return true
}

// songEnd is $97F0: the title song stops; the in-game track starts over.
func (t *Tracker) songEnd() {
	if t.ambientM {
		t.StartAmbient()
		return
	}
	t.stopped = true
}
