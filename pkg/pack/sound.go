package pack

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/karlbe/gnistaEngine/pkg/audio"
)

// Sounds loads sounds/N.wav as effect N. A file is mono or stereo PCM, 8 or 16 bits, at any
// rate; it is played back on one of the four channels like the original's effects. The
// manifest's sound_volumes gives an effect a volume from 0 to 64 (default 64).
func (p *Pack) Sounds() (map[int]audio.Entry, error) {
	es, err := os.ReadDir(p.path("sounds"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[int]audio.Entry{}
	for _, e := range es {
		name, ok := strings.CutSuffix(e.Name(), ".wav")
		id, err := strconv.Atoi(name)
		if !ok || err != nil || id < 0 || id >= 32 {
			continue
		}
		b, err := os.ReadFile(p.path("sounds", e.Name()))
		if err != nil {
			return nil, err
		}
		data, rate, err := decodeWAV(b)
		if err != nil {
			return nil, fmt.Errorf("sounds/%s: %w", e.Name(), err)
		}
		vol := 64
		if v, ok := p.Manifest.SoundVolumes[name]; ok {
			vol = v
		}
		if vol < 0 || vol > 64 {
			return nil, fmt.Errorf("sound_volumes[%s] = %d, want 0-64", name, vol)
		}
		if len(data)%2 == 1 {
			data = append(data, 0)
		}
		out[id] = audio.Entry{Words: len(data) / 2, Period: (audio.PALClock + rate/2) / rate, Volume: vol, Data: data}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// SoundIDs lists the effect numbers of a sound map, ascending.
func SoundIDs(m map[int]audio.Entry) []int {
	var ids []int
	for id := range m {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// decodeWAV reads PCM audio and returns it as mono signed 8-bit samples.
func decodeWAV(b []byte) ([]int8, int, error) {
	w, err := parseWAV(b)
	if err != nil {
		return nil, 0, err
	}
	out := make([]int8, len(w.pcm)/(w.channels*w.bits/8))
	for i := range out {
		sum := 0
		for c := 0; c < w.channels; c++ {
			sum += int(w.sample(i, c)) >> 8
		}
		out[i] = int8(sum / w.channels)
	}
	return out, w.rate, nil
}

// wavInfo is the format and sample data of a PCM WAV file.
type wavInfo struct {
	channels, bits, rate int
	pcm                  []byte
}

func parseWAV(b []byte) (wavInfo, error) {
	var w wavInfo
	le := binary.LittleEndian
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return w, fmt.Errorf("not a WAV file")
	}
	haveFmt := false
	for p := 12; p+8 <= len(b); {
		id, n := string(b[p:p+4]), int(le.Uint32(b[p+4:]))
		body := b[p+8:]
		if n > len(body) {
			n = len(body)
		}
		body = body[:n]
		switch id {
		case "fmt ":
			if n < 16 {
				return w, fmt.Errorf("short fmt chunk")
			}
			if f := le.Uint16(body); f != 1 {
				return w, fmt.Errorf("format %d: only uncompressed PCM is supported", f)
			}
			w.channels, w.rate, w.bits = int(le.Uint16(body[2:])), int(le.Uint32(body[4:])), int(le.Uint16(body[14:]))
			haveFmt = true
		case "data":
			w.pcm = body
		}
		p += 8 + n + n&1
	}
	if !haveFmt || w.pcm == nil {
		return w, fmt.Errorf("missing fmt or data chunk")
	}
	if (w.channels != 1 && w.channels != 2) || (w.bits != 8 && w.bits != 16) || w.rate < 1000 || w.rate > 96000 {
		return w, fmt.Errorf("%d channels, %d bits, %d Hz: want mono or stereo, 8 or 16 bits, 1000-96000 Hz", w.channels, w.bits, w.rate)
	}
	return w, nil
}

// sample returns channel c of frame i as a signed 16-bit value.
func (w wavInfo) sample(i, c int) int16 {
	frame := w.channels * w.bits / 8
	o := i*frame + c*w.bits/8
	if w.bits == 8 {
		return int16(int(w.pcm[o])-128) << 8
	}
	return int16(binary.LittleEndian.Uint16(w.pcm[o:]))
}

// Music loads music/title.wav (played once on the title) and music/game.wav (played over and
// over in the game). The manifest's music_volume (0-1, default 0.6) sets the level.
func (p *Pack) Music() (title, game *audio.Track, err error) {
	load := func(name string) (*audio.Track, error) {
		b, err := os.ReadFile(p.path("music", name+".wav"))
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		w, err := parseWAV(b)
		if err != nil {
			return nil, fmt.Errorf("music/%s.wav: %w", name, err)
		}
		n := len(w.pcm) / (w.channels * w.bits / 8)
		t := &audio.Track{Rate: w.rate, PCM: make([]int16, 2*n), Gain: p.Manifest.MusicVolume}
		for i := 0; i < n; i++ {
			l, r := w.sample(i, 0), w.sample(i, w.channels-1)
			t.PCM[2*i], t.PCM[2*i+1] = l, r
		}
		return t, nil
	}
	if title, err = load("title"); err != nil {
		return nil, nil, err
	}
	if game, err = load("game"); err != nil {
		return nil, nil, err
	}
	return title, game, nil
}
