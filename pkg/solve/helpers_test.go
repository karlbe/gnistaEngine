package solve

import "github.com/karlbe/gnistaEngine/pkg/input"

type seg struct {
	name string
	n    int
}

// play steps the game through a route like "right:125,none:50" without the bot's fighting.
func play(b *Bot, route string) {
	names := map[string]input.Actions{"none": 0, "up": input.Up, "down": input.Down, "left": input.Left, "right": input.Right, "fire": input.Fire}
	for _, part := range splitRoute(route) {
		for i := 0; i < part.n; i++ {
			b.S.Step(names[part.name])
		}
	}
}

func splitRoute(r string) []seg {
	var out []seg
	name, num, inNum := "", 0, false
	flush := func() {
		if name != "" {
			out = append(out, seg{name, num})
		}
		name, num, inNum = "", 0, false
	}
	for _, c := range r + "," {
		switch {
		case c == ',':
			flush()
		case c == ':':
			inNum = true
		case inNum:
			num = num*10 + int(c-'0')
		default:
			name += string(c)
		}
	}
	return out
}
