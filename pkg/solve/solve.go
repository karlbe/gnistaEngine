package solve

import (
	"fmt"
	"sort"

	"github.com/karlbe/gnistaEngine/pkg/game/script"

	"github.com/karlbe/gnistaEngine/pkg/input"
)

// goalBomb is true when the bot stands in the bomb room.
func goalBomb(b *Bot) bool { return b.S.Room != nil && b.S.Room.Bomb }

// Defuse cuts the bomb's right wire: it starts the wire cutting with the charge key, moves the
// cursor with right, and cuts with fire. It reports whether the game was won.
func (b *Bot) Defuse() bool {
	for i := 0; i < 600 && b.S.Room != nil && !b.S.Room.Showing(); i++ {
		b.Step(0)
	}
	if b.S.Room == nil || !b.S.Room.Bomb {
		return false
	}
	b.Hold(input.Charge, 2)
	for i := 0; i < 3000 && b.S.Room != nil; i++ {
		v := b.S.Room
		switch {
		case !v.Wires:
			b.Step(0)
		case v.Cursor < 3: // the fourth wire is the one
			b.Step(input.Right)
		default:
			b.Step(input.Fire)
		}
	}
	return b.S.Engine.G.Outcome == 3
}

// WatchEnding presses fire, a tick at a time, until the ending is over.
func (b *Bot) WatchEnding() bool {
	for i := 0; i < 20000 && !b.S.Finished(); i++ {
		if i%8 == 0 {
			b.Step(input.Fire)
		} else {
			b.Step(0)
		}
	}
	return b.S.Finished()
}

// kills is how many enemies the player's ammunition can kill: the shotgun (weapon 1) kills in
// one hit, the other weapons need two.
func kills(g *script.Globals) int {
	n := 0
	for w := 0; w < 3; w++ {
		if g.WeaponsOwned&(1<<w) == 0 {
			continue
		}
		rounds := int(g.Ammo[w].Rounds) + int(g.Ammo[w].Mags)*int(g.Ammo[w].MagSize)
		if w == 1 {
			n += rounds
		} else {
			n += rounds / 2
		}
	}
	return n
}

// Enough is how many kills the bot wants in stock: the bomb room's corridor sends waves of
// enemies, and the bot is only invulnerable, not safe from running out of ammunition.
const enough = 70

// wanted reports whether entering the room is useful: it holds a card the player lacks,
// ammunition when the stock is low (enemies on screen block the stairs and the lift until they
// are shot), or charges when none are left (the bomb room's door needs one).
func (b *Bot) wanted(room int) bool {
	if b.Visited[room] || room >= len(b.S.Rooms) {
		return false
	}
	r := b.S.Rooms[room]
	g := &b.S.Engine.G
	if card := r[1] & 0x1F; card&^g.Cards != 0 {
		return true
	}
	if r[1]&0x40 != 0 && g.WeaponsOwned&2 == 0 || r[1]&0x80 != 0 && g.WeaponsOwned&4 == 0 {
		return true // a weapon the player lacks (the shotgun kills in one hit)
	}
	if kills(g) < enough {
		for w := 0; w < 3; w++ {
			if g.WeaponsOwned&(1<<w) != 0 && r[2+w] > 0 {
				return true
			}
		}
	}
	return r[5] > 0 && g.Charges == 0
}

// guide makes a search heuristic from distances over the graph: the ticks the graph says
// it takes to get from where the bot stands to the target.
func guide(dist map[Loc]int32) func(*Bot) uint64 {
	return func(b *Bot) uint64 {
		if d, ok := dist[b.Pos().Loc]; ok {
			return uint64(d)
		}
		return 1 << 20
	}
}

// Solve plays the game with the bot until it is won or time runs out, and reports what
// happened. It collects what is needed to get to the bomb room (the cards, ammunition and
// charges) room by room, the nearest first by the graph's distances, and heads for the bomb
// room as soon as it has all the cards. The graph guides every search.
func Solve(b *Bot, g *Graph, logf func(format string, args ...any)) error {
	return solve(b, g, logf, false, 0)
}

// SolveAll plays as Solve does but visits every room it can first: after the cards and the
// ammunition, it goes on to the nearest unvisited room as long as more than reserve frames of
// the game clock are left, so that there is time for the way to the bomb.
func SolveAll(b *Bot, g *Graph, reserve int, logf func(format string, args ...any)) error {
	return solve(b, g, logf, false, reserve)
}

// SolveUntilCards plays as Solve does but stops when the player has all the cards.
func SolveUntilCards(b *Bot, g *Graph, logf func(format string, args ...any)) error {
	return solve(b, g, logf, true, 0)
}

