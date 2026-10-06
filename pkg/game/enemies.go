package game

import (
	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
)

// Trigger bits in a cell: bit 0 marks a trigger, bit 1 arms it (set at the start, cleared
// when it fires), bits 2-6 choose what follows the first enemy.
const (
	cellTrigger = 1 << 0
	cellArmed   = 1 << 1
)

// waves are the delays (frames) between the further enemies after a trigger, by trigger
// bit 2-5 ($562-$710). Bit 6 starts a single enemy.
var waves = [4][]uint16{
	{30, 40, 80},
	{30, 80, 80, 50, 40, 40},
	{30, 50, 80, 50, 40, 50, 40, 30},
	{30, 60, 80, 50, 40, 50, 40, 30, 20, 20, 20, 20, 20, 20, 20},
}

// Spawner is the state of the enemy spawner.
type Spawner struct {
	Cells   []uint8    // copy of the level's trigger table (the original changes it in place)
	Wave    bool       // $C26: a wave is running
	Delays  [15]uint16 // $BDA
	Next    int        // $C22: current delay
	Left    int16      // $C20: wave enemies after the current one
	Toggle  bool       // $C17 bit 0: alternates the first enemy's template
	Random  uint32     // $C2C: rotated to choose a side when the player stands still
	Variant int        // $C18: position in the list at $C02
	OffR    int16      // $C30, $C32: staggered start positions of wave enemies
	OffL    int16
	Contact bool // $BD8: an enemy reached the player; checks stop
	Danger  bool // $9A88: an enemy is on screen (used by the sound code)
}

func newSpawner(t *world.Tables) Spawner {
	sp := Spawner{Random: 0xAAAAAAAA, Cells: append([]uint8(nil), t.Triggers...)}
	// $496: arm every trigger up to the end marker.
	for i := range sp.Cells {
		if sp.Cells[i] == 0xFF {
			break
		}
		if sp.Cells[i]&cellTrigger != 0 {
			sp.Cells[i] |= cellArmed
		}
	}
	return sp
}

// enemies runs $508 before the actor scripts: the contact check, then the spawner. Both
// pause while the player is on stairs ($C34).
func (s *State) enemies() {
	vm := s.Engine
	if vm.G.Climbing != 0 {
		return
	}
	s.contact()
	sp := &s.Spawner
	if sp.Wave {
		if d := &sp.Delays[sp.Next]; *d != 0 {
			*d--
		} else {
			s.waveEnemy()
			return
		}
	}
	s.trigger()
}

// trigger is $522: an armed cell under the player starts an enemy and possibly a wave.
func (s *State) trigger() {
	sp, vm := &s.Spawner, s.Engine
	c := s.cell(vm.G.Cell)
	if *c&cellArmed == 0 {
		return
	}
	for bit := 2; bit <= 6; bit++ {
		if *c&(1<<bit) == 0 {
			continue
		}
		k := s.freeSlot()
		if k < 0 {
			return
		}
		s.firstEnemy(k)
		*c &^= cellArmed
		if bit == 6 {
			sp.Wave = false
			return
		}
		w := waves[bit-2]
		copy(sp.Delays[:], w)
		sp.Next, sp.Left, sp.Wave = 0, int16(len(w)-1), true
		return
	}
}

// cell returns the trigger flags at index i of the table; outside it there are none.
func (s *State) cell(i int) *uint8 {
	if i >= 0 && i < len(s.Spawner.Cells) {
		return &s.Spawner.Cells[i]
	}
	var b uint8
	return &b
}

// waveEnemy is $74E: when the delay has run out, the next wave enemy appears if the block
// right of the player's cell allows it and a slot is free.
func (s *State) waveEnemy() {
	sp, vm := &s.Spawner, s.Engine
	var block uint8
	if i := vm.G.Cell + 1; i >= 0 && i < len(s.content.Level.Cells) {
		block = s.content.Level.Cells[i]
	}
	if s.content.Level.T.BlockAttr[block]&1 == 0 {
		return
	}
	k := s.freeSlot()
	if k < 0 {
		return
	}
	// $7F2: in front of a walking player, otherwise on a pseudo-random side.
	right := vm.G.MoveBits&2 == 0
	if vm.G.MoveBits&1 == 0 {
		sp.Random = sp.Random>>1 | sp.Random<<31
		right = sp.Random&1 == 0
	}
	if right {
		s.spawn(k, s.prog.Spec.WaveRight[s.variant()], true, stagger(&sp.OffR))
	} else {
		s.spawn(k, s.prog.Spec.WaveLeft[s.variant()], false, stagger(&sp.OffL))
	}
	if sp.Left != 0 {
		sp.Left--
		sp.Next++
	} else {
		sp.Wave = false
	}
}

