package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTable(t *testing.T) {
	code, err := os.ReadFile(filepath.Join("..", "..", "assets-local", "extracted", "code.bin"))
	if err != nil {
		t.Skip("assets-local/extracted missing")
	}
	tab, err := ParseTable(code)
	if err != nil {
		t.Fatal(err)
	}
	if len(tab) != 20 {
		t.Fatalf("got %d entries, want 20", len(tab))
	}
	for i, e := range tab {
		t.Logf("%2d %+v", i, e)
		if e.Words == 0 || e.Period == 0 || e.Volume == 0 || e.Offset+2*e.Words > 82800 {
			t.Errorf("entry %d looks wrong: %+v", i, e)
		}
	}
	if tab[1].Offset != 2*tab[0].Words {
		t.Errorf("entry 1 should follow entry 0: %+v %+v", tab[0], tab[1])
	}
}

func TestMixerOnceAndLoop(t *testing.T) {
	data := make([]int8, 100)
	for i := range data {
		data[i] = 64
	}
	m := NewMixer([]Entry{{Offset: 0, Words: 50, Period: 161, Volume: 64}}, data, 22050) // 22 kHz
	buf := make([]byte, 4*1000)
	m.Load(0, 0)
	m.Read(buf)
	if buf[0] != 0 || buf[1] != 0 {
		t.Error("loaded channel with DMA off must be silent")
	}
	m.Start(0, true)
	m.Read(buf)
	if buf[0] == 0 && buf[1] == 0 {
		t.Error("started channel is silent")
	}
	if buf[4*999] != 0 || buf[4*999+1] != 0 {
		t.Error("one-shot sample must have ended")
	}
	m.Load(0, 0)
	m.Start(0, false)
	m.Read(buf)
	if buf[4*999] == 0 && buf[4*999+1] == 0 {
		t.Error("looping sample must still play")
	}
	m.Load(0, 0) // reload cuts it
	m.Read(buf)
	if buf[0] != 0 || buf[1] != 0 {
		t.Error("reload must silence the channel")
	}
}

func TestSetGain(t *testing.T) {
	data := make([]int8, 100)
	for i := range data {
		data[i] = 10
	}
	m := NewMixer([]Entry{{Words: 50, Period: 161, Volume: 20}}, data, 22050)
	m.Separation = 1
	read := func() int16 {
		buf := make([]byte, 4*8)
		m.Load(0, 0)
		m.Start(0, false)
		m.Read(buf)
		return int16(buf[0]) | int16(buf[1])<<8
	}
	plain := read()
	m.SetGain(0, 4)
	if loud := read(); loud != plain*4 {
		t.Errorf("a gain of 4 gave %d, want %d", loud, plain*4)
	}
	m.SetGain(99, 4) // out of range: ignored
}

func TestTrack(t *testing.T) {
	m := NewMixer(nil, nil, 44100)
	pcm := make([]int16, 2*100) // 100 frames: a constant level, left only
	for i := 0; i < 100; i++ {
		pcm[2*i] = 10000
	}
	tr := &Track{Rate: 44100, PCM: pcm, Gain: 1}
	if got := tr.Ticks(); got != 1 {
		t.Errorf("100 frames at 44100 Hz are %d ticks, want 1", got)
	}
	read := func(frames int) []byte {
		b := make([]byte, 4*frames)
		m.Read(b)
		return b
	}
	left := func(b []byte, i int) int { return int(int16(uint16(b[4*i]) | uint16(b[4*i+1])<<8)) }
	right := func(b []byte, i int) int { return int(int16(uint16(b[4*i+2]) | uint16(b[4*i+3])<<8)) }

	m.PlayTrack(tr, false)
	b := read(150)
	if l := left(b, 10); l < 9000 || l > 11000 {
		t.Errorf("left level %d, want about 10000", l)
	}
	if r := right(b, 10); r != 0 {
		t.Errorf("right level %d, want 0", r)
	}
	if l := left(b, 120); l != 0 {
		t.Errorf("after the end the level is %d, want 0", l)
	}

	m.PlayTrack(tr, true)
	m.MuteTrack(true)
	if l := left(read(50), 10); l != 0 {
		t.Errorf("a muted track plays %d", l)
	}
	m.MuteTrack(false)
	if l := left(read(250), 240); l < 9000 {
		t.Errorf("a looping track has gone quiet: %d", l)
	}
	m.Stop()
	if l := left(read(10), 5); l != 0 {
		t.Errorf("Stop left the track playing: %d", l)
	}
}