func solve(b *Bot, g *Graph, logf func(format string, args ...any), stopAtCards bool, reserve int) error {
	b.Invulnerable()
	b.Settle(300)
	// The bomb room's door is on tile row 23 (the 17th floor), in a stretch of corridor that
	// the map may not connect to the rest. The guide heads for the stretch's nearest known end.
	var bomb []PosKey
	for k := range g.Edges {
		if int(k.Y)/16 == 23 && int(k.X)/16 <= 29 {
			bomb = append(bomb, k)
		}
	}
	if len(bomb) == 0 {
		return fmt.Errorf("the graph has no position near the bomb room")
	}
	failed := map[int]int{} // rooms a search could not reach, by the number of rooms visited then
	for leg := 0; leg < 200; leg++ {
		if b.S.Over() {
			return fmt.Errorf("the game ended (outcome %d) after %d rooms, tick %d", b.S.Engine.G.Outcome, len(b.Visited), b.S.Tick)
		}
		gm := &b.S.Engine.G
		if stopAtCards && gm.Cards == 0x1F {
			return nil
		}
		if reserve > 0 && gm.Cards == 0x1F && kills(gm) >= enough && b.S.Clock.FramesLeft() > reserve {
			// time to spare: the nearest room not yet visited
			from := g.DistancesFrom(b.Pos())
			bestRoom, bestD := -1, int32(1<<30)
			for r := 0; r < len(b.S.Rooms); r++ {
				if b.Visited[r] || r == 52 || failed[r] == len(b.Visited)+1 {
					continue
				}
				for _, p := range g.DoorPositions(r) {
					if d, ok := from[p.Loc]; ok && d < bestD {
						bestRoom, bestD = r, d
					}
				}
			}
			if bestRoom >= 0 {
				room := bestRoom
				goal := func(n *Bot) bool { return n.Visited[room] }
				path, err := SearchH(b, goal, guide(g.DistancesTo(g.DoorPositions(room))), 8000)
				if err != nil {
					path, err = SearchFewest(b, goal, 300000)
				}
				if err != nil {
					failed[room] = len(b.Visited) + 1
					continue
				}
				if !b.Do(path) {
					return fmt.Errorf("a path failed")
				}
				logf("extra %2d: room %2d, %2d rooms, tick %5d, clock frames left %d", leg, room, len(b.Visited), b.S.Tick, b.S.Clock.FramesLeft())
				continue
			}
		}
		if gm.Cards == 0x1F && gm.Charges > 0 && (kills(gm) >= enough || len(b.wantedRooms()) == 0) {
			logf("leg %d: all cards, heading for the bomb room at tick %d (%d rooms)", leg, b.S.Tick, len(b.Visited))
			near := map[Loc]bool{}
			for _, k := range bomb {
				near[k.Loc] = true
			}
			path, err := SearchFewest(b, func(n *Bot) bool { return near[n.Pos().Loc] }, 1500000)
			if err != nil {
				return fmt.Errorf("no way to the bomb floor: %v", err)
			}
			if !b.Do(path) {
				return fmt.Errorf("the way to the bomb floor failed")
			}
			logf("at the bomb floor, tick %d", b.S.Tick)
			path, err = SearchFewest(b, goalBomb, 300000)
			if err != nil {
				return fmt.Errorf("no way to the bomb room from the bomb floor: %v", err)
			}
			if !b.Do(path) {
				return fmt.Errorf("the way to the bomb room failed")
			}
			if !b.Defuse() {
				return fmt.Errorf("the bomb was not defused (outcome %d)", gm.Outcome)
			}
			logf("the bomb is defused at tick %d", b.S.Tick)
			if !b.WatchEnding() {
				return fmt.Errorf("the ending did not finish")
			}
			return nil
		}
		// the wanted rooms, nearest first by the graph
		from := g.DistancesFrom(b.Pos())
		type cand struct {
			room int
			d    int32
		}
		var cands []cand
		for r := 0; r < len(b.S.Rooms); r++ {
			if !b.wanted(r) || failed[r] == len(b.Visited)+1 {
				continue
			}
			best := int32(1 << 30)
			for _, p := range g.DoorPositions(r) {
				if d, ok := from[p.Loc]; ok && d < best {
					best = d
				}
			}
			if best < 1<<30 {
				cands = append(cands, cand{r, best})
			}
		}
		sort.Slice(cands, func(i, j int) bool { return cands[i].d < cands[j].d })
		done := false
		for _, c := range cands {
			room := c.room
			goal := func(n *Bot) bool { return n.Visited[room] }
			path, err := SearchH(b, goal, guide(g.DistancesTo(g.DoorPositions(room))), 8000)
			if err != nil {
				failed[room] = len(b.Visited) + 1
				continue
			}
			if !b.Do(path) {
				return fmt.Errorf("a path failed")
			}
			logf("leg %2d: room %2d, %2d rooms, tick %5d, cards %05b, weapons %03b, charges %d", leg, room, len(b.Visited), b.S.Tick, gm.Cards, gm.WeaponsOwned, gm.Charges)
			done = true
			break
		}
		if !done { // nothing needed is in reach yet: take the nearest room, whatever it holds
			before := len(b.Visited)
			seenBefore := copyRooms(b.Visited)
			path, err := SearchFewest(b, func(n *Bot) bool { return len(n.Visited) > before }, 200000)
			if err != nil {
				return fmt.Errorf("no room can be reached after %d rooms at tick %d: %v", len(b.Visited), b.S.Tick, err)
			}
			if !b.Do(path) {
				return fmt.Errorf("a path failed")
			}
			logf("leg %2d: any room (%s), %2d rooms, tick %5d, cards %05b, weapons %03b, charges %d, mags %d/%d/%d", leg, newRooms(b, seenBefore), len(b.Visited), b.S.Tick, gm.Cards, gm.WeaponsOwned, gm.Charges, gm.Ammo[0].Mags, gm.Ammo[1].Mags, gm.Ammo[2].Mags)
		}
	}
	return fmt.Errorf("gave up")
}

func copyRooms(m map[int]bool) map[int]bool {
	c := make(map[int]bool, len(m))
	for k := range m {
		c[k] = true
	}
	return c
}

// newRooms lists the rooms visited since the snapshot.
func newRooms(b *Bot, before map[int]bool) string {
	s := ""
	for r := range b.Visited {
		if !before[r] {
			s += fmt.Sprintf("%d ", r)
		}
	}
	return s
}

// wantedRooms lists the rooms that are worth entering now.
func (b *Bot) wantedRooms() []int {
	var out []int
	for r := range b.S.Rooms {
		if b.wanted(r) {
			out = append(out, r)
		}
	}
	return out
}
