package solve

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/game"
)

func newBot(t *testing.T) *Bot {
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
	return NewBot(game.New(game.Content{Program: prog, Level: level}, e.StartX, e.StartY))
}

func TestFirstRoom(t *testing.T) {
	b := newBot(t)
	b.Settle(200)
	start := time.Now()
	path, err := Search(b, func(n *Bot) bool { return len(n.Visited) > 0 }, 5000)
	t.Logf("search took %v", time.Since(start))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("path: %s", path)
	if !b.Do(path) || len(b.Visited) == 0 {
		t.Fatal("the path did not enter a room")
	}
	t.Logf("entered rooms %v after %d ticks", b.Visited, len(b.Log))
}

func TestAllRoomsSoFar(t *testing.T) {
	if os.Getenv("SOLVE") == "" {
		t.Skip("set SOLVE=1 to run the solver's exploration tests")
	}
	b := newBot(t)
	b.Invulnerable()
	b.Settle(200)
	start := time.Now()
	for leg := 0; leg < 80; leg++ {
		before := len(b.Visited)
		path, err := Search(b, func(n *Bot) bool { return len(n.Visited) > before }, 20000)
		if err != nil {
			t.Logf("leg %d: stuck: %v", leg, err)
			break
		}
		if !b.Do(path) {
			t.Logf("leg %d: the path failed", leg)
			break
		}
		g := &b.S.Engine.G
		t.Logf("leg %2d: %2d rooms, tick %5d, path %3d macros, cards %05b weapons %03b charges %d lives %d invulnerable=%v/%v (%v)", leg, len(b.Visited), b.S.Tick, len(path), g.Cards, g.WeaponsOwned, g.Charges, g.Lives, b.S.Invulnerable, g.Invulnerable, time.Since(start).Round(time.Millisecond))
	}
	t.Logf("visited %d rooms in %d ticks", len(b.Visited), len(b.Log))
}

func TestSolveTheGame(t *testing.T) {
	if os.Getenv("SOLVE") == "" {
		t.Skip("set SOLVE=1 to run the solver (go run ./cmd/solve records its result)")
	}
	b := newBot(t)
	g, err := LoadGraph(graphFile)
	if err != nil {
		t.Skip("no level graph; run TestBuildGraph with SOLVE_GRAPH=1")
	}
	start := time.Now()
	err = Solve(b, g, t.Logf)
	t.Logf("took %v, %d ticks logged, tick %d, rooms %d, outcome %d, finished %v", time.Since(start).Round(time.Second), len(b.Log), b.S.Tick, len(b.Visited), b.S.Engine.G.Outcome, b.S.Finished())
	if err != nil {
		t.Fatal(err)
	}
}

const graphFile = "../../assets-local/solve/graph.json"

// TestBuildGraph explores the whole level with a fully equipped player and saves the map. It
// takes a while, so it only runs when SOLVE_GRAPH is set.
func TestBuildGraph(t *testing.T) {
	if os.Getenv("SOLVE_GRAPH") == "" {
		t.Skip("set SOLVE_GRAPH=1 to build the level graph")
	}
	b := newBot(t)
	b.Invulnerable()
	b.Settle(300)
	start := time.Now()
	g := BuildGraph(b, t.Logf)
	t.Logf("%d positions, %d doors, took %v", len(g.Edges), len(g.Doors), time.Since(start).Round(time.Second))
	os.MkdirAll(filepath.Dir(graphFile), 0o755)
	if err := g.Save(graphFile); err != nil {
		t.Fatal(err)
	}
}

func TestSolveAllRooms(t *testing.T) {
	if os.Getenv("SOLVE") == "" {
		t.Skip("set SOLVE=1 to run the solver")
	}
	b := newBot(t)
	g, err := LoadGraph(graphFile)
	if err != nil {
		t.Skip("no level graph")
	}
	start := time.Now()
	err = SolveAll(b, g, 16000, t.Logf)
	t.Logf("took %v, tick %d, rooms %d (visited flags %d), outcome %d, finished %v", time.Since(start).Round(time.Second), b.S.Tick, len(b.Visited), b.S.VisitedRooms(), b.S.Engine.G.Outcome, b.S.Finished())
	if err != nil {
		t.Fatal(err)
	}
}
