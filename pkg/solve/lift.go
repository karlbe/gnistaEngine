package solve

import (
	"fmt"

	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/input"
)

const floorPixels = 64

// onLift reports whether the player stands on a lift tile ($29, $2A).
func (b *Bot) onLift() bool {
	t := b.player().Sensors[0]
	return t == 0x29 || t == 0x2A
}

// scriptRange reports whether the player's script position lies in scripts first..last.
func (b *Bot) scriptRange(first, last int) bool {
	pc := b.player().PC
	p := b.S.Engine.Prog
	return pc >= p.Script(first) && pc < p.Script(last+1)
}

// LiftRide is a macro that rides the lift floors floors up or down. It holds up to call the
// cabin and step in (scripts 85-87), keeps the stick in the wanted direction until the ride
// moves the view, and presses fire during the last floor, which makes the cabin stop at the
// end of it (the exit is script 89).
func LiftRide(up bool, floors int) Macro {
	name := fmt.Sprintf("lift-down-%d", floors)
	if up {
		name = fmt.Sprintf("lift-up-%d", floors)
	}
	return Macro{name, func(b *Bot) bool {
		if !b.onLift() {
			return false
		}
		b.Clear()
		g := &b.S.Engine.G
		entered := false
		for i := 0; i < 700 && !b.S.Over(); i++ {
			if b.scriptRange(85, 87) {
				entered = true
				break
			}
			b.Step(input.Up)
		}
		if !entered {
			b.Settle(300)
			return false
		}
		dir := input.Down
		if up {
			dir = input.Up
		}
		y0 := g.ViewY
		for i := 0; i < 300 && abs(int(g.ViewY)-int(y0)) < 4; i++ {
			b.Step(dir)
		}
		if abs(int(g.ViewY)-int(y0)) < 4 {
			b.Settle(300) // no lift tile in that direction
			return false
		}
		// Ride; fire in the last floor makes the cabin stop at the end of it.
		for i := 0; i < 3000 && !b.scriptRange(89, 89) && !b.S.Over(); i++ {
			in := input.Actions(0)
			if abs(int(g.ViewY)-int(y0)) >= floorPixels*(floors-1)+8 {
				in = input.Fire
			}
			b.Step(in)
		}
		return b.Settle(900)
	}}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// movesAt are the macros tried from the bot's position.
func movesAt(b *Bot) []Macro {
	m := Moves()
	if d := b.S.DoorHere(); d.Door && d.Locked && b.S.Engine.G.Charges > 0 {
		m = append(m, BlowLeft, BlowRight)
	}
	if b.onLift() {
		for n := 1; n <= 6; n++ {
			m = append(m, LiftRide(true, n), LiftRide(false, n))
		}
	}
	return m
}

// Blow places an explosive charge on the door the player stands at, walks a stride away in
// the given direction as soon as the placing is done, and waits for the blast. The player
// has to be off the charge's tile when it goes off.
func Blow(dir input.Actions, name string) Macro {
	return Macro{name, func(b *Bot) bool {
		d := b.S.DoorHere()
		if !d.Door || !d.Locked || b.S.Engine.G.Charges == 0 {
			return false
		}
		b.Clear()
		// The charge key is only read when the player waits for orders. After a fight the
		// player may be in another wait, so up (which does nothing at a locked door) wakes the
		// player, and the charge key is tried again until a charge is used.
		charges := b.S.Engine.G.Charges
		for try := 0; try < 6 && b.S.Engine.G.Charges == charges && !b.S.Over(); try++ {
			b.Hold(input.Charge, 2)
			if b.S.Engine.G.Charges != charges {
				break
			}
			b.Wait(3)
			if b.S.Engine.G.Charges != charges {
				break
			}
			b.Hold(input.Up, 3)
			b.Wait(10)
		}
		if b.S.Engine.G.Charges == charges {
			return false
		}
		helper := &b.S.Engine.Slots[script.HelperSlot]
		// The player is busy placing it; the charge appears when that is done, and the player
		// is idle again a moment later. The fuse is short, so the step away comes at once.
		stable, prev := 0, -1
		for i := 0; i < 400 && !b.S.Over(); i++ {
			b.Step(0)
			pc := b.player().PC
			if helper.Actor != nil && pc == prev {
				stable++
			} else {
				stable = 0
			}
			prev = pc
			if stable >= 2 {
				break
			}
		}
		b.Hold(dir, 8)
		return b.Settle(900)
	}}
}

var (
	BlowLeft  = Blow(input.Left, "blow-left")
	BlowRight = Blow(input.Right, "blow-right")
)
