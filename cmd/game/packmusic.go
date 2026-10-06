package main

import "github.com/karlbe/gnistaEngine/pkg/audio"

// musicPlayer is what the game needs of the music: the original's tracker or, with a pack,
// recorded tracks instead of it.
type musicPlayer interface {
	StartSong()
	StartAmbient()
	SetStopped(bool)
	Finished() bool
	Tick()
}

// attacher is implemented by players that need the mixer once sound is on.
type attacher interface{ Attach(m *audio.Mixer) }

// packMusic plays a pack's recorded tracks where it has them and the original's tracker
// where it does not. It counts game ticks itself, so the title advances the same with or
// without sound.
type packMusic struct {
	orig        musicPlayer // the original's tracker, or silence
	title, game *audio.Track
	mixer       *audio.Mixer

	on       *audio.Track // the recorded track in use, nil: the tracker is
	left     int          // ticks of the title track left; 0 for a looping one
	finished bool
}

func (p *packMusic) Attach(m *audio.Mixer) {
	p.mixer = m
	if at, ok := p.orig.(attacher); ok {
		at.Attach(m)
	} else if tr, ok := p.orig.(*audio.Tracker); ok {
		tr.V = m
	}
}

func (p *packMusic) StartSong() {
	if p.title == nil {
		p.on = nil
		p.orig.StartSong()
		return
	}
	p.orig.SetStopped(true)
	p.on, p.left, p.finished = p.title, p.title.Ticks(), false
	if p.mixer != nil {
		p.mixer.PlayTrack(p.title, false)
	}
}

func (p *packMusic) StartAmbient() {
	if p.game == nil {
		p.on = nil
		p.orig.StartAmbient()
		return
	}
	p.orig.SetStopped(true)
	p.on, p.left, p.finished = p.game, 0, false
	if p.mixer != nil {
		p.mixer.PlayTrack(p.game, true)
	}
}

func (p *packMusic) SetStopped(s bool) {
	switch {
	case p.on == nil:
		p.orig.SetStopped(s)
	case p.on == p.game: // the game track pauses while an enemy is on screen
		if p.mixer != nil {
			p.mixer.MuteTrack(s)
		}
	}
}

func (p *packMusic) Finished() bool {
	if p.on == nil {
		return p.orig.Finished()
	}
	return p.finished
}

func (p *packMusic) Tick() {
	if p.on == nil {
		p.orig.Tick()
		return
	}
	if p.left > 0 {
		if p.left--; p.left == 0 {
			p.finished = true
		}
	}
}

// nopMusic is the music of a game that has none.
type nopMusic struct{}

func (nopMusic) StartSong()      {}
func (nopMusic) StartAmbient()   {}
func (nopMusic) SetStopped(bool) {}
func (nopMusic) Finished() bool  { return true }
func (nopMusic) Tick()           {}
