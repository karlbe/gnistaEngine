package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/input"
)

// Traces recorded from the original with tools/uae/capture_script.py.
type traceFile struct {
	Direction string  `json:"direction"`
	Seconds   float64 `json:"seconds"`
	Start     struct {
		Slot struct {
			X, Y         int16
			XMask        uint16 `json:"xmask"`
			YMask        uint16 `json:"ymask"`
			PC           int    `json:"pc"`
			TileX        int    `json:"tile_x"`
			TileY        int    `json:"tile_y"`
			Sensors      []uint8
			Upper, Lower struct{ X, Y, Frame int16 }
		} `json:"slot"`
		Globals struct {
			Dx, Dy         int16
			MoveCount      uint8 `json:"move_count"`
			MoveWait       uint8 `json:"move_wait"`
			Moving         uint8 `json:"moving"`
			FrameBaseUpper int16 `json:"frame_base_upper"`
			FrameBaseLower int16 `json:"frame_base_lower"`
			Weapon         int16 `json:"weapon"`
			WeaponsOwned   uint8 `json:"weapons_owned"`
			Charges        int16 `json:"charges"`
			ViewX          int16 `json:"view_x"`
			ViewY          int16 `json:"view_y"`
			Ammo           [][4]uint8
			MoveBits       uint8  `json:"move_bits"`
			Loop           *uint8 `json:"loop_117e"`
			Resume         int    `json:"resume"`
			ResumeLeft     int    `json:"resume_left"`
			Cell           *int   `json:"cell"`
			Lives          *int16 `json:"lives"`
			Health         int8   `json:"health"`
			Shooter        int    `json:"shooter_slot"`
			Climbing       int16  `json:"climbing"`
			Lift           uint8  `json:"lift"`
			Cards          uint8  `json:"cards"`
			Blocked        int16  `json:"blocked"`
			DownFlag       int16  `json:"down_flag"`
			FireLatch      int16  `json:"fire_latch"`
		} `json:"globals"`
		Spawner *struct {
			Cells   []uint8
			Wave    int16
			Left    int16
			Next    int
			Delays  []uint16
			Toggle  uint8
			Random  uint32
			Variant int
			OffR    int16 `json:"off_r"`
			OffL    int16 `json:"off_l"`
			Contact int16
		} `json:"spawner"`
		Enemies []*struct {
			PC, X, Y     int16
			XMask        uint16 `json:"xmask"`
			YMask        uint16 `json:"ymask"`
			TileX        int    `json:"tile_x"`
			TileY        int    `json:"tile_y"`
			Sensors      []uint8
			Dead         uint8
			Scripts      []int
			HP           uint16
			Upper, Lower struct{ X, Y, Frame, State int16 }
		} `json:"enemies"`
	} `json:"start"`
	Trace []struct {
		PC           int
		Upper, Lower int16
		Dx, X        int16
		E            []*[4]int
		Lives        *int16
		Shooter      int
		Outcome      int16
		ViewY        *int16 `json:"view_y"`
	} `json:"trace"`
}

// step is what the comparison looks at. Enemies, lives, shooter and outcome are only
// compared for traces that record them.
type step struct {
	PC                  int
	Upper, Lower, Dx, X int16
	E                   [4][4]int // slots 1-3 (pc, x, frames) and the cabin (pc, x, frame, state)
	Lives               int16
	Shooter             int
	Outcome             int16
	ViewY               int16
}

var noEnemy = [4]int{-1, -1, -1, -1}

var traceInput = map[string]input.Actions{
	"none": 0, "up": input.Up, "right": input.Right, "down": input.Down, "left": input.Left, "fire": input.Fire,
}

func TestAgainstOriginalTrace(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "assets-local", "uae", "trace_*.json"))
	if len(files) == 0 {
		t.Skip("no recorded traces")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) { compareTrace(t, file) })
	}
}

