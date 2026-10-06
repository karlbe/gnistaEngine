package solve

import (
	"container/heap"
	"fmt"

	"github.com/karlbe/gnistaEngine/pkg/input"
)

// A macro is one thing the bot does from a settled position and then waits to settle again.
type Macro struct {
	Name string
	Run  func(b *Bot) bool // false: the macro could not be done
}

// Taps start one move: a tap of a direction walks a stride (or turns round), up and down take
// stairs, ladders and doors, and call the lift.
var (
	TapLeft  = Macro{"left", func(b *Bot) bool { return b.walk(input.Left, 8) }}
	TapRight = Macro{"right", func(b *Bot) bool { return b.walk(input.Right, 8) }}
	TapUp    = Macro{"up", func(b *Bot) bool { b.Clear(); b.Hold(input.Up, 8); return b.afterMove() }}
	TapDown  = Macro{"down", func(b *Bot) bool { b.Clear(); b.Hold(input.Down, 8); return b.afterMove() }}
)

// walk holds a direction for n ticks while ignoring the enemies, then settles.
func (b *Bot) walk(dir input.Actions, n int) bool {
	b.walking = true
	defer func() { b.walking = false }()
	b.Hold(dir, n)
	return b.afterMove()
}

// afterMove finishes a macro: if a room was entered it is read and left again, then the
// player settles.
func (b *Bot) afterMove() bool {
	ok := b.Settle(4000) // a wave of enemies can take a while to shoot
	if b.S.Room != nil {
		return b.visitRoom()
	}
	return ok
}

// visitRoom waits for the room's message and leaves the room with down.
func (b *Bot) visitRoom() bool {
	r := b.S.Room
	if r == nil {
		return true
	}
	for i := 0; i < 600 && b.S.Room != nil && !b.S.Room.Showing(); i++ {
		b.Step(0)
	}
	if b.S.Room == nil {
		return true
	}
	if b.S.Room.Bomb { // the bomb room is handled by the caller
		return true
	}
	b.Step(0)
	b.Hold(input.Down, 2)
	b.Wait(2)
	return b.Settle(600)
}

// Node is a settled position.
type Node struct {
	X, Y   int16
	Left   bool
	Cards  uint8 // what the player has, and the doors that stand open, change what can be reached
	Guns   uint8
	Blown  int
	Rounds int  // rooms visited: a visited room is empty
	Enemy  bool // an enemy is on screen: stairs, lifts and doors do nothing meanwhile
}

// NodeOf describes where the bot stands.
func (b *Bot) NodeOf() Node {
	p := b.player()
	g := &b.S.Engine.G
	return Node{p.X, p.Y, b.facingLeft(), g.Cards, g.WeaponsOwned, b.S.BlownDoors(), len(b.Visited), b.enemyOnScreen()}
}

// Runs hold a direction for several strides at once, which is quicker than one stride at a
// time: the key is released in the last stride, so the player stops at its end. A stride is
// 32 ticks.
func runMacro(dir input.Actions, name string, strides int) Macro {
	return Macro{name, func(b *Bot) bool { return b.walk(dir, 32*(strides-1)+8) }}
}

var (
	RunRight3 = runMacro(input.Right, "right3", 3)
	RunLeft3  = runMacro(input.Left, "left3", 3)
	RunRight8 = runMacro(input.Right, "right8", 8)
	RunLeft8  = runMacro(input.Left, "left8", 8)
)

// Moves are the macros tried from every position.
func Moves() []Macro {
	return []Macro{TapRight, TapLeft, TapUp, TapDown, RunRight3, RunLeft3, RunRight8, RunLeft8}
}

// Step records how a node was reached.
type Step struct {
	From  Node
	Macro Macro
}

// Path is a list of macros from a start node.
type Path []Macro

func (p Path) String() string {
	s := ""
	for i, m := range p {
		if i > 0 {
			s += " "
		}
		s += m.Name
	}
	return s
}

