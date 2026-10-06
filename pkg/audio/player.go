package audio

import (
	"time"

	eaudio "github.com/hajimehoshi/ebiten/v2/audio"
)

// SampleRate is the output rate of the player.
const SampleRate = 44100

// Play starts streaming the mixer to the default output device. The returned player must
// be kept alive for as long as the sound should play.
func Play(m *Mixer) (*eaudio.Player, error) {
	ctx := eaudio.NewContext(SampleRate)
	p, err := ctx.NewPlayer(m)
	if err != nil {
		return nil, err
	}
	p.SetBufferSize(60 * time.Millisecond)
	p.Play()
	return p, nil
}
