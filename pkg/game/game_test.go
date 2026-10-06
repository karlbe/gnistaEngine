package game

import (
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/input"
)

func load(t *testing.T) (Content, *content.Extracted) {
	l := content.Loader{Root: filepath.Join("..", "..", "assets-local")}
	e, err := l.Extracted()
	if err != nil {
		t.Skip(err)
	}
	level, err := l.Level()
	if err != nil {
		t.Skip(err)
	}
	prog, err := l.Program()
	if err != nil {
		t.Fatal(err)
	}
	return Content{Program: prog, Level: level}, e
}

func TestStepIsDeterministic(t *testing.T) {
	c, e := load(t)
	seq := []input.Actions{0, input.Right, input.Right | input.Fire, input.Up, 0, input.Left, input.Down}
	a, b := New(c, e.StartX, e.StartY), New(c, e.StartX, e.StartY)
	for i := 0; i < 1000; i++ {
		in := seq[i/40%len(seq)]
		a.Step(in)
		b.Step(in)
	}
	if !reflect.DeepEqual(a.Engine.Slots, b.Engine.Slots) || a.Engine.G != b.Engine.G {
		t.Fatalf("states differ:\n%+v\n%+v", a.Engine.G, b.Engine.G)
	}
	if a.Tick != 1000 {
		t.Fatalf("tick = %d, want 1000", a.Tick)
	}
}

// The player walks right from the start and stops at the end of the bridge, like the
// original (docs/re/script-engine.md).
func TestWalkRightToWall(t *testing.T) {
	c, e := load(t)
	s := New(c, e.StartX, e.StartY)
	for i := 0; i < 250; i++ {
		s.Step(input.Right)
		if s.Halted != nil {
			t.Fatal(s.Halted)
		}
	}
	if got := s.Engine.G.ViewX; got != 8112 {
		t.Fatalf("view x = %d, want 8112", got)
	}
}

// Up the stairs at the end of the bridge, as recorded in the original: walk right to the
// wall, tap left to turn, hold up. The original ends at view (8040,1296).
func TestClimbStairs(t *testing.T) {
	c, e := load(t)
	s := New(c, e.StartX, e.StartY)
	run := func(in input.Actions, n int) {
		for i := 0; i < n; i++ {
			s.Step(in)
			if s.Halted != nil {
				t.Fatalf("tick %d: %v", s.Tick, s.Halted)
			}
		}
	}
	run(input.Right, 150)
	run(input.Left, 7)
	run(0, 50)
	run(input.Up, 150)
	run(0, 50)
	if g := s.Engine.G; g.ViewX != 8040 || g.ViewY != 1296 {
		t.Fatalf("view (%d,%d), want (8040,1296)", g.ViewX, g.ViewY)
	}
}

// The clock runs from 23:35:00 to 24:00:00, 25 minutes of frames, then the game ends.
func TestClockEndsGame(t *testing.T) {
	c := newClock()
	n := 0
	for !c.tick() {
		n++
	}
	if n+1 != 25*60*TicksPerSecond {
		t.Fatalf("time ran out after %d frames, want %d", n+1, 25*60*TicksPerSecond)
	}
}

// play runs "name:ticks" steps, e.g. "right:150,none:50".
func play(t *testing.T, s *State, route string) {
	t.Helper()
	names := map[string]input.Actions{"none": 0, "up": input.Up, "down": input.Down, "left": input.Left, "right": input.Right, "fire": input.Fire}
	for _, part := range strings.Split(route, ",") {
		name, n, _ := strings.Cut(part, ":")
		ticks, err := strconv.Atoi(n)
		if err != nil {
			t.Fatalf("bad step %q", part)
		}
		for i := 0; i < ticks; i++ {
			s.Step(names[name])
			if s.Halted != nil {
				t.Fatalf("%s: %v", part, s.Halted)
			}
		}
	}
}

