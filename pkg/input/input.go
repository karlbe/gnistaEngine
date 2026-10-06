// Package input defines the platform-independent player input for one simulation tick.
package input

// Actions is the set of controls held during one tick: the original's joystick (four
// directions and fire) plus its game keys (Space places a charge, F1-F3 pick a weapon,
// P pauses until fire).
type Actions uint16

const (
	Up Actions = 1 << iota
	Down
	Left
	Right
	Fire
	Charge
	Weapon1
	Weapon2
	Weapon3
	Pause
	Cheat              // not in the original: all weapons, cards, full ammo and full health
	ToggleInvulnerable // not in the original: pressed once to switch on or off
	ToggleSpeed        // not in the original: double speed on or off
)

func (a Actions) Has(b Actions) bool { return a&b != 0 }

// Source produces the actions for the next tick (keyboard, gamepad, replay, network).
type Source interface {
	Poll() Actions
}
