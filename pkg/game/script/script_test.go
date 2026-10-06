package script

import (
	"encoding/binary"
	"errors"
	"testing"
)

// build returns a program whose script n starts at the given bytecode.
func build(t *testing.T, scripts map[int][]byte) *Program {
	t.Helper()
	code := make([]byte, ScriptTable+NumScripts*4+0x1000)
	next := ScriptTable + NumScripts*4
	end := next
	for n := 0; n < NumScripts; n++ { // default: every script just yields forever
		binary.BigEndian.PutUint32(code[ScriptTable+n*4:], uint32(end))
	}
	code[end] = 0
	end++
	for n, bc := range scripts {
		binary.BigEndian.PutUint32(code[ScriptTable+n*4:], uint32(end))
		copy(code[end:], bc)
		end += len(bc)
	}
	_ = next
	p, err := NewProgram(code)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type fakeEnv struct {
	joy   uint8
	key   uint8
	tiles map[[2]int]uint8
	def   uint8
	hit   bool
}

func (e *fakeEnv) Joystick() uint8               { return e.joy }
func (e *fakeEnv) LastKey() uint8                { return e.key }
func (e *fakeEnv) HelperActor() *Actor           { return &Actor{} }
func (e *fakeEnv) EnableChannel(int)             {}
func (e *fakeEnv) Door()                         {}
func (e *fakeEnv) DoorBlown()                    {}
func (e *fakeEnv) LiftCard(int) uint8            { return 0 }
func (e *fakeEnv) MatrixCols() int               { return 29 }
func (e *fakeEnv) Sound(int, uint8)              {}
func (e *fakeEnv) TakeRollRequest() bool         { return false }
func (e *fakeEnv) LiftExitRequest() (bool, bool) { return false, false }
func (e *fakeEnv) SilenceChannel(int)            {}

// Tile: with the view at (0,0), world tiles equal slot tile + offset.
func (e *fakeEnv) Tile(x, y int) uint8 {
	if t, ok := e.tiles[[2]int{x, y}]; ok {
		return t
	}
	return e.def
}

func newVM(t *testing.T, scripts map[int][]byte, env *fakeEnv) *VM {
	vm := &VM{Prog: build(t, scripts), Env: env}
	vm.Slots[0] = Slot{PC: vm.Prog.Script(0), Actor: &Actor{}, XMask: 0x8000, YMask: 0x8000, TileX: 10, TileY: 6}
	return vm
}

func TestYieldAndFrames(t *testing.T) {
	// frames 6/7 (op01), wait; frames 9/9 (op16 -> +394), wait; goto self. The goto and the
	// first op01 run in the same frame, so the third frame shows 6 again.
	vm := newVM(t, map[int][]byte{0: {1, 6, 7, 0, 16, 9, 9, 0, 34, 0}}, &fakeEnv{def: 115})
	for i, want := range []uint16{6, 394 + 9, 6, 394 + 9} {
		if err := vm.Frame(); err != nil {
			t.Fatal(err)
		}
		if got := vm.Slots[0].Actor.Upper.Frame; got != want {
			t.Fatalf("frame %d: upper = %d, want %d", i, got, want)
		}
	}
}

func TestDirectionalFrame(t *testing.T) {
	vm := newVM(t, map[int][]byte{0: {21, 10, 20, 2, 1, 2, 0}}, &fakeEnv{def: 115})
	vm.G.Weapon = 3
	if err := vm.Frame(); err != nil {
		t.Fatal(err)
	}
	a := vm.Slots[0].Actor
	if a.Upper.Frame != 1+0x74+30 || a.Lower.Frame != 2+0x74+60 {
		t.Fatalf("frames %d/%d", a.Upper.Frame, a.Lower.Frame)
	}
}

func TestWalkBlockedByWall(t *testing.T) {
	// script 0: if right and free -> script 1, else wait and retry. script 1: marks with frame 1.
	scripts := map[int][]byte{0: {45, 1, 0, 34, 0}, 1: {16, 1, 1, 0, 34, 1}}
	env := &fakeEnv{def: 115, joy: JoyRight}
	vm := newVM(t, scripts, env)
	vm.Slots[0].Sensors[SensSide] = 3 // wall: tile id below 11
	if err := vm.Frame(); err != nil {
		t.Fatal(err)
	}
	if vm.Slots[0].Actor.Upper.Frame != 0 {
		t.Fatal("walked into a wall")
	}
	vm.Slots[0].Sensors[SensSide] = 115
	if err := vm.Frame(); err != nil {
		t.Fatal(err)
	}
	if vm.Slots[0].Actor.Upper.Frame != 0x18A+1 {
		t.Fatal("did not start walking with a free path")
	}
}

func TestStepRefreshesSensorsAtTileBoundary(t *testing.T) {
	env := &fakeEnv{def: 115, tiles: map[[2]int]uint8{{12, 6}: 5, {11, 7}: 40}}
	// move right 16 times, one pixel per frame
	vm := newVM(t, map[int][]byte{0: {22, 0, 34, 0}}, env)
	for i := 0; i < 15; i++ {
		if err := vm.Frame(); err != nil {
			t.Fatal(err)
		}
		if vm.Slots[0].Sensors[SensSide] != 0 {
			t.Fatalf("sensors refreshed early, after %d pixels", i+1)
		}
	}
	if err := vm.Frame(); err != nil {
		t.Fatal(err)
	}
	s := vm.Slots[0]
	if s.X != 16 || s.XMask != 0x8000 {
		t.Fatalf("x=%d mask=%#x", s.X, s.XMask)
	}
	if s.Sensors[SensSide] != 5 || s.Sensors[SensBelow] != 40 || s.Sensors[SensHere] != 115 {
		t.Fatalf("sensors %v", s.Sensors)
	}
}

func TestMovementOpcodes(t *testing.T) {
	vm := newVM(t, map[int][]byte{0: {28, 0xFF, 2, 9, 0, 33, 0, 29, 0}}, &fakeEnv{def: 115})
	// op33 sets dy=1 and clears the moving flag but, like the original, keeps $8F5F.
	steps := []Globals{{Dx: -1, Dy: 2, MoveCount: 9, Moving: 1}, {Dx: 0, Dy: 1, MoveCount: 9}, {}}
	for i, want := range steps {
		if err := vm.Frame(); err != nil {
			t.Fatal(err)
		}
		g := vm.G
		if g.Dx != want.Dx || g.Dy != want.Dy || g.MoveCount != want.MoveCount || g.Moving != want.Moving {
			t.Fatalf("step %d: %+v, want %+v", i, g, want)
		}
	}
}

func TestGosubReturn(t *testing.T) {
	vm := newVM(t, map[int][]byte{0: {35, 1, 17, 2, 0}, 1: {17, 1, 0, 36}}, &fakeEnv{def: 115})
	for i, want := range []uint16{0x74 + 1, 0x74 + 2} {
		if err := vm.Frame(); err != nil {
			t.Fatal(err)
		}
		if got := vm.Slots[0].Actor.Upper.Frame; got != want {
			t.Fatalf("frame %d: %d, want %d", i, got, want)
		}
	}
}

// Bytes past the opcode table are reported, not guessed at.
func TestUnknownOpcode(t *testing.T) {
	vm := newVM(t, map[int][]byte{0: {91}}, &fakeEnv{def: 115})
	var u ErrUnimplemented
	if err := vm.Frame(); !errors.As(err, &u) || u.Op != 91 {
		t.Fatalf("err = %v", err)
	}
}

func TestNoYieldIsAnError(t *testing.T) {
	vm := newVM(t, map[int][]byte{0: {34, 0}}, &fakeEnv{def: 115})
	if err := vm.Frame(); err == nil {
		t.Fatal("expected an error for a script that never yields")
	}
}