// The walkthrough's first door: stairs, shoot the enemy, lift to floor 5, right to the
// door, up. The room is empty, as in the original; the clock stands still inside, and
// pulling down leaves.
func TestFirstDoor(t *testing.T) {
	c, e := loadWith(t, false) // the walkthrough is the original's route
	s := New(c, e.StartX, e.StartY)
	play(t, s, "right:125,none:50,left:8,none:50,up:150,left:10,none:75,fire:300,none:50,left:350,none:50,up:600,fire:20,none:200,right:150,none:50,up:20")
	if s.Room == nil {
		t.Fatalf("not in a room: view (%d,%d)", s.Engine.G.ViewX, s.Engine.G.ViewY)
	}
	if s.Room.Message != messageEmpty || s.Room.Picture != pictureEmpty {
		t.Fatalf("room %+v, want the empty room", *s.Room)
	}
	clock := s.Clock
	play(t, s, "none:100")
	if s.Clock != clock || !s.Room.Showing() {
		t.Fatalf("clock %+v -> %+v, showing %v", clock, s.Clock, s.Room.Showing())
	}
	s.SetImprovements(true) // leaving a room without ducking is one of them
	play(t, s, "down:1")
	if s.Room != nil {
		t.Fatal("still in the room after down")
	}
	// Down still held after leaving must not make the player duck; once released, down works again.
	pc := s.Engine.Slots[0].PC
	play(t, s, "down:30")
	if got := s.Engine.Slots[0].PC; got != pc {
		t.Fatalf("holding down after leaving the room changed the player's script from $%04X to $%04X", pc, got)
	}
	play(t, s, "none:5,down:20")
	if s.Engine.Slots[0].PC == pc {
		t.Fatal("down does nothing after being released")
	}
}

// After the clock runs out: flash, black, two pictures, then the game is finished.
func TestExplosionEnding(t *testing.T) {
	c, e := load(t)
	s := New(c, e.StartX, e.StartY)
	s.Engine.G.Outcome = OutcomeExplosion
	n := 0
	for !s.Finished() {
		s.Step(0)
		if n++; n > 10000 {
			t.Fatal("ending never finishes")
		}
	}
	if want := flashFrames + blackFrames + 2*pictureFrames; n != want {
		t.Fatalf("ending took %d frames, want %d", n, want)
	}
	if got := s.Ending.Steps[2].Picture; got != "ET" {
		t.Fatalf("first picture %q, want ET", got)
	}
}

// Walking plays footsteps (op79: effects 11-13 on channel 0, then op85 starts them once);
// firing plays the weapon's effect.
func TestSoundCommands(t *testing.T) {
	c, e := load(t)
	s := New(c, e.StartX, e.StartY)
	var loads, once int
	seen := map[uint8]bool{}
	for i := 0; i < 200; i++ {
		s.Step(input.Right)
		for _, cmd := range s.Sounds() {
			switch cmd.Op {
			case Load:
				loads++
				seen[cmd.Sample] = true
				if cmd.Channel != 0 {
					t.Errorf("footstep on channel %d", cmd.Channel)
				}
			case StartOnce:
				once++
			}
		}
	}
	if loads < 4 || once < 4 {
		t.Fatalf("walking 200 ticks gave %d loads and %d starts", loads, once)
	}
	for id := range seen {
		if id < 11 || id > 13 {
			t.Errorf("unexpected footstep effect %d", id)
		}
	}
	var shots []AudioCmd
	for i := 0; i < 30; i++ {
		s.Step(input.Fire)
		shots = append(shots, s.Sounds()...)
	}
	if len(shots) == 0 {
		t.Fatal("firing made no sound")
	}
	t.Logf("fire: %+v", shots)
}

func TestCheat(t *testing.T) {
	c, e := loadWith(t, true)
	s := New(c, e.StartX, e.StartY)
	s.Step(input.Cheat)
	g := s.Engine.G
	if g.WeaponsOwned != 7 || g.Cards != 0x1F || g.Charges != 9 || g.Lives != startLives || g.Health != 3 {
		t.Fatalf("cheat did not fill everything: %+v", g)
	}
	for i, a := range g.Ammo {
		if a.Rounds != a.MagSize || a.Mags != 9 {
			t.Errorf("weapon %d ammo %+v", i, a)
		}
	}
	m := &s.Map
	if !m.Discovered(0, 0) || !m.Discovered(m.W-1, m.H-1) || !m.Discovered(m.W/2, m.H/2) {
		t.Error("the cheat did not reveal the whole map")
	}
}

