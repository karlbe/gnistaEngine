// Command solve plays the game with a bot and records the inputs that win it, as a fixture the
// playthrough test replays. It needs the original's files in assets-local.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/game"
	"github.com/karlbe/gnistaEngine/pkg/solve"
)

func main() {
	root := flag.String("assets", "assets-local", "asset root directory")
	graphPath := flag.String("graph", "", "level graph file (default <assets>/solve/graph.json; built if missing)")
	out := flag.String("out", "pkg/solve/testdata/playthrough.txt", "where to write the fixture")
	reserve := flag.Int("reserve", 16000, "game clock frames to keep for the way to the bomb; the rest is spent visiting more rooms (0: only what the bomb needs)")
	flag.Parse()

	l := content.Loader{Root: *root}
	ext, err := l.Extracted()
	if err != nil {
		log.Fatal(err)
	}
	prog, err := l.Program()
	if err != nil {
		log.Fatal(err)
	}
	level, err := l.Level()
	if err != nil {
		log.Fatal(err)
	}
	c := game.Content{Program: prog, Level: level}
	newBot := func() *solve.Bot { return solve.NewBot(game.New(c, ext.StartX, ext.StartY)) }

	if *graphPath == "" {
		*graphPath = filepath.Join(*root, "solve", "graph.json")
	}
	g, err := solve.LoadGraph(*graphPath)
	if err != nil {
		log.Printf("no graph (%v); building it", err)
		g = solve.BuildGraph(newBot(), func(f string, a ...any) { log.Printf(f, a...) })
		os.MkdirAll(filepath.Dir(*graphPath), 0o755)
		if err := g.Save(*graphPath); err != nil {
			log.Fatal(err)
		}
	}
	start := time.Now()
	b := newBot()
	logf := func(f string, a ...any) { log.Printf(f, a...) }
	if *reserve > 0 {
		err = solve.SolveAll(b, g, *reserve, logf)
	} else {
		err = solve.Solve(b, g, logf)
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("won: %d ticks, %d rooms, in %v", len(b.Log), b.S.VisitedRooms(), time.Since(start).Round(time.Second))
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, []byte(solve.EncodeInputs(b.Log)+"\n"), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", *out)
}