func compareTrace(t *testing.T, file string) {
	c, _ := load(t)
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var tr traceFile
	if err := json.Unmarshal(b, &tr); err != nil {
		t.Fatal(err)
	}
	in, ok := traceInput[tr.Direction]
	if !ok {
		t.Fatalf("unknown direction %q", tr.Direction)
	}

	extended := tr.Start.Spawner != nil
	withViewY := len(tr.Trace) > 0 && tr.Trace[0].ViewY != nil
	var orig []step
	for _, r := range tr.Trace {
		s := step{PC: r.PC, Upper: r.Upper, Lower: r.Lower, Dx: r.Dx, X: r.X}
		if extended {
			for k := range s.E {
				s.E[k] = noEnemy
				if k < len(r.E) && r.E[k] != nil {
					s.E[k] = *r.E[k]
				}
			}
			s.Lives, s.Shooter, s.Outcome = *r.Lives, r.Shooter, r.Outcome
			if r.ViewY != nil {
				s.ViewY = *r.ViewY
			}
		}
		// The capture also keys on view_x and may sample between the actor loop and the
		// scroll, which leaves repeated steps in the fields compared here.
		if len(orig) == 0 || s != orig[len(orig)-1] {
			orig = append(orig, s)
		}
	}

	st, g := tr.Start.Slot, tr.Start.Globals
	s := New(c, int(g.ViewX), int(g.ViewY))
	vm := s.Engine
	p := &vm.Slots[0]
	p.X, p.Y, p.XMask, p.YMask, p.PC, p.TileX, p.TileY = st.X, st.Y, st.XMask, st.YMask, st.PC, st.TileX, st.TileY
	copy(p.Sensors[:], st.Sensors)
	p.Actor.Upper = script.Sprite{X: st.Upper.X, Y: st.Upper.Y, Frame: uint16(st.Upper.Frame)}
	p.Actor.Lower = script.Sprite{X: st.Lower.X, Y: st.Lower.Y, Frame: uint16(st.Lower.Frame)}
	G := &vm.G
	G.Dx, G.Dy, G.MoveCount, G.MoveWait, G.Moving = g.Dx, g.Dy, g.MoveCount, g.MoveWait, g.Moving
	G.FrameBaseUpper, G.FrameBaseLower, G.Weapon, G.WeaponsOwned, G.Charges = g.FrameBaseUpper, g.FrameBaseLower, g.Weapon, g.WeaponsOwned, g.Charges
	G.MoveBits, G.Resume, G.ResumeLeft = g.MoveBits, g.Resume, g.ResumeLeft
	if g.Loop != nil && *g.Loop != 0xFF {
		G.Looping, G.LoopLeft = true, *g.Loop
	}
	for i, a := range g.Ammo {
		G.Ammo[i] = script.Ammo{Rounds: a[0], Mags: a[1], MagSize: a[2], HUD: a[3]}
	}
	if g.Cell != nil {
		G.Cell, G.Lives, G.Health, G.Shooter = *g.Cell, *g.Lives, g.Health, g.Shooter
		G.Climbing, G.Blocked, G.DownFlag, G.FireLatch = g.Climbing, g.Blocked, g.DownFlag != 0, g.FireLatch != 0
		G.LiftBusy, G.LiftCalled, G.Cards = g.Lift&1 != 0, g.Lift&2 != 0, g.Cards
	}
	if sp := tr.Start.Spawner; sp != nil {
		ours := &s.Spawner
		copy(ours.Cells[:], sp.Cells)
		copy(ours.Delays[:], sp.Delays)
		ours.Wave, ours.Left, ours.Next = sp.Wave != 0, sp.Left, sp.Next
		ours.Toggle, ours.Random, ours.Variant = sp.Toggle&1 != 0, sp.Random, sp.Variant
		ours.OffR, ours.OffL, ours.Contact = sp.OffR, sp.OffL, sp.Contact != 0
	} else {
		// Older traces: the spawner as at the start of the game, but with the player's
		// cell taken from the view.
		G.Cell = (int(g.ViewY)/16/4+1)*29 + int(g.ViewX)/16/20
	}

	for k, e := range tr.Start.Enemies {
		if e == nil {
			continue
		}
		if e.Scripts == nil {
			t.Skip("the trace starts with enemies on screen, whose records were not recorded")
		}
		sl := &vm.Slots[k+1]
		a := &s.env.enemies[k]
		a.Upper = script.Sprite{X: e.Upper.X, Y: e.Upper.Y, Frame: uint16(e.Upper.Frame)}
		a.Lower = script.Sprite{X: e.Lower.X, Y: e.Lower.Y, Frame: uint16(e.Lower.Frame)}
		a.State = uint16(e.Upper.State)
		sl.PC, sl.X, sl.Y, sl.XMask, sl.YMask, sl.TileX, sl.TileY = int(e.PC), e.X, e.Y, e.XMask, e.YMask, e.TileX, e.TileY
		copy(sl.Sensors[:], e.Sensors)
		sl.Dead, sl.Actor = e.Dead != 0, a
		copy(sl.Enemy.Scripts[:], e.Scripts)
		sl.Enemy.HP = e.HP
	}

	snap := func() step {
		pl := vm.Slots[0]
		r := step{PC: pl.PC, Upper: int16(pl.Actor.Upper.Frame), Lower: int16(pl.Actor.Lower.Frame), Dx: G.Dx, X: pl.X}
		if extended {
			for k := range r.E {
				r.E[k] = noEnemy
				e := vm.Slots[k+1]
				switch {
				case e.Actor == nil:
				case k == 3:
					r.E[k] = [4]int{e.PC, int(e.X), int(e.Actor.Upper.Frame), int(e.Actor.State)}
				default:
					r.E[k] = [4]int{e.PC, int(e.X), int(e.Actor.Upper.Frame), int(e.Actor.Lower.Frame)}
				}
			}
			r.Lives, r.Shooter, r.Outcome = G.Lives, G.Shooter, G.Outcome
			if withViewY {
				r.ViewY = G.ViewY
			}
		}
		return r
	}
	ours := []step{snap()}
	var stopped error
	seconds := tr.Seconds
	if seconds == 0 { // older captures without a duration
		seconds = 4
	}
	for f := 0; f < int(50*seconds) && !s.Over(); f++ {
		s.Step(in)
		if err := s.Halted; err != nil {
			// Unported opcodes/routines end the comparison; real mismatches fail below.
			var u script.ErrUnimplemented
			var r script.ErrUnmapped
			if !errors.As(err, &u) && !errors.As(err, &r) {
				t.Fatalf("frame %d: %v", f, err)
			}
			stopped = fmt.Errorf("frame %d: %w", f, err)
			break
		}
		if r := snap(); r != ours[len(ours)-1] {
			ours = append(ours, r)
		}
	}
	// Every recorded step must appear in the port's steps in the same order. The capture
	// polls a running emulator and sometimes misses a frame, so up to maxMissed port steps
	// may lie between two recorded ones. The port may run past the end of the capture.
	const maxMissed = 2
	j, missed := 0, 0
	for i, want := range orig {
		k := j
		for k < len(ours) && k-j <= maxMissed && ours[k] != want {
			k++
		}
		if k >= len(ours) {
			if stopped == nil {
				t.Errorf("port produced %d steps; recorded step %d of %d not reached", len(ours), i, len(orig))
			} else {
				t.Logf("%d of %d recorded steps match the original", i, len(orig))
			}
			break
		}
		if ours[k] != want {
			t.Fatalf("step %d differs:\n original %+v\n port     %+v", i, want, ours[j])
		}
		missed += k - j
		j = k + 1
		if i == len(orig)-1 {
			t.Logf("%d of %d recorded steps match the original (%d frames not sampled)", len(orig), len(orig), missed)
		}
	}
	if stopped != nil {
		t.Logf("comparison stopped early: %v", stopped)
	}
}
