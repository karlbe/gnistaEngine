package audio

// Track is a recorded piece of music (stereo, 16 bit) that a pack can use in place of the
// original's tracker songs. The mixer plays it over the effects.
type Track struct {
	Rate int     // frames per second
	PCM  []int16 // left, right, left, right...
	Gain float64 // 0 means 0.6
}

// Frames is the length in stereo frames.
func (t *Track) Frames() int { return len(t.PCM) / 2 }

// Ticks is the length in 50 Hz game ticks, rounded up.
func (t *Track) Ticks() int { return (t.Frames()*50 + t.Rate - 1) / t.Rate }

type trackState struct {
	t       *Track
	pos     float64 // in frames
	playing bool
	loop    bool
	muted   bool
}

// PlayTrack starts a track from the beginning, replacing any that plays.
func (m *Mixer) PlayTrack(t *Track, loop bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.track = trackState{t: t, playing: t != nil && len(t.PCM) >= 2, loop: loop}
}

// MuteTrack silences the track without losing its place (the game does this while an enemy
// is on screen).
func (m *Mixer) MuteTrack(mute bool) {
	m.mu.Lock()
	m.track.muted = mute
	m.mu.Unlock()
}

// StopTrack ends the track.
func (m *Mixer) StopTrack() {
	m.mu.Lock()
	m.track = trackState{}
	m.mu.Unlock()
}

// nextTrack returns the track's next frame (already scaled to the mixer's sums), or zeros.
func (m *Mixer) nextTrack() (l, r float64) {
	s := &m.track
	if !s.playing {
		return 0, 0
	}
	t := s.t
	n := t.Frames()
	i := int(s.pos)
	if i >= n {
		if !s.loop {
			s.playing = false
			return 0, 0
		}
		s.pos -= float64(n)
		i = int(s.pos)
	}
	j := i + 1
	if j >= n {
		if s.loop {
			j = 0
		} else {
			j = i
		}
	}
	f := s.pos - float64(i)
	g := t.Gain
	if g == 0 {
		g = 0.6
	}
	s.pos += float64(t.Rate) / float64(m.rate)
	if s.muted {
		return 0, 0
	}
	// A side of effects sums to about +-32768/2 in these units; a full-scale track gets the
	// same headroom.
	l = (float64(t.PCM[2*i])*(1-f) + float64(t.PCM[2*j])*f) * g / 2
	r = (float64(t.PCM[2*i+1])*(1-f) + float64(t.PCM[2*j+1])*f) * g / 2
	return l, r
}
