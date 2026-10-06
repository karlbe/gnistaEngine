package solve

import (
	"container/heap"
	"encoding/json"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"
)

// Loc is a place: where the player stands and which way it faces.
type Loc struct {
	X, Y int16
	Left bool
}

// PosKey is a settled state of the player: a place and the script it waits in. The same place
// can be left in different ways depending on the script, for example after firing, when the
// weapon is up and the first direction pressed only turns.
type PosKey struct {
	Loc
	PC int32
}

// Pos is the bot's position.
func (b *Bot) Pos() PosKey {
	p := b.player()
	return PosKey{Loc{p.X, p.Y, b.facingLeft()}, int32(p.PC)}
}

// Edge is a macro from one position to another and what it costs in ticks.
type Edge struct {
	To    PosKey
	Macro string
	Ticks int32
}

// Graph is every position a fully equipped player can reach and the macros between them. It
// is a map of the level, built once, that tells the search how far a door is.
type Graph struct {
	Edges map[PosKey][]Edge
	Doors map[PosKey]int // positions where "up" enters a room, with the room
}

// MacroByName rebuilds a macro from its name.
func MacroByName(name string) (Macro, bool) {
	for _, m := range allMacros() {
		if m.Name == name {
			return m, true
		}
	}
	return Macro{}, false
}

func allMacros() []Macro {
	m := append([]Macro(nil), Moves()...)
	m = append(m, BlowLeft, BlowRight)
	for n := 1; n <= 8; n++ {
		m = append(m, LiftRide(true, n), LiftRide(false, n))
	}
	return m
}

type expansion struct {
	from  PosKey
	macro string
	bot   *Bot
	ticks int32
	room  int // a room entered by the macro, or -1
}

// BuildGraph explores the level from the bot's position, breadth first, with all macros, on
// copies of the game with everything given. It works on several cores.
func BuildGraph(start *Bot, logf func(string, ...any)) *Graph {
	g := &Graph{Edges: map[PosKey][]Edge{}, Doors: map[PosKey]int{}}
	root := start.Clone()
	root.S.GiveEverything()
	root.S.DropMapMemory()
	root.Visited = map[int]bool{}
	seen := map[PosKey]bool{root.Pos(): true}
	frontier := []*Bot{root}
	workers := runtime.NumCPU()
	t0 := time.Now()
	for level := 0; len(frontier) > 0; level++ {
		results := make(chan []expansion, len(frontier))
		var wg sync.WaitGroup
		work := make(chan *Bot)
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for f := range work {
					var out []expansion
					for _, m := range movesAt(f) {
						c := f.Clone()
						c.S.GiveEverything() // the clock must not run out
						c.Visited = map[int]bool{}
						t := c.S.Tick
						if !m.Run(c) || c.S.Over() {
							continue
						}
						room := -1
						for r := range c.Visited {
							room = r
						}
						out = append(out, expansion{f.Pos(), m.Name, c, int32(c.S.Tick - t), room})
					}
					results <- out
				}
			}()
		}
		go func() {
			for _, f := range frontier {
				work <- f
			}
			close(work)
			wg.Wait()
			close(results)
		}()
		var next []*Bot
		for out := range results {
			for _, e := range out {
				to := e.bot.Pos()
				if e.room >= 0 && to == e.from {
					g.Doors[e.from] = e.room
				}
				if to == e.from {
					continue
				}
				g.Edges[e.from] = append(g.Edges[e.from], Edge{to, e.macro, e.ticks})
				if !seen[to] {
					seen[to] = true
					next = append(next, e.bot)
				}
			}
		}
		if logf != nil {
			logf("level %d: %d positions so far, %d new (%v)", level, len(seen), len(next), time.Since(t0).Round(time.Second))
		}
		frontier = next
	}
	return g
}