// stagger cycles a wave start offset through 12, 6, 0 ($85E, $88C).
func stagger(off *int16) int16 {
	if *off != 0 {
		*off -= 6
	} else {
		*off = 12
	}
	return *off
}

// variant is $A86: the next entry of the wave order selects one of three templates. When the
// order has run out it starts again, taking its first entry twice in a row as the original does.
func (s *State) variant() int {
	sp, order := &s.Spawner, s.prog.Spec.WaveOrder
	if sp.Variant >= len(order) {
		sp.Variant = 0
		return min(int(order[0]), 2)
	}
	v := order[sp.Variant]
	sp.Variant++
	return min(int(v), 2)
}

// firstEnemy is $7B6: the trigger's enemy appears at the edge the player faces, with one
// of two templates in turn.
func (s *State) firstEnemy(k int) {
	sp := &s.Spawner
	off := 0
	if sp.Toggle {
		off = 1
	}
	sp.Toggle = !sp.Toggle
	if s.Engine.G.MoveBits&2 == 0 {
		s.spawn(k, s.prog.Spec.FirstRight[off], true, 6)
	} else {
		s.spawn(k, s.prog.Spec.FirstLeft[off], false, 6)
	}
}

// freeSlot is $942: the first of slots 1-3 without an actor, or -1.
func (s *State) freeSlot() int {
	for k := 1; k <= 3; k++ {
		if s.Engine.Slots[k].Actor == nil {
			return k
		}
	}
	return -1
}

// spawn starts slot k from a template ($98E-$A84) at the right ($8BA) or left ($8FE) edge
// of the view, d pixels in, at floor height.
func (s *State) spawn(k int, tmpl script.Template, right bool, d int16) {
	vm := s.Engine
	sl := &vm.Slots[k]
	sl.PC = tmpl.PC
	sl.Enemy.Scripts = tmpl.Scripts
	sl.Enemy.HP = 2
	x := vm.G.ViewX + 1 + d
	if right {
		x = vm.G.ViewX + 0x14F - d
	}
	y := vm.G.ViewY + 0x60
	a := &s.env.enemies[k-1]
	*a = script.Actor{
		Upper: script.Sprite{X: x, Y: y, Frame: a.Upper.Frame},
		Lower: script.Sprite{X: x, Y: y - 16, Frame: a.Lower.Frame},
		State: 1,
	}
	sl.X, sl.Y, sl.Actor, sl.Dead = x, y, a, false
}

// contact is $ACA: enemies that have left the view are removed; an enemy on screen blocks
// stairs and lifts ($C1C); one within 32 pixels catches the player, which ends the checks.
func (s *State) contact() {
	sp, vm := &s.Spawner, s.Engine
	if sp.Contact {
		return
	}
	vm.G.Blocked, sp.Danger = 0, false
	p := &vm.Slots[0]
	px := uint16(p.X)
	for k := 1; k <= 3; k++ {
		e := &vm.Slots[k]
		if e.Actor == nil {
			continue
		}
		ex := uint16(e.X)
		if ex <= px {
			if ex < uint16(vm.G.ViewX) {
				s.despawn(k)
				continue
			}
		} else if ex > uint16(vm.G.ViewX+0x150) {
			s.despawn(k)
			continue
		}
		vm.G.Blocked, sp.Danger = 1, true
		if vm.G.Invulnerable {
			continue
		}
		switch {
		case ex <= px && px-32 < ex:
			p.PC = s.prog.Spec.ContactLeft
		case ex > px && px+32 > ex:
			p.PC = s.prog.Spec.ContactRight
		default:
			continue
		}
		e.PC = e.Enemy.Scripts[script.EnemyAfterShot]
		sp.Contact = true
		return
	}
}

// despawn is $B74: the enemy's bobs are hidden and the slot freed.
func (s *State) despawn(k int) {
	vm := s.Engine
	e := &vm.Slots[k]
	e.Actor.State = 2
	e.Actor = nil
	if vm.G.Shooter == k {
		vm.G.Shooter = 0
	}
}
