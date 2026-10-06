package solve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/game"
)

// TestPlaythrough plays the game from the start to the end by replaying a recorded input
// sequence (testdata/playthrough.txt, made by go run ./cmd/solve): all five cards and all three
// weapons are collected, the bomb room's door is blown open, the bomb is defused by cutting the
// right wire, and the ending is watched to its last page. Nothing else is checked against the
// original; what it proves is that the whole game can be finished and that the simulation is
// deterministic, since the replay only holds if every tick behaves exactly as when it was made.
func TestPlaythrough(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "playthrough.txt"))
	if err != nil {
		t.Skip("no recorded playthrough; run go run ./cmd/solve")
	}
	inputs, err := DecodeInputs(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	l := content.Loader{Root: filepath.Join("..", "..", "assets-local")}
	e, err := l.Extracted()
	if err != nil {
		t.Skip(err)
	}
	level, err := l.Level()
	if err != nil {
		t.Skip(err)
	}
	prog, err := l.Program()
	if err != nil {
		t.Fatal(err)
	}
	s := game.New(game.Content{Program: prog, Level: level}, e.StartX, e.StartY)
	s.SetImprovements(true) // the recording uses the invulnerability cheat as its first input

	for i, in := range inputs {
		s.Step(in)
		if s.Halted != nil {
			t.Fatalf("tick %d: %v", i, s.Halted)
		}
	}
	g := &s.Engine.G
	t.Logf("%d inputs, %d rooms visited, cards %05b, weapons %03b, outcome %d, clock %d%d:%d%d",
		len(inputs), s.VisitedRooms(), g.Cards, g.WeaponsOwned, g.Outcome, s.Clock.Min10, s.Clock.Min, s.Clock.Sec10, s.Clock.Sec)
	if g.Outcome != game.OutcomeWin {
		t.Fatalf("the game ended with outcome %d, want %d (the bomb defused)", g.Outcome, game.OutcomeWin)
	}
	if !s.Finished() {
		t.Error("the ending was not watched to its end")
	}
	if g.Cards != 0x1F {
		t.Errorf("cards %05b, want all five", g.Cards)
	}
	if g.WeaponsOwned != 7 {
		t.Errorf("weapons %03b, want all three", g.WeaponsOwned)
	}
	if s.VisitedRooms() < 25 {
		t.Errorf("only %d rooms visited", s.VisitedRooms())
	}
	if left := s.Clock.FramesLeft(); left < 0 || left > 20000 {
		t.Errorf("%d clock frames left at the end", left)
	}
}
