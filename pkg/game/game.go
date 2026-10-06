// Package game holds the simulation state and rules. It must not depend on Ebitengine,
// rendering or audio, so that it stays deterministic, testable and reusable.
package game

import (
	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
	"github.com/karlbe/gnistaEngine/pkg/input"
)

// TicksPerSecond matches the original's PAL vertical blank rate.
const TicksPerSecond = 50

// Content is the static data a game runs on. It is not part of the state.
type Content struct {
	Program *script.Program
	Level   *world.Level

	// Improvements switches on the changes that are not in the original (see
	// script.VM.Improve). Settings, not state: it is not saved with a game.
	Improvements bool
}

// State is the simulation state.
//
// The engine still holds pointers (actor data, program, environment) at run time. Save and
// LoadState (savestate.go) turn them into plain data and back; a network game would use the
// same.
type State struct {
	Tick         uint64
	Last         input.Actions
	Engine       *script.VM
	Spawner      Spawner
	Clock        Clock
	Invulnerable bool                // cheat: shots and enemy contact do nothing
	Fast         bool                // cheat: the simulation runs two frames per tick (the clock does not)
	IgnoreDown   bool                // not in the original: down, held to leave a room, does nothing until released
	Paused       bool                // $7602; P toggles it (the original pauses while P is held, until fire)
	Rooms        [][roomRecLen]uint8 // the room records ($ACAC), changed as rooms are visited
	Room         *Visit              // the room on screen, if any
	Ending       *Ending             // the ending, once the game is over
	Stamps       []script.Sprite     // bobs drawn into the playfield for good (corpses, a blown door)
	Map          MapMemory           // not in the original: what the player has seen, for the minimap
	Halted       error               // the first unported opcode or routine a script reached this tick
	env          *env
	prog         *script.Program
	content      Content
}

// Original addresses used by the start-of-game setup ($258-$42C).
const (
	startLives = 9   // $1C02
	playerOffX = 160 // the player stands 10 tiles in and 6 down in the view
	playerOffY = 96
)

// Level returns the level the game runs on.
func (s *State) Level() *world.Level { return s.content.Level }

// New sets up a new game like the original's init code ($258-$42C).
func New(c Content, startX, startY int) *State {
	e := &env{level: c.Level}
	vm := &script.VM{Prog: c.Program, Env: e, Improve: c.Improvements}
	t := &c.Level.T
	x, y := int16(t.StartViewX+playerOffX), int16(t.StartViewY+playerOffY) // 0x1FD0, 0x5B0 in the original
	vm.Slots[0] = script.Slot{
		X: x, Y: y, XMask: 0x8000, YMask: 0x8000, PC: c.Program.Spec.Player, TileX: 10, TileY: 6,
		Sensors: [6]uint8{0xFF, 0xFF, 0xFF, 0xFF},
		Actor: &script.Actor{
			Upper: script.Sprite{X: x, Y: y, Frame: c.Program.Spec.PlayerFrames[0]},
			Lower: script.Sprite{X: x, Y: y - 16, Frame: c.Program.Spec.PlayerFrames[1]},
			State: 1,
		},
	}
	g := &vm.G
	g.ViewX, g.ViewY = int16(startX), int16(startY)
	g.WeaponsOwned, g.Charges, g.Moving = 1, 2, 1
	for i := range g.Ammo {
		g.Ammo[i].MagSize = c.Program.Spec.MagSize[i]
	}
	g.Ammo[0].Rounds, g.Ammo[0].Mags = 7, 5
	g.Cell, g.Lives = t.StartCell(c.Level.Cols), startLives // $4EE starts at 0x296
	s := &State{Engine: vm, Spawner: newSpawner(t), Clock: newClock(), env: e, prog: c.Program, content: c, Map: newMapMemory(c.Level)}
	e.onDoor, e.onBlown = s.enterRoom, s.doorBlown
	loadRooms(s)
	s.discover()
	return s
}

