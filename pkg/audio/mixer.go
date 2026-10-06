package audio

import "sync"

// PALClock is Paula's sample clock on a PAL machine; a channel's rate is PALClock / period.
const PALClock = 3546895

// DefaultSeparation keeps the Amiga's left/right split but lets each side leak into the
// other, as emulators usually do; hard panning puts all footsteps and shots in one ear.
const DefaultSeparation = 0.5

type voice struct {
	entry   Entry
	loaded  bool    // registers set ($64FE and friends)
	playing bool    // DMA on
	loop    bool    // DMA left on: Paula restarts the sample until the channel is reloaded
	pos     float64 // in samples
}

// Mixer is four Paula channels mixed to 16-bit stereo. Like the Amiga, channels 0 and 3
// go to the left and 1 and 2 to the right.
//
// The original starts an effect in two steps: a script op loads the channel's registers
// with DMA off ($64FE/$652C/$655A/$6588 -> Load), then a later op switches DMA on
// (Start). One that also sets the length to one word ($21xx, ops 85-88) plays the sample
// once; one that does not (op89) loops it until the channel is loaded again.
type Mixer struct {
	mu    sync.Mutex
	table []Entry
	data  []int8
	rate  int
	v     [4]voice
	track trackState // recorded music from a pack, if any

	// Separation is how far the channels are panned: 1 is the Amiga's hard left/right
	// (channels 0 and 3 left, 1 and 2 right), 0 is mono.
	Separation float64

	// Paused holds every channel where it is and outputs silence.
	Paused bool
}

// NewMixer makes a mixer producing rate Hz output from the effects table and sample data.
func NewMixer(table []Entry, data []int8, rate int) *Mixer {
	return &Mixer{table: table, data: data, rate: rate, Separation: DefaultSeparation}
}

// SetGain sets the gain of effect id (see Entry.Gain).
func (m *Mixer) SetGain(id int, g float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id >= 0 && id < len(m.table) {
		m.table[id].Gain = g
	}
}

// LoadEntry sets channel ch to an entry that is not in the effects table (a note of the
// music), with DMA off like Load.
func (m *Mixer) LoadEntry(ch int, e Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.v[ch&3] = voice{entry: e, loaded: true}
}

// Load sets channel ch to effect id with DMA off, which cuts whatever it was playing.
func (m *Mixer) Load(ch int, id uint8) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := &m.v[ch&3]
	*v = voice{}
	if int(id) < len(m.table) {
		v.entry, v.loaded = m.table[id], true
	}
}

// Start switches channel ch's DMA on. With once, the sample plays through and the channel
// falls silent (the original's one-word loop); otherwise it repeats.
func (m *Mixer) Start(ch int, once bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := &m.v[ch&3]
	if !v.loaded {
		return
	}
	if !v.playing {
		v.playing, v.pos = true, 0
	}
	v.loop = !once
}

// SetPaused pauses or resumes all channels.
func (m *Mixer) SetPaused(p bool) {
	m.mu.Lock()
	m.Paused = p
	m.mu.Unlock()
}

// Stop silences every channel.
func (m *Mixer) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.v = [4]voice{}
	m.track = trackState{}
}

// Read fills p with signed 16-bit little-endian stereo frames, as an io.Reader for an
// audio player. It always fills p completely.
func (m *Mixer) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(p) / 4
	if m.Paused {
		clear(p[:4*n])
		return 4 * n, nil
	}
	for i := 0; i < n; i++ {
		var l, r float64
		near, far := (1+m.Separation)/2, (1-m.Separation)/2
		for c := range m.v {
			s := float64(m.next(&m.v[c]))
			if c == 0 || c == 3 {
				l, r = l+s*near, r+s*far
			} else {
				l, r = l+s*far, r+s*near
			}
		}
		// A side sums two channels of up to +-128*64; scale into 16 bits.
		tl, tr := m.nextTrack()
		put(p[4*i:], int(l*2+tl*2))
		put(p[4*i+2:], int(r*2+tr*2))
	}
	return 4 * n, nil
}

func put(b []byte, v int) {
	if v > 32767 {
		v = 32767
	} else if v < -32768 {
		v = -32768
	}
	b[0], b[1] = byte(v), byte(v>>8)
}

// next returns the channel's next output sample, scaled by its volume (-8192..8128).
func (m *Mixer) next(v *voice) int {
	if !v.playing {
		return 0
	}
	e := v.entry
	data := m.data
	if e.Data != nil {
		data = e.Data
	}
	n := e.Words * 2
	if e.Offset+n > len(data) {
		n = len(data) - e.Offset
	}
	if n <= 0 || e.Period <= 0 {
		v.playing = false
		return 0
	}
	i := int(v.pos)
	if i >= n {
		if !v.loop {
			v.playing = false
			return 0
		}
		v.pos -= float64(n)
		i = int(v.pos)
	}
	s := int(data[e.Offset+i]) * e.Volume
	if e.Gain != 0 {
		s = int(float64(s) * e.Gain)
	}
	v.pos += float64(PALClock) / float64(e.Period) / float64(m.rate)
	return s
}
