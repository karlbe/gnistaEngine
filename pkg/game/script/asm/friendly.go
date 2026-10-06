package asm

import (
	"fmt"
	"strconv"
	"strings"
)

// The friendly language. Scripts are written with plain words and named values; this file
// turns them into the engine's instructions (see ops in asm.go), which are still accepted as
// they are for anything the friendly forms do not cover. docs/script-reference.md describes
// the language; in short:
//
//	const walk_ticks = 4                 a named number
//	sound pistol = 0                     a named sound effect
//	pose stand_right legs=0 body=64      a named pair of pictures
//
//	script walk_right
//	    camera follow right              the view follows the player, one pixel a tick
//	    repeat 8 times as i              i counts 0 to 7
//	        hold legs=1+i body=64 for walk_ticks ticks moving right
//	    end
//	    if holding right and path_clear goto walk_right
//	    camera stop
//	    resume controls right

type pose struct{ legs, body, offset string }

// lowStmt is a statement in the engine's own words, with the line it came from.
type lowStmt = stmt

type lowerCtx struct {
	file   string
	consts map[string]int
	poses  map[string]pose
	vars   map[string]int
}

func (c *lowerCtx) errf(line int, format string, a ...any) error {
	return fmt.Errorf("%s:%d: %s", c.file, line, fmt.Sprintf(format, a...))
}

// number evaluates a number: digits, a named constant, a repeat variable, or sums and
// differences of those ("walk0+i", "0x10", "-3"). ok is false if a name is unknown.
func (c *lowerCtx) number(tok string) (int, bool) {
	total, sign, cur := 0, 1, ""
	flush := func() bool {
		if cur == "" {
			return true
		}
		v, ok := c.term(cur)
		if !ok {
			return false
		}
		total += sign * v
		cur = ""
		return true
	}
	for i, r := range tok {
		if (r == '+' || r == '-') && i > 0 {
			if !flush() {
				return 0, false
			}
			sign = 1
			if r == '-' {
				sign = -1
			}
			continue
		}
		if r == '-' && i == 0 {
			sign = -1
			continue
		}
		cur += string(r)
	}
	if !flush() {
		return 0, false
	}
	return total, true
}

func (c *lowerCtx) term(s string) (int, bool) {
	if strings.Contains(s, "*") { // products bind tighter than sums
		prod := 1
		for _, f := range strings.Split(s, "*") {
			v, ok := c.term(f)
			if !ok {
				return 0, false
			}
			prod *= v
		}
		return prod, true
	}
	if v, err := strconv.ParseInt(s, 0, 32); err == nil {
		return int(v), true
	}
	if v, ok := c.vars[s]; ok {
		return v, true
	}
	v, ok := c.consts[s]
	return v, ok
}

// val turns a token into an operand: a number if it can be evaluated, otherwise unchanged (a
// script name).
func (c *lowerCtx) val(tok string) string {
	if v, ok := c.number(tok); ok {
		return strconv.Itoa(v)
	}
	return tok
}

// keywords splits tokens into positional words and key=value pairs.
func keywords(f []string) (pos []string, kw map[string]string) {
	kw = map[string]string{}
	for _, t := range f {
		if k, v, ok := strings.Cut(t, "="); ok && k != "" {
			kw[k] = v
		} else {
			pos = append(pos, t)
		}
	}
	return pos, kw
}

var directions = map[string][2]int{
	"right": {1, 0}, "left": {-1, 0}, "up": {0, -1}, "down": {0, 1},
	"up-right": {1, -1}, "up-left": {-1, -1}, "down-right": {1, 1}, "down-left": {-1, 1},
}

var stepOp = map[string]string{"right": "stepr", "left": "stepl", "up": "stepu", "down": "stepd"}

func low(line int, f ...string) lowStmt { return lowStmt{line, f} }

