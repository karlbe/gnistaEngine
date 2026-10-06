package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/karlbe/gnistaEngine/pkg/audio"
	"github.com/karlbe/gnistaEngine/pkg/game"
	"github.com/karlbe/gnistaEngine/pkg/solve"
)

// dumpReplay plays a recorded game (the file go run ./cmd/solve writes) as fast as the
// simulation goes, without a window or sound, and saves every n-th frame as a PNG in dir. The
// pictures show the original's graphics, so they stay on this machine (the directory belongs
// under assets-local, which is not committed).
func (a *app) dumpReplay(file, dir string, every int, wav string) error {
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	inputs, err := solve.DecodeInputs(string(raw))
	if err != nil {
		return err
	}
	if every < 1 {
		every = 1
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// The recording uses the invulnerability cheat as its first input.
	a.setImprovements(true)
	a.render.Minimap = false
	type frame struct {
		n   int
		pix []byte
	}
	work := make(chan frame, 64)
	var wg sync.WaitGroup
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	var firstErr error
	var mu sync.Mutex
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range work {
				img := &image.RGBA{Pix: f.pix, Stride: 4 * 320, Rect: image.Rect(0, 0, 320, 256)}
				out, err := os.Create(filepath.Join(dir, fmt.Sprintf("frame_%06d.png", f.n)))
				if err == nil {
					err = enc.Encode(out, img)
					out.Close()
				}
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
			}
		}()
	}
	var sound bytes.Buffer
	start := time.Now()
	saved := 0
	a.startAmbient()
	for i, in := range inputs {
		a.state.Step(in)
		if a.mixer != nil { // one tick of sound: 44100 / 50 frames
			a.tickMusic()
			a.playSounds()
			a.mixer.SetPaused(a.state.Paused)
			buf := make([]byte, 4*audio.SampleRate/game.TicksPerSecond)
			a.mixer.Read(buf)
			sound.Write(buf)
		}
		if i%every != 0 && i != len(inputs)-1 {
			continue
		}
		a.render.Draw(nil, a.state, a.content.Level)
		work <- frame{i, append([]byte(nil), a.render.Pixels()...)}
		saved++
	}
	close(work)
	wg.Wait()
	if a.mixer != nil {
		if err := writeWAV(wav, sound.Bytes()); err != nil {
			return err
		}
	}
	fmt.Printf("%d ticks played, %d pictures written to %s in %v\n", len(inputs), saved, dir, time.Since(start).Round(time.Second))
	return firstErr
}

// writeWAV saves signed 16-bit stereo frames at the mixer's rate.
func writeWAV(path string, pcm []byte) error {
	var h bytes.Buffer
	le := binary.LittleEndian
	put32 := func(v uint32) { binary.Write(&h, le, v) }
	put16 := func(v uint16) { binary.Write(&h, le, v) }
	h.WriteString("RIFF")
	put32(uint32(36 + len(pcm)))
	h.WriteString("WAVEfmt ")
	put32(16)
	put16(1) // PCM
	put16(2) // channels
	put32(audio.SampleRate)
	put32(audio.SampleRate * 4)
	put16(4)
	put16(16)
	h.WriteString("data")
	put32(uint32(len(pcm)))
	h.Write(pcm)
	return os.WriteFile(path, h.Bytes(), 0o644)
}
