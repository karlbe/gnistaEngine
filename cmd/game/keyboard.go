package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/karlbe/gnistaEngine/pkg/input"
)

// keyboard maps the cursor keys (or WASD) and Ctrl to the joystick, like the WinUAE test setup
// (kbd2), and Space and F1-F3 to the original's game keys.
type keyboard struct {
	improve *bool // WASD and the cheat keys are not in the original
}

// keys are the original's controls, extras the keys that only work with the improvements.
var keys = map[ebiten.Key]input.Actions{
	ebiten.KeyArrowUp: input.Up, ebiten.KeyArrowDown: input.Down,
	ebiten.KeyArrowLeft: input.Left, ebiten.KeyArrowRight: input.Right,
	ebiten.KeyControlRight: input.Fire, ebiten.KeyControlLeft: input.Fire,
	ebiten.KeySpace: input.Charge,
	ebiten.KeyF1:    input.Weapon1, ebiten.KeyF2: input.Weapon2, ebiten.KeyF3: input.Weapon3,
	ebiten.KeyP: input.Pause,
}

var extras = map[ebiten.Key]input.Actions{
	ebiten.KeyW: input.Up, ebiten.KeyS: input.Down, ebiten.KeyA: input.Left, ebiten.KeyD: input.Right,
	ebiten.KeyF4: input.Cheat, ebiten.KeyF5: input.ToggleInvulnerable, ebiten.KeyF6: input.ToggleSpeed,
}

func (k keyboard) Poll() input.Actions {
	var a input.Actions
	for key, act := range keys {
		if ebiten.IsKeyPressed(key) {
			a |= act
		}
	}
	if k.improve != nil && *k.improve {
		for key, act := range extras {
			if ebiten.IsKeyPressed(key) {
				a |= act
			}
		}
	}
	return a
}