// Step advances the simulation by one tick like the original's frame ($7870): enemies
// ($508), actor scripts ($C36), scroll ($8888). An unported opcode stops that script where
// it is; the engine retries it every tick.
func (s *State) Step(in input.Actions) {
	s.env.audio = s.env.audio[:0]
	if s.Over() {
		if s.Ending == nil {
			s.Ending = s.newEnding()
		}
		s.Ending.step(in)
		return
	}
	s.Tick++
	pressed := in &^ s.Last // keys that went down this tick
	s.Last = in
	improve := s.Engine.Improve
	if improve { // cheats and the pause toggle are not in the original
		if in.Has(input.Cheat) {
			s.cheat()
		}
		if pressed.Has(input.ToggleInvulnerable) {
			s.Invulnerable = !s.Invulnerable
			s.Engine.G.Invulnerable = s.Invulnerable
		}
		if pressed.Has(input.ToggleSpeed) {
			s.Fast = !s.Fast
		}
		if pressed.Has(input.Pause) {
			s.Paused = !s.Paused
		}
		if s.Paused {
			return
		}
	}
	if s.Room != nil {
		s.stepRoom(in)
		return
	}
	if !improve && s.Paused { // the original: paused while P was held, until fire
		s.Paused = !in.Has(input.Fire)
		return
	}
	if s.IgnoreDown {
		if in.Has(input.Down) {
			in &^= input.Down
		} else {
			s.IgnoreDown = false
		}
	}
	s.env.in = in
	if pressed.Has(input.Fire | input.Left | input.Right) {
		s.env.exitBuf, s.env.exitRt = liftExitBuffer, in.Has(input.Right)
	} else if s.env.exitBuf > 0 {
		s.env.exitBuf--
	}
	if improve && pressed.Has(input.Down) && in&(input.Left|input.Right) != 0 {
		s.env.roll = rollBuffer
	} else if s.env.roll > 0 {
		s.env.roll--
	}
	s.frame()
	if s.Fast && !s.Over() && s.Room == nil {
		s.frame()
	}
	s.discover()
	if s.Clock.tick() {
		s.Engine.G.Outcome = 2
	}
	if !improve {
		s.Paused = in.Has(input.Pause) // $7602: the frame interrupt waits for fire while P is down
	}
}

// frame is the simulation part of a tick: enemies, actor scripts, scroll.
func (s *State) frame() {
	s.bobsErased()
	s.enemies()
	s.Halted = s.Engine.Frame()
	s.stamp()
}

// GiveEverything is a tool for searching the level: all cards and weapons, full ammunition and
// charges, and an unlimited clock. It is not an input; a game that used it cannot be replayed.
func (s *State) GiveEverything() {
	g := &s.Engine.G
	g.WeaponsOwned, g.Cards, g.Charges = 7, 0x1F, 9
	for i := range g.Ammo {
		g.Ammo[i].Rounds, g.Ammo[i].Mags = g.Ammo[i].MagSize, 9
	}
	for i := range s.Rooms {
		s.Rooms[i][recBlown] = 1 // every door open, so a map of the level shows them all
	}
	s.SetTimeLeft(1500)
}

// DropMapMemory forgets what the minimap has seen and stops recording it, which makes the
// state much smaller to copy. For searches that do not draw anything.
func (s *State) DropMapMemory() { s.Map = MapMemory{} }

// SetImprovements switches the changes that are not in the original on or off.
func (s *State) SetImprovements(on bool) {
	s.Engine.Improve = on
	if !on { // the cheats are part of the improvements
		s.Invulnerable, s.Fast, s.IgnoreDown = false, false, false
		s.Engine.G.Invulnerable = false
	}
}

// cheat gives all weapons, all five cards, nine charges, full magazines and nine spare ones,
// and full health, and reveals the whole level on the minimap. Not in the original. Health is the counter at $1C05 (it resets to 3 when
// a life is lost) and Lives the "hits" left at $1C02, which the first-aid room also resets.
func (s *State) cheat() {
	g := &s.Engine.G
	g.WeaponsOwned, g.Cards, g.Charges = 7, 0x1F, 9
	for i := range g.Ammo {
		g.Ammo[i].Rounds, g.Ammo[i].Mags = g.Ammo[i].MagSize, 9
		g.Ammo[i].HUD |= 1
	}
	g.Lives, g.Health = startLives, 3
	g.HUDDirty |= 5
	s.Map.reveal(0, 0, s.Map.W, s.Map.H)
}