// Search finds the cheapest list of macros, in game ticks, that makes goal true, searching
// from the bot's position over clones of the game. goal is checked after each macro. It
// gives up after limit macros have been tried.
func Search(start *Bot, goal func(*Bot) bool, limit int) (Path, error) {
	return SearchH(start, goal, nil, limit)
}

// Toward is a guess of how many ticks it takes to get from the bot's position to a door: a
// tick per pixel along the floor and a few hundred per floor.
func Toward(d DoorTile) func(*Bot) uint64 {
	return func(b *Bot) uint64 {
		p := b.player()
		dx := abs(int(p.X)/16 - d.X)
		dy := abs(int(p.Y)/16-d.Y) / 4
		return uint64(dx*16 + dy*350)
	}
}

// SearchH is Search guided by a guess h of the ticks left (weighted A*: it finds a way much
// sooner than the cheapest one, which is all that is needed).
func SearchH(start *Bot, goal func(*Bot) bool, h func(*Bot) uint64, limit int) (Path, error) {
	t0 := start.S.Tick
	weight := uint64(0)
	if h != nil {
		weight = 2
	}
	pq := &queue{}
	best := map[Node]uint64{start.NodeOf(): 0}
	heap.Push(pq, &item{bot: start.Clone(), cost: 0, prio: 0})
	tried := 0
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(*item)
		if c, ok := best[cur.bot.NodeOf()]; ok && c < cur.cost {
			continue // a cheaper way here was found meanwhile
		}
		for _, m := range movesAt(cur.bot) {
			if tried++; tried > limit {
				return nil, fmt.Errorf("gave up after %d macros", limit)
			}
			nb := cur.bot.Clone()
			if !m.Run(nb) || nb.S.Over() {
				continue
			}
			path := append(append(Path(nil), cur.path...), m)
			if goal(nb) {
				return path, nil
			}
			n := nb.NodeOf()
			cost := nb.S.Tick - t0
			if c, ok := best[n]; ok && c <= cost {
				continue
			}
			best[n] = cost
			prio := cost
			if h != nil {
				prio += weight * h(nb)
			}
			heap.Push(pq, &item{bot: nb, path: path, cost: cost, prio: prio})
		}
	}
	return nil, fmt.Errorf("no way found (%d macros tried)", tried)
}

type item struct {
	bot  *Bot
	path Path
	cost uint64 // ticks so far
	prio uint64 // cost plus the guess, what the queue orders by
}

type queue []*item

func (q queue) Len() int            { return len(q) }
func (q queue) Less(i, j int) bool  { return q[i].prio < q[j].prio }
func (q queue) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x interface{}) { *q = append(*q, x.(*item)) }
func (q *queue) Pop() interface{} {
	old := *q
	n := len(old)
	it := old[n-1]
	*q = old[:n-1]
	return it
}

// Do runs the macros on the bot.
func (b *Bot) Do(p Path) bool {
	for _, m := range p {
		if !m.Run(b) {
			return false
		}
	}
	return true
}

// SearchFewest is Search with the fewest macros as the cost instead of ticks. It finds a way
// much sooner, but not always the quickest one.
func SearchFewest(start *Bot, goal func(*Bot) bool, limit int) (Path, error) {
	type entry struct {
		bot  *Bot
		path Path
	}
	seen := map[Node]bool{start.NodeOf(): true}
	queue := []entry{{start.Clone(), nil}}
	tried := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, m := range movesAt(cur.bot) {
			if tried++; tried > limit {
				return nil, fmt.Errorf("gave up after %d macros", limit)
			}
			nb := cur.bot.Clone()
			if !m.Run(nb) || nb.S.Over() {
				continue
			}
			path := append(append(Path(nil), cur.path...), m)
			if goal(nb) {
				return path, nil
			}
			if n := nb.NodeOf(); !seen[n] {
				seen[n] = true
				queue = append(queue, entry{nb, path})
			}
		}
	}
	return nil, fmt.Errorf("no way found (%d macros tried)", tried)
}