// A killed enemy's last frame stays in the playfield (state 4 bobs are stamped, not erased).
func TestCorpsesStay(t *testing.T) {
	c, e := load(t)
	s := New(c, e.StartX, e.StartY)
	s.Step(0)
	s.firstEnemy(s.freeSlot())
	for i := 0; i < 600 && len(s.Stamps) == 0; i++ {
		in := input.Fire
		if i/10%2 == 1 {
			in = 0
		}
		s.Step(in | input.Cheat)
	}
	if len(s.Stamps) == 0 {
		t.Fatal("the enemy was not killed or left no corpse")
	}
	n := len(s.Stamps)
	for i := 0; i < 50; i++ {
		s.Step(0)
	}
	if len(s.Stamps) != n {
		t.Fatalf("stamps went from %d to %d while standing still", n, len(s.Stamps))
	}
	t.Logf("%d stamps after %d ticks: %+v", n, s.Tick, s.Stamps)
}

func TestInvulnerableAndSpeedToggles(t *testing.T) {
	c, e := loadWith(t, true)
	s := New(c, e.StartX, e.StartY)
	s.Step(input.ToggleInvulnerable | input.ToggleSpeed)
	if !s.Invulnerable || !s.Fast || !s.Engine.G.Invulnerable {
		t.Fatalf("toggles not on: %v %v", s.Invulnerable, s.Fast)
	}
	s.Step(input.ToggleInvulnerable | input.ToggleSpeed) // held: no second toggle
	if !s.Invulnerable || !s.Fast {
		t.Fatal("held key toggled again")
	}
	s.Step(0)
	s.Step(input.ToggleSpeed)
	if s.Fast || !s.Invulnerable {
		t.Fatalf("speed should be off: fast %v inv %v", s.Fast, s.Invulnerable)
	}

	// Double speed walks twice as far in the same number of ticks.
	walk := func(fast bool) int16 {
		s := New(c, e.StartX, e.StartY)
		s.Fast = fast
		for i := 0; i < 60; i++ {
			s.Step(input.Right)
		}
		return s.Engine.Slots[0].X
	}
	x0 := int16(0x1FD0)
	if n, f := walk(false)-x0, walk(true)-x0; f < n*3/2 {
		t.Errorf("fast walked %d px against %d normally", f, n)
	}
}

func TestPauseToggles(t *testing.T) {
	c, e := loadWith(t, true)
	s := New(c, e.StartX, e.StartY)
	s.Step(input.Pause)
	if !s.Paused {
		t.Fatal("P did not pause")
	}
	x, clock := s.Engine.Slots[0].X, s.Clock
	for i := 0; i < 100; i++ {
		s.Step(input.Right | input.Fire)
	}
	if s.Engine.Slots[0].X != x || s.Clock != clock || !s.Paused {
		t.Fatal("the game moved while paused, or fire resumed it")
	}
	s.Step(0)
	s.Step(input.Pause)
	if s.Paused {
		t.Fatal("second P did not resume")
	}
}

// rollFrom taps down tapTicks ticks, tapAt ticks into a run to the right, then lets the
// player come to rest. It reports whether the player rolled (script 104) and where the
// player stopped.
func rollFrom(c Content, e *content.Extracted, tapAt, tapTicks int) (rolled bool, x int16) {
	s := New(c, e.StartX, e.StartY)
	// The sensors that look ahead are only refreshed when a tile boundary is crossed, so
	// during the first pixels the roll is refused, in the original too.
	for i := 0; i < 20; i++ {
		s.Step(input.Right)
	}
	for i := 0; i < 36+tapAt+tapTicks+40; i++ {
		in := input.Right
		if i >= tapAt && i < tapAt+tapTicks {
			in |= input.Down
		}
		if i > tapAt+tapTicks+40 {
			in = 0
		}
		s.Step(in)
		if pc, p := s.Engine.Slots[0].PC, s.Engine.Prog; pc >= p.Script(104) && pc < p.Script(105) {
			rolled = true
		}
	}
	for i := 0; i < 200; i++ {
		s.Step(0)
	}
	return rolled, s.Engine.Slots[0].X
}