// env connects the script engine to the level and the player's input.
type env struct {
	level   *world.Level
	in      input.Actions
	cabin   script.Actor
	onDoor  func()          // called by Door
	onBlown func()          // called by DoorBlown
	enemies [3]script.Actor // bob pairs $8170 + $30*k for slots 1-3
	audio   []AudioCmd      // this tick's sound commands
	roll    int             // ticks a press of down (with a direction) can still start a roll
	exitBuf int             // ticks a press of fire, left or right can still end a lift ride
	exitRt  bool            // that press was right
}

// liftExitBuffer is how long such a press waits for the lift ride's next look (op61). The
// ride looks every few ticks, but only about 36 ticks after it (re)starts at each floor.
const liftExitBuffer = 40

// LiftExitRequest reports a recent press of fire, left or right and clears it. Not in the
// original.
func (e *env) LiftExitRequest() (any, right bool) {
	any, right = e.exitBuf > 0, e.exitRt
	e.exitBuf = 0
	return
}

// rollBuffer is how long such a press waits for the run loop's roll check (op47), which
// comes once per 32-tick stride. The roll has to start there, as in the original: started
// anywhere else the player ends up off the 32-pixel lattice the doors and stairs are laid
// out for.
const rollBuffer = 40

// TakeRollRequest reports a buffered press of down with a direction and clears it. Not in
// the original. Together with the masking in Joystick it makes a tap of down while
// running start the roll at the next check.
func (e *env) TakeRollRequest() bool {
	t := e.roll > 0
	e.roll = 0
	return t
}

// AudioOp is what a script asks a Paula channel to do.
type AudioOp uint8

const (
	Load      AudioOp = iota // set the channel's registers to effect Sample with DMA off ($64FE...)
	Start                    // DMA on: the sample repeats until the channel is loaded again (op89)
	StartOnce                // DMA on, then length 1: the sample plays once (ops 85-88)
)

// AudioCmd is one sound command, in the order the scripts issued it. The simulation only
// records them; whoever plays sound reads Sounds after each Step.
type AudioCmd struct {
	Op      AudioOp
	Channel int
	Sample  uint8
}

// soundChannels maps the original's sound routines to Paula channels 0-3.
var soundChannels = map[int]int{0x64FE: 0, 0x652C: 1, 0x655A: 2, 0x6588: 3}

// Sounds returns the sound commands issued by the last Step. The slice is reused.
func (s *State) Sounds() []AudioCmd { return s.env.audio }

// Joystick encodes the input like $235C: one direction (up, down, right, left in that
// order of priority, or "none"), plus fire.
func (e *env) Joystick() uint8 {
	in := e.in
	// While a roll waits for op47, the stick reads the direction alone: down would take
	// priority over it and make the run loop's end-of-stride check stop the player.
	if e.roll > 0 && in&(input.Left|input.Right) != 0 {
		in &^= input.Down
	}
	return joystick(in)
}

func joystick(in input.Actions) uint8 {
	var j uint8
	switch {
	case in.Has(input.Up):
		j = script.JoyUp
	case in.Has(input.Down):
		j = script.JoyDown
	case in.Has(input.Right):
		j = script.JoyRight
	case in.Has(input.Left):
		j = script.JoyLeft
	default:
		j = script.JoyNone
	}
	if in.Has(input.Fire) {
		j |= script.JoyFire
	}
	return j
}

// LastKey returns the raw key code ($69AD) of a held game key. The original ($698A) reads
// the keyboard's last event every frame, so a released key reads as its code + $80, which
// matches nothing.
func (e *env) LastKey() uint8 {
	switch {
	case e.in.Has(input.Charge):
		return 0x40
	case e.in.Has(input.Weapon1):
		return 0x50
	case e.in.Has(input.Weapon2):
		return 0x51
	case e.in.Has(input.Weapon3):
		return 0x52
	}
	return 0
}

func (e *env) HelperActor() *script.Actor { return &e.cabin }
func (e *env) EnableChannel(ch int)       { e.audio = append(e.audio, AudioCmd{Start, ch, 0}) }
func (e *env) Door()                      { e.onDoor() }
func (e *env) DoorBlown()                 { e.onBlown() }
func (e *env) LiftCard(c int) uint8       { return e.level.T.LiftCard(c) }
func (e *env) MatrixCols() int            { return e.level.Cols }
func (e *env) SilenceChannel(ch int)      { e.audio = append(e.audio, AudioCmd{StartOnce, ch, 0}) }
func (e *env) Sound(routine int, id uint8) {
	e.audio = append(e.audio, AudioCmd{Load, soundChannels[routine], id})
}
func (e *env) Tile(x, y int) uint8 { return uint8(e.level.Tile(x, y)) }