// lowerStmt translates one friendly statement into engine statements. ok is false if the verb
// is not a friendly one, and the statement then goes through as it is.
func (c *lowerCtx) lowerStmt(line int, f []string) (out []lowStmt, ok bool, err error) {
	emit := func(fields ...string) { out = append(out, low(line, fields...)) }
	waits := func(n int) {
		for i := 0; i < n; i++ {
			emit("wait")
		}
	}
	num := func(tok string) (int, error) {
		v, ok := c.number(tok)
		if !ok {
			return 0, c.errf(line, "%q is not a number or a named value", tok)
		}
		return v, nil
	}
	rest := f[1:]
	switch f[0] {
	case "wait":
		n := 1
		if len(rest) > 0 {
			if n, err = num(rest[0]); err != nil {
				return nil, true, err
			}
		}
		waits(n)
		return out, true, nil

	case "show", "hold":
		if f[0] == "show" && len(rest) == 1 && rest[0] == "actor" {
			emit("show")
			return out, true, nil
		}
		pos, kw := keywords(rest)
		p := pose{kw["legs"], kw["body"], kw["offset"]}
		var i int
		if len(pos) > 0 && pos[0] != "for" {
			named, found := c.poses[pos[0]]
			if !found {
				return nil, true, c.errf(line, "no pose named %q (a pose is declared with: pose NAME legs=A body=B)", pos[0])
			}
			p = named
			i = 1
		}
		if p.legs == "" || p.body == "" {
			return nil, true, c.errf(line, "%s needs a pose or legs=... body=...", f[0])
		}
		op := "frames"
		switch p.offset {
		case "", "0":
		case "116":
			op = "frames116"
		case "254":
			op = "frames254"
		default:
			return nil, true, c.errf(line, "offset must be 0, 116 or 254")
		}
		emit(op, c.val(p.legs), c.val(p.body))
		if f[0] == "show" {
			return out, true, nil
		}
		// hold ... for N ticks [moving DIR]
		pos = pos[i:]
		if len(pos) < 2 || pos[0] != "for" {
			return nil, true, c.errf(line, "hold needs: for N ticks")
		}
		n, err := num(pos[1])
		if err != nil {
			return nil, true, err
		}
		dir, every := "", 1
		for j := 2; j < len(pos); j++ {
			switch pos[j] {
			case "tick", "ticks":
			case "moving":
				if j+1 >= len(pos) {
					return nil, true, c.errf(line, "moving needs a direction")
				}
				dir = pos[j+1]
				j++
			case "every": // every N ticks: the step is taken on the first of each N ticks
				if j+1 >= len(pos) {
					return nil, true, c.errf(line, "every needs a number of ticks")
				}
				if every, err = num(pos[j+1]); err != nil || every < 1 {
					return nil, true, c.errf(line, "every needs a number of ticks, 1 or more")
				}
				j++
			default:
				return nil, true, c.errf(line, "unexpected %q", pos[j])
			}
		}
		var steps []string
		if dir != "" {
			for _, d := range strings.Split(dir, "-") { // up-right is a step up and a step right
				step, found := stepOp[d]
				if !found {
					return nil, true, c.errf(line, "cannot move %q (right, left, up, down, or up-right and so on)", dir)
				}
				steps = append(steps, step)
			}
		}
		for k := 0; k < n; k++ {
			if k%every == 0 {
				for _, st := range steps {
					emit(st)
				}
			}
			emit("wait")
		}
		return out, true, nil

	case "place":
		if len(rest) == 1 && rest[0] == "charge" {
			emit("charge_place")
			return out, true, nil
		}
		if len(rest) != 3 || (rest[0] != "legs" && rest[0] != "body") {
			return nil, true, c.errf(line, "place legs|body X Y")
		}
		emit(rest[0]+"_at", c.val(rest[1]), c.val(rest[2]))
		return out, true, nil

	case "move":
		if len(rest) < 1 {
			return nil, true, c.errf(line, "move DIRECTION [pixels]")
		}
		step, found := stepOp[rest[0]]
		if !found {
			return nil, true, c.errf(line, "cannot move %q (right, left, up or down)", rest[0])
		}
		n := 1
		if len(rest) > 1 && rest[1] != "camera" {
			if n, err = num(rest[1]); err != nil {
				return nil, true, err
			}
		}
		withCamera := rest[len(rest)-1] == "camera" // the view moves a pixel with each step
		nudge := map[string]string{"right": "scroll_r", "left": "scroll_l", "up": "scroll_u", "down": "scroll_d"}[rest[0]]
		for i := 0; i < n; i++ {
			emit(step)
			if withCamera {
				emit(nudge)
			}
			emit("wait")
		}
		return out, true, nil

	case "step":
		step, found := stepOp[strings.Join(rest, "")]
		if !found {
			return nil, true, c.errf(line, "step right|left|up|down (one pixel, no waiting)")
		}
		emit(step)
		return out, true, nil

	case "start":
		// start sound [channel N]: the effect that was loaded (by fire weapon, say) begins
		ch := "0"
		if len(rest) == 3 && rest[1] == "channel" {
			ch = c.val(rest[2])
		} else if len(rest) != 1 || rest[0] != "sound" {
			return nil, true, c.errf(line, "start sound [channel N]")
		}
		emit("silence" + ch)
		return out, true, nil

	case "cabin":
		if len(rest) == 2 && rest[0] == "picture" {
			emit("legs_622", c.val(rest[1]))
			return out, true, nil
		}
		return nil, true, c.errf(line, "cabin picture N (the lift cabin, and the charge, are drawn with frame 622 + N)")

	case "camera":
		switch {
		case len(rest) >= 2 && rest[0] == "follow":
			d, found := directions[rest[1]]
			if !found {
				return nil, true, c.errf(line, "unknown direction %q", rest[1])
			}
			every, speed := 1, 1
			for j := 2; j+1 < len(rest); j += 2 {
				v, err := num(rest[j+1])
				if err != nil {
					return nil, true, err
				}
				switch rest[j] {
				case "every": // one move every N ticks
					every = v
				case "speed": // N pixels at a time
					speed = v
				default:
					return nil, true, c.errf(line, "camera follow DIRECTION [every N ticks] [speed N]")
				}
			}
			emit("scroll", strconv.Itoa(d[0]*speed), strconv.Itoa(d[1]*speed), strconv.Itoa(every-1))
		case len(rest) == 1 && rest[0] == "stop":
			emit("scroll_stop")
		case len(rest) >= 2 && rest[0] == "nudge":
			name := map[string]string{"right": "scroll_r", "left": "scroll_l", "up": "scroll_u", "down": "scroll_d"}[rest[1]]
			if name == "" {
				return nil, true, c.errf(line, "nudge right, left, up or down")
			}
			emit(name)
		default:
			return nil, true, c.errf(line, "camera follow DIRECTION [every N ticks] | camera stop | camera nudge DIRECTION")
		}
		return out, true, nil

	case "face":
		if len(rest) != 1 || (rest[0] != "left" && rest[0] != "right") {
			return nil, true, c.errf(line, "face left|right")
		}
		emit("face_" + rest[0])
		return out, true, nil

	case "play":
		if len(rest) < 1 {
			return nil, true, c.errf(line, "play SOUND [channel N] [looping]")
		}
		ch, looping := 0, false
		for j := 1; j < len(rest); j++ {
			switch rest[j] {
			case "channel":
				if j+1 >= len(rest) {
					return nil, true, c.errf(line, "channel needs a number, 0-3")
				}
				if ch, err = num(rest[j+1]); err != nil {
					return nil, true, err
				}
				j++
			case "looping":
				looping = true
			default:
				return nil, true, c.errf(line, "play SOUND [channel N] [looping]: unexpected %q", rest[j])
			}
		}
		if ch < 0 || ch > 3 {
			return nil, true, c.errf(line, "channel is 0-3")
		}
		if looping && ch != 1 {
			return nil, true, c.errf(line, "only channel 1 can loop")
		}
		emit("sound"+strconv.Itoa(ch), c.val(rest[0]))
		if looping {
			emit("start1")
		} else {
			emit("silence" + strconv.Itoa(ch))
		}
		return out, true, nil

	case "footstep":
		emit("step_sound")
		emit("silence0")
		return out, true, nil
	case "stairstep":
		emit("stair_sound")
		emit("silence0")
		return out, true, nil

	case "goto", "call":
		if len(rest) != 1 {
			return nil, true, c.errf(line, "%s SCRIPT", f[0])
		}
		emit(f[0], c.val(rest[0]))
		return out, true, nil
	case "return":
		emit("return")
		return out, true, nil

	case "controls":
		// controls facing right|left  right=A left=B up_stairs=C ... default=D
		pos, kw := keywords(rest)
		if len(pos) != 2 || pos[0] != "facing" || (pos[1] != "right" && pos[1] != "left") {
			return nil, true, c.errf(line, "controls facing right|left KEY=SCRIPT ...")
		}
		keys := []string{"right", "left", "up_stairs", "up_ladder", "lift_call", "lift_ready", "charge", "down_stairs", "down_ladder", "duck", "after_roll", "fire"}
		def, haveDef := kw["default"]
		fields := []string{"control_" + pos[1]}
		for _, k := range keys {
			v, set := kw[k]
			if !set {
				if !haveDef {
					return nil, true, c.errf(line, "controls needs %s=... (or default=... for all you leave out)", k)
				}
				v = def
			}
			fields = append(fields, c.val(v))
			delete(kw, k)
		}
		delete(kw, "default")
		for k := range kw {
			return nil, true, c.errf(line, "controls has no %q (keys: %s)", k, strings.Join(keys, ", "))
		}
		emit(fields...)
		return out, true, nil

	case "branch":
		_, kw := keywords(rest)
		switch {
		case kw["fire"] != "" && kw["idle"] != "":
			emit("if_fire", c.val(kw["fire"]), c.val(kw["idle"]))
		case kw["fire"] != "" && kw["down"] != "":
			emit("if_fire_down", c.val(kw["fire"]), c.val(kw["down"]))
		default:
			return nil, true, c.errf(line, "branch fire=A idle=B | branch fire=A down=B")
		}
		return out, true, nil

	case "if":
		if len(rest) == 6 && rest[0] == "holding" && (rest[1] == "right" || rest[1] == "left") && rest[2] == "and" && rest[3] == "path_clear" && rest[4] == "goto" {
			emit("if_"+rest[1]+"_free", c.val(rest[5]))
			return out, true, nil
		}
		if len(rest) == 3 && rest[0] == "rolling" && rest[1] == "goto" {
			emit("if_roll", c.val(rest[2]))
			return out, true, nil
		}
		return nil, true, c.errf(line, "if holding right|left and path_clear goto SCRIPT | if rolling goto SCRIPT")

	case "check":
		if len(rest) < 1 || rest[0] != "hits" {
			return nil, true, c.errf(line, "check hits damage=N right_hurt=A right_dead=B left_hurt=C left_dead=D")
		}
		_, kw := keywords(rest[1:])
		for _, k := range []string{"damage", "right_hurt", "right_dead", "left_hurt", "left_dead"} {
			if kw[k] == "" {
				return nil, true, c.errf(line, "check hits needs %s=", k)
			}
		}
		emit("take_hit", c.val(kw["damage"]), c.val(kw["right_hurt"]), c.val(kw["right_dead"]), c.val(kw["left_hurt"]), c.val(kw["left_dead"]))
		return out, true, nil
	case "back":
		if len(rest) == 2 && rest[0] == "from" && rest[1] == "hit" {
			emit("hit_return")
			return out, true, nil
		}
		return nil, true, c.errf(line, "back from hit")

	case "resume":
		if len(rest) == 2 && rest[0] == "controls" && (rest[1] == "right" || rest[1] == "left") {
			emit("resume_" + rest[1])
			return out, true, nil
		}
		return nil, true, c.errf(line, "resume controls right|left")

	case "shoot":
		pos, _ := keywords(rest)
		if len(pos) < 1 || (pos[0] != "right" && pos[0] != "left") {
			return nil, true, c.errf(line, "shoot right|left [second_death]")
		}
		name := "shoot_" + pos[0]
		if len(pos) > 1 && pos[1] == "second_death" {
			name = "shoot2_" + pos[0]
		}
		emit(name)
		return out, true, nil

	case "fire":
		if len(rest) >= 1 && rest[0] == "weapon" {
			_, kw := keywords(rest[1:])
			if kw["sound"] == "" || kw["empty"] == "" {
				return nil, true, c.errf(line, "fire weapon sound=S empty=SCRIPT")
			}
			emit("fire_weapon", c.val(kw["empty"]), c.val(kw["sound"]))
			return out, true, nil
		}

	case "weapon":
		if len(rest) >= 1 && rest[0] == "sprites" {
			_, kw := keywords(rest[1:])
			emit("setbase", c.val(kw["legs"]), c.val(kw["body"]))
			return out, true, nil
		}

	case "enemy":
		switch strings.Join(rest, " ") {
		case "shoots":
			emit("enemy_fire")
		case "revives":
			emit("revive")
		default:
			return nil, true, c.errf(line, "enemy shoots | enemy revives")
		}
		return out, true, nil

	case "remove":
		emit("deactivate")
		return out, true, nil
	case "game":
		if strings.Join(rest, " ") == "over explosion" {
			emit("explode_end")
			return out, true, nil
		}
	case "hide":
		emit("hide")
		return out, true, nil
	case "charge":
		name := map[string]string{"show": "charge_show", "explodes": "charge_blow"}[strings.Join(rest, " ")]
		if name == "" {
			return nil, true, c.errf(line, "charge show | charge explodes (and place charge)")
		}
		emit(name)
		return out, true, nil
	case "lift":
		pos, kw := keywords(rest)
		if len(pos) == 0 {
			return nil, true, c.errf(line, "lift choose|cabin|called|busy|gone|exit_check|board_up|board_down ...")
		}
		switch pos[0] {
		case "choose":
			emit("lift_choose", c.val(kw["up"]), c.val(kw["down"]))
		case "cabin":
			if len(pos) != 2 {
				return nil, true, c.errf(line, "lift cabin 0|1|2|3|hide|show|up|down")
			}
			switch pos[1] {
			case "0", "1", "2", "3":
				emit("cabin" + pos[1])
			case "hide", "show", "up", "down":
				emit("cabin_" + pos[1])
			default:
				return nil, true, c.errf(line, "lift cabin 0|1|2|3|hide|show|up|down")
			}
		case "called", "busy", "gone":
			emit("lift_" + pos[0])
		case "exit_check":
			emit("lift_exit_check")
		case "board_up":
			emit("lift_enter_up")
		case "board_down":
			emit("lift_enter_down")
		default:
			return nil, true, c.errf(line, "unknown lift word %q", pos[0])
		}
		return out, true, nil
	}
	return nil, false, nil
}