// A short tap of down while running always starts a roll, and it starts at the run loop's
// own check, so the player ends up in the same place whenever in the stride the tap came
// (the original's roll leaves the player on the same 32-pixel lattice as walking).
func TestRollTapAlwaysRolls(t *testing.T) {
	c, e := loadWith(t, true)
	var first int16
	for at := 0; at < 34; at++ { // a whole 32-tick run cycle
		rolled, x := rollFrom(c, e, at, 1)
		if !rolled {
			t.Errorf("a 1-tick tap of down %d ticks into the stride did not roll", at)
			continue
		}
		if at == 0 {
			first = x
		}
		if (x-first)%32 != 0 {
			t.Errorf("a tap at %d ended at x=%d, off the 32-pixel lattice of the tap at 0 (x=%d)", at, x, first)
		}
	}
	if (first-8144)%16 != 0 {
		t.Errorf("the roll ended at x=%d, which is not a multiple of 16 from the start", first)
	}
}

func TestRollTapLeft(t *testing.T) {
	c, e := loadWith(t, true)
	for _, at := range []int{0, 7, 15, 23} { // not 31: that stride's roll check meets a wall two tiles ahead
		s := New(c, e.StartX, e.StartY)
		for i := 0; i < 20; i++ {
			s.Step(input.Left)
		}
		rolled := false
		for i := 0; i < 36+at+40; i++ {
			in := input.Left
			if i == at {
				in |= input.Down
			}
			s.Step(in)
			if pc, p := s.Engine.Slots[0].PC, s.Engine.Prog; pc >= p.Script(105) && pc < p.Script(106) {
				rolled = true
			}
		}
		if !rolled {
			t.Errorf("a tap of down %d ticks into the left run cycle did not roll", at)
		}
	}
}

// loadWith is load with the improvements switched on or off.
func loadWith(t *testing.T, improve bool) (Content, *content.Extracted) {
	c, e := load(t)
	c.Improvements = improve
	return c, e
}

// Up and down at stairs work whichever way the player faces. Without improvements they only
// work facing the stairs, as in the original.
func TestImproveStairsEitherFacing(t *testing.T) {
	climb := func(improve bool, route string, in string) (before, after int16) {
		c, e := loadWith(t, improve)
		s := New(c, e.StartX, e.StartY)
		for _, a := range stepsOf(route) {
			s.Step(a)
		}
		before = s.Engine.G.ViewY
		for _, a := range stepsOf(in) {
			s.Step(a)
		}
		return before, s.Engine.G.ViewY
	}
	// At the foot, facing right with the stairs on the left.
	foot := "right:125,none:50,left:40,none:50,right:40,none:60"
	if b, a := climb(false, foot, "up:150,none:50"); a != b {
		t.Errorf("original: up facing away from the stairs moved the view %d -> %d", b, a)
	}
	if b, a := climb(true, foot, "up:150,none:50"); a >= b {
		t.Errorf("improved: up facing away from the stairs did not climb (%d -> %d)", b, a)
	}
	// At the top, facing left.
	top := "right:125,none:50,left:8,none:50,up:150,none:50"
	if b, a := climb(false, top, "down:150,none:50"); a != b {
		t.Errorf("original: down at the top moved the view %d -> %d", b, a)
	}
	if b, a := climb(true, top, "down:150,none:50"); a <= b {
		t.Errorf("improved: down at the top did not descend (%d -> %d)", b, a)
	}
}

// The walkthrough up to the point where the player stands at the lift, facing left.
const toTheLift = "right:125,none:50,left:8,none:50,up:150,left:10,none:75,fire:300,none:50,left:350,none:50"