// Over reports whether the game has ended: $21EA is set (op90 sets 2) and the original
// leaves the game loop for the ending screens, which are not ported yet.
func (s *State) Over() bool { return s.Engine.G.Outcome != 0 }

// Clock is the countdown shown as 23:MM:SS in the HUD ($761C, run after each frame). It
// starts at 23:35:00 ($76D4); at 24:00:00 the bombs go off ($21EA = 2), so a game lasts
// 25 minutes of frames.
type Clock struct {
	Frames, Sec, Sec10, Min, Min10 int16 // $785A-$7862
}

func newClock() Clock { return Clock{Min: 5, Min10: 3} }

// SetTimeLeft sets the clock so that this many seconds are left before 24:00:00. A tool for
// reaching the endings without playing 25 minutes.
func (s *State) SetTimeLeft(seconds int) {
	t := 3600 - min(max(seconds, 1), 1500) // seconds into the hour, from 23:35:00 on
	s.Clock = Clock{Min10: int16(t / 600), Min: int16(t / 60 % 10), Sec10: int16(t % 60 / 10), Sec: int16(t % 10)}
}

// FramesLeft is how many frames the clock still runs before the bombs go off: it counts
// from 23:35:00 to 24:00:00, 25 minutes of 50 frames a second. A query for tools.
func (c Clock) FramesLeft() int {
	elapsed := ((int(c.Min10)*10+int(c.Min)-35)*60+int(c.Sec10)*10+int(c.Sec))*50 + int(c.Frames)
	return 75000 - elapsed
}

// tick advances the clock by one frame and reports when time has run out.
func (c *Clock) tick() bool {
	if c.Frames++; c.Frames < 50 {
		return false
	}
	c.Frames = 0
	if c.Sec++; c.Sec < 10 {
		return false
	}
	c.Sec = 0
	if c.Sec10++; c.Sec10 < 6 {
		return false
	}
	c.Sec10 = 0
	if c.Min++; c.Min < 10 {
		return false
	}
	c.Min = 0
	if c.Min10++; c.Min10 < 6 {
		return false
	}
	c.Min10 = 0
	return true
}

// stampMargin is how far outside the view a stamp is kept, in pixels.
const stampMargin = 64

// stamp keeps the bobs that reached state 4 this frame. The original draws a bob in that
// state once without saving the background behind it ($7F12-$7F3E), so it is never erased
// and stays in the playfield bitmap: enemy corpses ($5921, op77) and the door a charge has
// blown (op74). Hypothesis: the bitmap redraws what scrolls in, so a stamp is dropped when
// it is more than stampMargin outside the view.
func (s *State) stamp() {
	vm := s.Engine
	if a := &s.env.cabin; a.State == 4 {
		s.Stamps = append(s.Stamps, a.Upper) // the helper is a single bob
	}
	for i := range s.env.enemies {
		if a := &s.env.enemies[i]; a.State == 4 {
			s.Stamps = append(s.Stamps, a.Upper, a.Lower)
		}
	}
	keep := s.Stamps[:0]
	for _, sp := range s.Stamps {
		if d := int(sp.X) - int(vm.G.ViewX); d > -stampMargin && d < 0x150+stampMargin {
			keep = append(keep, sp)
		}
	}
	s.Stamps = keep
}

// bobsErased is the part of the frame interrupt's bob drawing that changes state ($7E30,
// $7F12): at the start of each frame a bob that was hidden (2) or removed (4) has been
// erased from the screen and becomes empty (0).
func (s *State) bobsErased() {
	actors := []*script.Actor{&s.env.cabin, s.Engine.Slots[0].Actor}
	for i := range s.env.enemies {
		actors = append(actors, &s.env.enemies[i])
	}
	for _, a := range actors {
		if a != nil && (a.State == 2 || a.State == 4) {
			a.State = 0
		}
	}
}
