// Package solve plays the game with a bot. It is a development tool: it finds a sequence of
// inputs that finishes the game, so that a test can replay it. The search works on clones of
// the game state, over macro actions (walk one stride, take the stairs, use a door ...), and
// the bot shoots enemies by itself while it moves.
package solve

import (
	"github.com/karlbe/gnistaEngine/pkg/game"
	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/input"
)

// Bot is a game being played, with the log of the inputs given.
type Bot struct {
	S       *game.State
	Log     []input.Actions
	Visited map[int]bool // rooms entered
	walking bool         // walking ignores the enemies: the bot is invulnerable, and they only block stairs, lifts and doors
}

// NewBot starts a bot on a state.
func NewBot(s *game.State) *Bot {
	s.DropMapMemory() // the bot draws nothing, and a small state is quick to copy
	return &Bot{S: s, Visited: map[int]bool{}}
}

// Invulnerable switches the invulnerability cheat on with the first input (it needs the
// improvements flag), so that the fights cannot kill the bot.
func (b *Bot) Invulnerable() {
	b.S.SetImprovements(true)
	b.Step(input.ToggleInvulnerable)
}

// Clone copies the bot, with an empty log.
func (b *Bot) Clone() *Bot {
	v := make(map[int]bool, len(b.Visited))
	for k := range b.Visited {
		v[k] = true
	}
	return &Bot{S: b.S.Clone(), Visited: v, walking: b.walking}
}

// Step gives the game one tick of input. The bot's fighting takes over the input while an
// enemy is on screen.
func (b *Bot) Step(in input.Actions) {
	if !b.walking {
		if f, ok := b.fight(); ok {
			in = f
		}
	}
	b.Log = append(b.Log, in)
	b.S.Step(in)
	if r := b.S.Room; r != nil {
		b.Visited[r.Index] = true
	}
}

// Wait gives n ticks of no input.
func (b *Bot) Wait(n int) {
	for i := 0; i < n; i++ {
		b.Step(0)
	}
}

// Hold gives n ticks of the same input.
func (b *Bot) Hold(in input.Actions, n int) {
	for i := 0; i < n; i++ {
		b.Step(in)
	}
}

// player is the player's slot.
func (b *Bot) player() *script.Slot { return &b.S.Engine.Slots[0] }

// facingLeft reports the player's facing ($506 bit 1).
func (b *Bot) facingLeft() bool { return b.S.Engine.G.MoveBits&2 != 0 }

// busy reports a script the player cannot be interrupted in: stairs, a ladder, the lift.
func (b *Bot) busy() bool {
	g := &b.S.Engine.G
	return g.Climbing != 0 || g.LiftBusy
}

// fight shoots the nearest enemy on screen: it turns towards it and fires.
func (b *Bot) fight() (input.Actions, bool) {
	if b.S.Room != nil || b.busy() || b.S.Over() {
		return 0, false
	}
	vm := b.S.Engine
	p := vm.Slots[0]
	best, bd := -1, 1<<30
	for k := 1; k <= 3; k++ {
		e := vm.Slots[k]
		if e.Actor == nil || e.Dead {
			continue
		}
		d := int(e.X) - int(p.X)
		if d < 0 {
			d = -d
		}
		if d < bd {
			best, bd = k, d
		}
	}
	if best < 0 {
		return 0, false
	}
	if w, ok := b.bestWeapon(); !ok {
		return 0, false
	} else if int(vm.G.Weapon) != w {
		return [3]input.Actions{input.Weapon1, input.Weapon2, input.Weapon3}[w], true
	}
	right := vm.Slots[best].X > p.X
	if right == b.facingLeft() { // facing the wrong way
		if right {
			return input.Right, true
		}
		return input.Left, true
	}
	return input.Fire, true
}

// bestWeapon picks the weapon to fight with: the shotgun (weapon 1) kills in one hit, then
// the sub-machine gun, then the pistol. Only weapons the player owns and that have ammo count.
func (b *Bot) bestWeapon() (int, bool) {
	g := &b.S.Engine.G
	for _, w := range []int{1, 2, 0} {
		if g.WeaponsOwned&(1<<w) != 0 && (g.Ammo[w].Rounds > 0 || g.Ammo[w].Mags > 0) {
			return w, true
		}
	}
	return 0, false
}

// settled reports whether the player may be idle: not in a room, not on stairs or in the
// lift. Settle also checks that the script position stands still.
func (b *Bot) settled() bool {
	return b.S.Room == nil && !b.busy() && !b.S.Over()
}

// Settle waits, with no input, until the player is idle or has entered a room. It gives up
// after limit ticks and reports whether the player did settle.
func (b *Bot) Settle(limit int) bool {
	still := 0
	prev := b.player().PC
	for i := 0; i < limit; i++ {
		b.Step(0)
		if b.S.Over() {
			return false
		}
		if b.S.Room != nil {
			return true
		}
		pc := b.player().PC
		if b.settled() && pc == prev && (b.walking || !b.enemyOnScreen()) {
			if still++; still >= 5 {
				return true
			}
		} else {
			still = 0
		}
		prev = pc
	}
	return false
}

func (b *Bot) enemyOnScreen() bool {
	for k := 1; k <= 3; k++ {
		if b.S.Engine.Slots[k].Actor != nil {
			return true
		}
	}
	return false
}

// Clear shoots until no enemy is on screen (or time or ammunition runs out). Stairs, lifts and
// doors do nothing while an enemy is on screen.
func (b *Bot) Clear() {
	for i := 0; i < 3000 && b.enemyOnScreen() && !b.S.Over() && b.S.Room == nil; i++ {
		if _, fighting := b.fight(); !fighting {
			return
		}
		b.Step(0)
	}
}
