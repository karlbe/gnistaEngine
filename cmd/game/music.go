package main

import (
	"github.com/karlbe/gnistaEngine/pkg/audio"
	"github.com/karlbe/gnistaEngine/pkg/content"
)

// loadMusic builds the music player: the song file NSIA, the title song's instruments
// (NSIMusicSound) and the three samples of the in-game track (IAZ, DAS, DBY). Its voices are
// silent until the mixer is started.
func loadMusic(l content.Loader, code []byte) (*audio.Tracker, error) {
	nsia, err := l.Disk("NSIA")
	if err != nil {
		return nil, err
	}
	song, err := audio.ParseSong(nsia)
	if err != nil {
		return nil, err
	}
	instruments, err := audio.ParseMusicInstruments(code)
	if err != nil {
		return nil, err
	}
	bank, err := l.Sample("NSIMusicSound")
	if err != nil {
		return nil, err
	}
	for i := range instruments {
		instruments[i].Data = bank.Data
	}
	words, err := audio.ParseAmbientLengths(code)
	if err != nil {
		return nil, err
	}
	var ambient []audio.Entry
	for i, name := range []string{"IAZ", "DAS", "DBY"} {
		s, err := l.Sample(name)
		if err != nil {
			return nil, err
		}
		ambient = append(ambient, audio.Entry{Words: words[i], Data: s.Data})
	}
	return audio.NewTracker(song, instruments, ambient, audio.Silent{}), nil
}