// rideLift calls the lift and rides until the first stop of the cabin's ride script (script
// 93), then plays exit, and returns the state a long while after.
func rideLift(t *testing.T, improve bool, exit string) *State {
	c, e := loadWith(t, improve)
	s := New(c, e.StartX, e.StartY)
	for _, a := range stepsOf(toTheLift) {
		s.Step(a)
	}
	p := s.Engine.Prog
	for i := 0; i < 600; i++ {
		s.Step(input.Up)
		if pc := s.Engine.Slots[0].PC; pc >= p.Script(93) && pc < p.Script(94) {
			break
		}
	}
	if pc := s.Engine.Slots[0].PC; pc < p.Script(93) || pc >= p.Script(94) {
		t.Fatalf("the player never started riding (script $%04X)", pc)
	}
	for _, a := range stepsOf("none:20," + exit + ",none:500") {
		s.Step(a)
	}
	return s
}

// Left or right leaves the lift at the next stop, like fire. Without improvements only fire
// does, and the ride goes on to the top.
func TestImproveLeaveLiftWithAnyButton(t *testing.T) {
	top := rideLift(t, false, "right:3").Engine.G.ViewY
	for _, exit := range []string{"right:3", "left:3", "fire:3"} {
		if y := rideLift(t, true, exit).Engine.G.ViewY; y <= top {
			t.Errorf("improved: %s did not leave the lift early (view y %d, the top is %d)", exit, y, top)
		}
	}
	// The original looks for fire only now and then, so it is held.
	if y := rideLift(t, false, "fire:45").Engine.G.ViewY; y <= top {
		t.Errorf("original: fire did not leave the lift early (view y %d, the top is %d)", y, top)
	}
}

// After a lift ride the player came out facing left. With improvements, leaving with right
// leaves the player facing right, so running right starts at once instead of turning
// round first.
func TestImproveFaceRightAfterLift(t *testing.T) {
	runStart := func(s *State) int {
		p := s.Engine.Prog
		for i := 1; i <= 80; i++ {
			s.Step(input.Right)
			if pc := s.Engine.Slots[0].PC; pc >= p.Script(59) && pc < p.Script(62) {
				return i
			}
		}
		return -1
	}
	orig := rideLift(t, false, "fire:45")
	if orig.Engine.G.MoveBits&2 == 0 {
		t.Error("original: the player does not come out of the lift facing left")
	}
	turn := runStart(orig)
	imp := rideLift(t, true, "right:3")
	if imp.Engine.G.MoveBits&2 != 0 {
		t.Error("improved: leaving with right did not leave the player facing right")
	}
	if fast := runStart(imp); fast < 0 || fast >= turn {
		t.Errorf("improved: running right started after %d ticks, the original turn takes %d", fast, turn)
	}
	// Leaving with left or fire still leaves the player facing left.
	if rideLift(t, true, "left:3").Engine.G.MoveBits&2 == 0 {
		t.Error("improved: leaving with left should face left")
	}
}

// Without the improvements the cheats, the roll buffer and the pause toggle do nothing, and
// the original's pause (P held, until fire) works.
func TestOriginalBehaviourWithoutImprovements(t *testing.T) {
	c, e := loadWith(t, false)
	s := New(c, e.StartX, e.StartY)
	s.Step(input.Cheat | input.ToggleInvulnerable | input.ToggleSpeed)
	if s.Engine.G.WeaponsOwned != 1 || s.Invulnerable || s.Fast {
		t.Fatal("a cheat worked without the improvements")
	}
	// The original: P pauses ($7602) and the frame interrupt waits for fire.
	s.Step(input.Pause)
	s.Step(0)
	if !s.Paused {
		t.Fatal("P did not pause")
	}
	x := s.Engine.Slots[0].X
	for i := 0; i < 20; i++ {
		s.Step(input.Right)
	}
	if s.Engine.Slots[0].X != x || !s.Paused {
		t.Fatal("the game moved while paused, or something but fire ended the pause")
	}
	s.Step(input.Fire)
	if s.Paused {
		t.Fatal("fire did not end the pause")
	}
}