// Save writes the graph as JSON.
func (g *Graph) Save(path string) error {
	type edge struct {
		From, To flat
		Macro    string
		Ticks    int32
	}
	type door struct {
		At   flat
		Room int
	}
	var d struct {
		Edges []edge
		Doors []door
	}
	for from, es := range g.Edges {
		for _, e := range es {
			d.Edges = append(d.Edges, edge{flatten(from), flatten(e.To), e.Macro, e.Ticks})
		}
	}
	for at, r := range g.Doors {
		d.Doors = append(d.Doors, door{flatten(at), r})
	}
	sort.Slice(d.Edges, func(i, j int) bool {
		a, b := d.Edges[i], d.Edges[j]
		if a.From != b.From {
			return a.From.X < b.From.X || a.From.X == b.From.X && (a.From.Y < b.From.Y || a.From.Y == b.From.Y && (a.From.PC < b.From.PC || a.From.PC == b.From.PC && !a.From.Left && b.From.Left))
		}
		return a.Macro < b.Macro
	})
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// LoadGraph reads a graph written by Save.
func LoadGraph(path string) (*Graph, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d struct {
		Edges []struct {
			From, To flat
			Macro    string
			Ticks    int32
		}
		Doors []struct {
			At   flat
			Room int
		}
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	g := &Graph{Edges: map[PosKey][]Edge{}, Doors: map[PosKey]int{}}
	for _, e := range d.Edges {
		g.Edges[e.From.pos()] = append(g.Edges[e.From.pos()], Edge{e.To.pos(), e.Macro, e.Ticks})
	}
	for _, x := range d.Doors {
		g.Doors[x.At.pos()] = x.Room
	}
	return g, nil
}

// flat is a PosKey as it is written to the file.
type flat struct {
	X, Y int16
	Left bool
	PC   int32
}

func flatten(p PosKey) flat { return flat{p.X, p.Y, p.Left, p.PC} }
func (f flat) pos() PosKey  { return PosKey{Loc{f.X, f.Y, f.Left}, f.PC} }

// DoorPositions are the positions where "up" enters the room.
func (g *Graph) DoorPositions(room int) []PosKey {
	var out []PosKey
	for p, r := range g.Doors {
		if r == room {
			out = append(out, p)
		}
	}
	return out
}

// DistancesTo returns, for every place, the ticks it takes to reach any of the targets,
// by running Dijkstra's algorithm backwards over the edges.
func (g *Graph) DistancesTo(targets []PosKey) map[Loc]int32 {
	rev := map[PosKey][]Edge{} // To -> edges, with To holding the source
	for from, es := range g.Edges {
		for _, e := range es {
			rev[e.To] = append(rev[e.To], Edge{from, e.Macro, e.Ticks})
		}
	}
	return byLoc(dijkstra(targets, rev))
}

// byLoc keeps the shortest distance of the states at each place.
func byLoc(d map[PosKey]int32) map[Loc]int32 {
	out := make(map[Loc]int32, len(d))
	for k, v := range d {
		if old, ok := out[k.Loc]; !ok || v < old {
			out[k.Loc] = v
		}
	}
	return out
}

// DistancesFrom is the same forwards: the ticks from p to every place. If the graph has not
// seen p's script at that place, any state at the place stands in.
func (g *Graph) DistancesFrom(p PosKey) map[Loc]int32 {
	if _, ok := g.Edges[p]; !ok {
		for k := range g.Edges {
			if k.Loc == p.Loc {
				p = k
				break
			}
		}
	}
	return byLoc(dijkstra([]PosKey{p}, g.Edges))
}

func dijkstra(sources []PosKey, edges map[PosKey][]Edge) map[PosKey]int32 {
	dist := map[PosKey]int32{}
	pq := &posQueue{}
	for _, s := range sources {
		dist[s] = 0
		heap.Push(pq, posItem{s, 0})
	}
	for pq.Len() > 0 {
		it := heap.Pop(pq).(posItem)
		if d, ok := dist[it.p]; ok && d < it.d {
			continue
		}
		for _, e := range edges[it.p] {
			nd := it.d + e.Ticks
			if d, ok := dist[e.To]; !ok || nd < d {
				dist[e.To] = nd
				heap.Push(pq, posItem{e.To, nd})
			}
		}
	}
	return dist
}

type posItem struct {
	p PosKey
	d int32
}
type posQueue []posItem

func (q posQueue) Len() int            { return len(q) }
func (q posQueue) Less(i, j int) bool  { return q[i].d < q[j].d }
func (q posQueue) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *posQueue) Push(x interface{}) { *q = append(*q, x.(posItem)) }
func (q *posQueue) Pop() interface{} {
	old := *q
	n := len(old)
	it := old[n-1]
	*q = old[:n-1]
	return it
}

// PathTo returns the macros that the graph says lead from one position to another, found by
// Dijkstra's algorithm, or false when there is none.
func (g *Graph) PathTo(from, to PosKey) ([]string, bool) {
	dist := map[PosKey]int32{from: 0}
	prev := map[PosKey]Edge{}
	prevFrom := map[PosKey]PosKey{}
	pq := &posQueue{}
	heap.Push(pq, posItem{from, 0})
	for pq.Len() > 0 {
		it := heap.Pop(pq).(posItem)
		if d := dist[it.p]; d < it.d {
			continue
		}
		if it.p == to {
			var names []string
			for p := to; p != from; p = prevFrom[p] {
				names = append([]string{prev[p].Macro}, names...)
			}
			return names, true
		}
		for _, e := range g.Edges[it.p] {
			nd := it.d + e.Ticks
			if d, ok := dist[e.To]; !ok || nd < d {
				dist[e.To] = nd
				prev[e.To] = e
				prevFrom[e.To] = it.p
				heap.Push(pq, posItem{e.To, nd})
			}
		}
	}
	return nil, false
}

// PathEdges is PathTo with the positions: the edges to follow, in order.
func (g *Graph) PathEdges(from, to PosKey) ([]Edge, bool) {
	dist := map[PosKey]int32{from: 0}
	prev := map[PosKey]Edge{}
	prevFrom := map[PosKey]PosKey{}
	pq := &posQueue{}
	heap.Push(pq, posItem{from, 0})
	for pq.Len() > 0 {
		it := heap.Pop(pq).(posItem)
		if d := dist[it.p]; d < it.d {
			continue
		}
		if it.p == to {
			var es []Edge
			for p := to; p != from; p = prevFrom[p] {
				es = append([]Edge{prev[p]}, es...)
			}
			return es, true
		}
		for _, e := range g.Edges[it.p] {
			nd := it.d + e.Ticks
			if d, ok := dist[e.To]; !ok || nd < d {
				dist[e.To] = nd
				prev[e.To] = e
				prevFrom[e.To] = it.p
				heap.Push(pq, posItem{e.To, nd})
			}
		}
	}
	return nil, false
}
