// Package script is a port of the original game's actor script engine: a bytecode
// interpreter that runs one script per actor slot every frame (docs/re/script-engine.md).
// The scripts themselves are the original bytecode, loaded as data.
package script

import (
	"encoding/binary"
	"fmt"
)

// Hunk-relative addresses in the original executable.
const (
	ScriptTable = 0x2BBE // 170 pointers to scripts
	NumScripts  = 170
	NumSlots    = 5
	HelperSlot  = 4 // the lift cabin ($2BA0, actor $8128): $16A8, op48-60, op62/63
)

// Joystick bits as stored in $23CA by the input routine ($235C).
const (
	JoyNone  = 1 << 0
	JoyUp    = 1 << 1
	JoyRight = 1 << 2
	JoyDown  = 1 << 3
	JoyLeft  = 1 << 4
	JoyFire  = 1 << 5
)

// Tile ids used by the engine.
const (
	solidBelow = 11   // tile ids below this block movement (op45-47)
	liftA      = 0x29 // lift tiles (one per floor), checked by op44, op62/63 and $16A8
	liftB      = 0x2A
)

// Keyboard codes compared by op64 (F1-F3 select a weapon).
const keyF1, keyF3 = 0x50, 0x52

// Program is the bytecode (the code hunk) plus its script table.
type Program struct {
	code    []byte
	scripts []int
	Spec    Spec
}

func NewProgram(code []byte) (*Program, error) {
	if len(code) < ScriptTable+NumScripts*4 {
		return nil, fmt.Errorf("code hunk too small (%d bytes)", len(code))
	}
	p := &Program{code: code, scripts: make([]int, NumScripts)}
	for i := range p.scripts {
		a := int(binary.BigEndian.Uint32(code[ScriptTable+i*4:]))
		if a <= 0 || a >= len(code) {
			return nil, fmt.Errorf("script %d points outside the code hunk: %#x", i, a)
		}
		p.scripts[i] = a
	}
	return p, nil
}

// Byte returns the byte at a hunk-relative address (static data in the code hunk).
func (p *Program) Byte(a int) uint8 { return p.code[a] }

// Long returns the big-endian long at a hunk-relative address.
func (p *Program) Long(a int) int { return int(binary.BigEndian.Uint32(p.code[a:])) }

// Len is the size of the code hunk.
func (p *Program) Len() int { return len(p.code) }

// Script returns the address of script n.
func (p *Program) Script(n int) int { return p.scripts[n] }

// Sprite is one of an actor's two drawn parts (upper and lower body).
type Sprite struct {
	X, Y  int16
	Frame uint16
}

// Actor is the drawable state an actor slot points to.
type Actor struct {
	Upper, Lower Sprite // original offsets +0/+2/+4 and +$18/+$1A/+$1C
	State        uint16 // +6 (4 = removed)
}

// Sensor indices into Slot.Sensors (original slot offsets $16-$1B).
const (
	SensHere      = iota // tile the actor moved into
	SensBelow            // tile under it
	SensAbove2           // two rows up
	SensSide             // next tile in the facing direction
	SensBelowSide        // one row down, two tiles ahead
	SensSide2            // two tiles ahead
)

// Slot is one actor slot (30 bytes at $2B28 + n*$1E in the original).
type Slot struct {
	X, Y         int16  // +0, +2: pixel position
	XMask, YMask uint16 // +4, +6: one bit per pixel within the current tile
	PC           int    // +$0C: script position
	Actor        *Actor // +$10: nil = slot unused
	TileX, TileY int    // +$14, +$15: tile position in the view
	Sensors      [6]uint8
	Dead         bool  // +$1C: set when an enemy is killed (op68/69 skip it)
	Enemy        Enemy // +8: the enemy's record (a copy of its spawn template)
}

// Env connects the engine to the rest of the game.
type Env interface {
	Joystick() uint8                    // $23CA
	LiftExitRequest() (any, right bool) // not in the original: fire, left or right was pressed recently (right: it was right); reading it clears it
	TakeRollRequest() bool              // not in the original: down was pressed together with a direction and has not been used; reading it clears it
	LastKey() uint8                     // $69AD
	Tile(x, y int) uint8                // level tile id at world tile (x, y)
	HelperActor() *Actor                // actor data for the stair helper ($8128)
	Sound(routine int, id uint8)        // play effect id from the table at $6432 via routine $64FE/$652C/$655A/$6588
	SilenceChannel(ch int)              // op85-88: let audio channel ch stop after the current sample
	EnableChannel(ch int)               // op89: switch audio channel ch's DMA on
	Door()                              // $23CC: up at a door (tiles $26-$28); the game shows the room
	DoorBlown()                         // op74: the room record for the player's cell is marked blown open
	MatrixCols() int                    // width of the level's block matrix
	LiftCard(blockCol int) uint8        // card bits ($505) needed to call a lift in this block column of the level; 0 for none
}

// Ammo is one weapon's ammunition record ($4F6 + weapon*4).
type Ammo struct {
	Rounds, Mags, MagSize, HUD uint8
}

// Globals are the engine variables shared by all slots.
type Globals struct {
	ViewX, ViewY    int16 // $8F56, $8F58: view position in level pixels
	Dx, Dy          int16 // $8F5A, $8F5C: per-frame scroll / movement
	MoveCount       uint8 // $8F5F: frames to wait between scroll steps
	MoveWait        uint8 // $8F5E: frames left to wait
	Moving          uint8 // $8F60
	FrameBaseUpper  int16 // $EA0
	FrameBaseLower  int16 // $EA2
	Weapon          int16 // low word of $4F2: 0-2, selects the sprite set via the frame bases
	Ammo            [3]Ammo
	SoundCycle      int   // $20CE: position in the zero-terminated effect list at $20C2 (op79)
	StairSoundCycle int   // $2120: the same for the list at $2114 (op80)
	WeaponsOwned    uint8 // $504: bit n = weapon n available
	Charges         int16 // $502: explosive charges left
	HUDDirty        uint8 // $507: bits ask the HUD to redraw (bit 0 = charges)
	MoveBits        uint8 // $506: bit 0 = walking forward, bit 1 = facing left
	Looping         bool  // $117E != $FF: an op37 loop is running
	LoopLeft        uint8 // $117E while Looping: repeats left
	Climbing        int16 // $C34: set by op40 for up/down
	Blocked         int16 // $C1C: when set, op40 ignores up and takes the third down branch
	Return          int   // $1130: gosub return address
	Resume          int   // $17DC: op40's own address, used by op42
	ResumeLeft      int   // $17E0: op41's own address, used by op43
	FireLatch       bool  // $1A24
	Shooter         int   // $1BFC: slot of the enemy whose shot hits the player next (0 = none)
	HitResult       int16 // $1C00: 0 = no effect, 1 = lost a life, 2 = dead
	Lives           int16 // $1C02: "HITS" left
	Damage          uint8 // $1C04: damage of the last op66
	Health          int8  // $1C05: damage taken before the next life is lost
	HitResume       int   // $1C10: the script position after the last op66 (op67)
	Cell            int   // $4EE: level matrix cell index, follows the view's block position
	Cards           uint8 // $505: bit n = elevator/door card n collected
	LiftBusy        bool  // $17C6 bit 0: the lift cabin is in use
	LiftCalled      bool  // $17C6 bit 1 (set by op58, cleared by op60)
	DownFlag        bool  // $18BC
	Outcome         int16 // $21EA
	Invulnerable    bool  // cheat, not in the original: op66 takes no damage
	LiftExitRight   bool  // improvement: the lift was left with right, so the exit ends facing right
}

// VM runs the actor scripts.
type VM struct {
	Prog  *Program
	Slots [NumSlots]Slot
	G     Globals
	Env   Env

	// Improve switches on the changes that are not in the original: leaving a lift with
	// left or right too, leaving it facing right when right was chosen, and taking stairs
	// whichever way the player faces. See the places that test it.
	Improve bool
}

// ErrUnimplemented is returned for opcodes that are not mapped yet.
type ErrUnimplemented struct {
	Op, PC, Slot int
}

func (e ErrUnimplemented) Error() string {
	return fmt.Sprintf("slot %d: opcode %d at $%04X not implemented", e.Slot, e.Op, e.PC)
}

// maxOpsPerSlot guards against scripts that never yield.
const maxOpsPerSlot = 10000

// Frame runs every active slot until it yields, like the actor loop at $C36, then scrolls
// the view by (dx, dy) like $892C. The scroll comes after the scripts, so sensors read
// during a frame still see the previous view position.
//
// A slot that reaches an unported opcode stays on it and the frame goes on; the first
// such error is returned.
func (vm *VM) Frame() error {
	var first error
	for i := range vm.Slots {
		s := &vm.Slots[i]
		if s.Actor == nil {
			continue
		}
		if err := vm.run(i, s); err != nil && first == nil {
			first = err
		}
	}
	vm.scroll()
	return first
}

// Level geometry used for $4EE: block size in tiles.
const blockW, blockH = 20, 4

func viewBlock(v int16, tiles int) int { return floorDiv(floorDiv(int(v), 16), tiles) }

// scroll is $8888: every (MoveCount+1)th frame the view moves by (dx, dy). Unless the
// movement was set with op28 (Moving), it lasts one step and dx/dy are cleared.
func (vm *VM) scroll() {
	if vm.G.MoveWait > 0 {
		vm.G.MoveWait--
		return
	}
	vm.G.MoveWait = vm.G.MoveCount
	bx, by := viewBlock(vm.G.ViewX, blockW), viewBlock(vm.G.ViewY, blockH)
	vm.G.ViewX += vm.G.Dx
	vm.G.ViewY += vm.G.Dy
	// $892C keeps $4EE in step with the view's block position.
	vm.G.Cell += viewBlock(vm.G.ViewX, blockW) - bx + (viewBlock(vm.G.ViewY, blockH)-by)*vm.Env.MatrixCols()
	if vm.G.Moving&1 == 0 {
		vm.G.Dx, vm.G.Dy = 0, 0
	}
}

// tile returns the level tile at the slot's tile position (relative to the view, like
// $FFC) plus (dx, dy).
func (vm *VM) tile(s *Slot, dx, dy int) uint8 {
	return vm.Env.Tile(floorDiv(int(vm.G.ViewX), 16)+s.TileX+dx, floorDiv(int(vm.G.ViewY), 16)+s.TileY+dy)
}

func floorDiv(a, b int) int {
	if a < 0 {
		return -((-a + b - 1) / b)
	}
	return a / b
}

func (vm *VM) run(slot int, s *Slot) error {
	code := vm.Prog.code
	arg := func(k int) uint8 { return code[s.PC+k] }
	gotoArg := func(k int) { s.PC = vm.Prog.Script(int(arg(k))) }
	for n := 0; n < maxOpsPerSlot; n++ {
		at := s.PC
		op := int(code[at])
		s.PC++
		switch op {
		case 0, 4, 5, 14, 15: // yield until next frame
			return nil
		case 1:
			vm.setFrames(s, 0)
		case 2:
			vm.setFrames(s, 0x74)
		case 3:
			vm.setFrames(s, 0xFE)
		case 6, 7, 8, 9, 10, 11, 12, 13, 17, 18, 19, 20:
			f := singleFrameOps[op]
			sp, base := &s.Actor.Upper, vm.G.FrameBaseUpper
			if f.lower {
				sp, base = &s.Actor.Lower, vm.G.FrameBaseLower
			}
			if f.weapon {
				sp.Frame = vm.dirFrame(arg(0), f.offset, base)
			} else {
				sp.Frame = uint16(arg(0)) + f.offset
			}
			s.PC++
		case 16:
			s.Actor.Upper.Frame = uint16(arg(0)) + 0x18A
			s.Actor.Lower.Frame = uint16(arg(1)) + 0x18A
			s.PC += 2
		case 21:
			vm.G.FrameBaseUpper, vm.G.FrameBaseLower = int16(arg(0)), int16(arg(1))
			s.PC += 2
		case 22:
			vm.step(s, 1, 0)
		case 23:
			vm.step(s, -1, 0)
		case 24:
			vm.step(s, 0, -1)
		case 25:
			vm.step(s, 0, 1)
		case 26:
			s.Actor.Upper.X = s.X + int16(int8(arg(0)))
			s.Actor.Upper.Y = s.Y + int16(int8(arg(1)))
			s.PC += 2
		case 27:
			s.Actor.Lower.X = s.X + int16(int8(arg(0)))
			s.Actor.Lower.Y = s.Y + int16(int8(arg(1)))
			s.PC += 2
		case 28:
			vm.G.Dx, vm.G.Dy = int16(int8(arg(0))), int16(int8(arg(1)))
			vm.G.MoveCount, vm.G.MoveWait, vm.G.Moving = arg(2), 0, 1
			s.PC += 3
		case 29:
			vm.G.Dx, vm.G.Dy, vm.G.MoveCount, vm.G.MoveWait, vm.G.Moving = 0, 0, 0, 0, 0
		case 30, 31, 32, 33: // right, left, up, down by one pixel per frame
			d := unitMoves[op-30]
			vm.G.Dx, vm.G.Dy, vm.G.Moving = d[0], d[1], 0
		case 34:
			gotoArg(0)
		case 35:
			vm.G.Return = s.PC + 1
			gotoArg(0)
		case 36:
			s.PC = vm.G.Return
		case 37: // $1134: goto arg(1), arg(0) times in a row, then fall through
			switch {
			case !vm.G.Looping:
				vm.G.Looping, vm.G.LoopLeft = true, arg(0)-1
				gotoArg(1)
			case vm.G.LoopLeft == 0:
				vm.G.Looping = false
				s.PC += 2
			default:
				vm.G.LoopLeft--
				gotoArg(1)
			}
		case 38: // $1180: face right
			vm.faceSensors(s, 1)
			vm.G.MoveBits &^= 2
		case 39: // $11AE: face left
			vm.faceSensors(s, -1)
			vm.G.MoveBits |= 2
		case 40, 41:
			yield, err := vm.control(slot, s, vm.controls(op == 41))
			if err != nil {
				s.PC = at
				return err
			}
			if yield {
				return nil
			}
		case 44:
			joy := vm.Env.Joystick()
			switch {
			case joy&JoyUp != 0 && isLift(vm.tile(s, 0, -4)):
				gotoArg(0)
			case joy&JoyDown != 0 && isLift(vm.tile(s, 0, 4)):
				gotoArg(1)
			default:
				s.PC += 2
			}
		case 45, 46:
			dir := map[int]uint8{45: JoyRight, 46: JoyLeft}[op]
			if vm.Env.Joystick()&dir != 0 && s.Sensors[SensSide] >= solidBelow {
				gotoArg(0)
			} else {
				s.PC++
			}
		case 47: // the roll. Looks once per 32-tick run stride; the buffered press (not in the original) makes a short tap count
			if s.Sensors[SensSide2] >= solidBelow && (vm.Env.Joystick()&JoyDown != 0 || vm.Env.TakeRollRequest()) {
				vm.G.DownFlag = true
				gotoArg(0)
			} else {
				s.PC++
			}
		case 61:
			if vm.liftExit() {
				vm.G.FireLatch = true
			}
		case 62, 63:
			if vm.G.FireLatch {
				vm.G.FireLatch = false
				break
			}
			if s.Sensors[SensHere] != liftA {
				break
			}
			// Addresses of the helper's and this actor's scripts.
			pair := vm.Prog.Spec.LiftHelper[op-62]
			helperPC, selfPC := pair[0], pair[1]
			h := &vm.Slots[HelperSlot]
			h.PC, h.Actor = helperPC, vm.Env.HelperActor()
			s.PC = selfPC
		case 64:
			joy := vm.Env.Joystick()
			k := vm.Env.LastKey()
			switch {
			case joy&JoyFire != 0:
				gotoArg(0)
			case k >= keyF1 && k <= keyF3:
				s.PC += 2
			case joy&JoyNone != 0:
				gotoArg(1)
			default:
				s.PC += 2
			}
		case 65:
			joy := vm.Env.Joystick()
			switch {
			case joy&JoyFire != 0:
				gotoArg(0)
			case joy&JoyDown != 0:
				gotoArg(1)
			default:
				s.PC += 2
			}
		case 66: // $1B18: take a hit from the enemy in $1BFC
			vm.op66(s)
		case 67: // $1C06: back to after the last op66
			s.PC = vm.G.HitResume
		case 48, 49, 50, 51: // $18D2-$1920: start the lift cabin with one of four scripts
			h := &vm.Slots[HelperSlot]
			h.PC = vm.Prog.Spec.Cabin[op-48]
			h.Actor = vm.Env.HelperActor()
		case 52, 53: // $193A, $194A: show (1) or hide (2) this actor's bobs
			s.Actor.State = uint16(op - 51)
		case 54: // $195A: hide the cabin
			vm.Env.HelperActor().State = 2
		case 55, 56: // $196A, $1980: move the cabin a floor up/down and show it
			a := vm.Env.HelperActor()
			a.Upper.Y += [2]int16{-64, 64}[op-55]
			a.State = 1
		case 57: // $1996: show the cabin
			vm.Env.HelperActor().State = 1
		case 58: // $19A6
			vm.G.LiftBusy, vm.G.LiftCalled = false, true
		case 59: // $19BA
			vm.G.LiftBusy = true
		case 60: // $19C6: the cabin is gone; ends this actor's frame
			vm.G.LiftBusy, vm.G.LiftCalled = false, false
			h := &vm.Slots[HelperSlot]
			h.X, h.Y, h.PC, h.Actor = 0, 0, 0, nil
			a := vm.Env.HelperActor()
			a.Upper.X, a.Upper.Y, a.Upper.Frame, a.State = 0, 0, 0, 2
			return nil
		case 72: // $1F5A: a charge (the helper slot) is placed at this actor
			h := &vm.Slots[HelperSlot]
			h.X, h.Y, h.PC = s.X, s.Y, vm.Prog.Spec.Charge
			h.Actor = vm.Env.HelperActor()
			h.Actor.Upper.X, h.Actor.Upper.Y = s.X, s.Y
		case 73: // $1F8A: show the charge
			vm.Env.HelperActor().State = 1
		case 74: // $1F9A: the charge goes off; ends this actor's frame
			if p := &vm.Slots[0]; p.X == s.X {
				p.PC = vm.Prog.Spec.BlownUp // the player stood on it
				return nil
			}
			s.Actor = nil
			vm.Env.HelperActor().State = 4
			vm.Env.DoorBlown()
			return nil
		case 75: // $1FE8: alive again, restart the actor's main script
			s.Dead = false
			s.PC = s.Enemy.Scripts[0]
		case 76: // $1FF6: shoot at the player, unless another enemy already has
			if vm.G.Shooter == 0 {
				vm.G.Shooter = slot
			}
		case 84:
			vm.Env.Sound(0x6588, arg(0))
			s.PC++
		case 88:
			vm.Env.SilenceChannel(3)
		case 89:
			vm.Env.EnableChannel(1)
		case 78: // fire: use a round, reload from a spare magazine, goto n when empty; then op81
			a := &vm.G.Ammo[vm.G.Weapon]
			if a.Rounds == 0 {
				if a.Mags == 0 {
					gotoArg(0)
					break
				}
				a.Mags--
				a.Rounds = a.MagSize
				a.HUD |= 1
			}
			a.Rounds--
			s.PC++
			vm.Env.Sound(0x64FE, arg(0))
			s.PC++
		case 79: // next effect from a cyclic list: footsteps
			vm.cycleSound(&vm.G.SoundCycle, vm.Prog.Spec.StepSounds)
		case 80: // the same with the list for stairs
			vm.cycleSound(&vm.G.StairSoundCycle, vm.Prog.Spec.StairSounds)
		case 81:
			vm.Env.Sound(0x64FE, arg(0))
			s.PC++
		case 82:
			vm.Env.Sound(0x652C, arg(0))
			s.PC++
		case 83:
			vm.Env.Sound(0x655A, arg(0))
			s.PC++
		case 42: // back to the last op40 (its address is saved in $17DC)
			s.PC = vm.G.Resume
		case 43: // back to the last op41 ($17E0)
			if vm.Improve && at == vm.Prog.Spec.LiftExitEnd {
				// The lift exit script (89) ends by turning left and returning to op41, so
				// the player always came out facing left.
				right := vm.G.LiftExitRight || vm.Env.Joystick()&JoyRight != 0
				vm.G.LiftExitRight = false
				if right {
					s.Actor.Upper.Frame = vm.dirFrame(vm.Prog.code[vm.Prog.Spec.LiftExitFrames], 0, vm.G.FrameBaseUpper)
					s.Actor.Lower.Frame = vm.dirFrame(vm.Prog.code[vm.Prog.Spec.LiftExitFrames+1], 0, vm.G.FrameBaseLower)
					s.PC = vm.G.Resume
					break
				}
			}
			s.PC = vm.G.ResumeLeft
		case 68, 69, 70, 71: // fire right/left ($1C14, $1CE0), and the same from scripts 12/15 ($1DAC, $1E78)
			if t := vm.target(slot, op&1 == 1); t >= 0 {
				death := EnemyDeath
				if op >= 70 {
					death = EnemyDeath2
				}
				vm.hitEnemy(t, death)
			}
		case 85, 86, 87:
			vm.Env.SilenceChannel(op - 85)
		case 77: // deactivate this actor
			s.Actor.State = 4
			s.Actor = nil
			return nil
		case 90: // ends the game: outcome 2 (explosion screen)
			vm.G.Outcome = 2
			return nil
		default:
			s.PC = at
			return ErrUnimplemented{Op: op, PC: at, Slot: slot}
		}
	}
	return fmt.Errorf("slot %d: no yield after %d opcodes (PC $%04X)", slot, maxOpsPerSlot, s.PC)
}

var unitMoves = [4][2]int16{{1, 0}, {-1, 0}, {0, -1}, {0, 1}}

// The end of the lift exit script (89): op43 at liftExitEnd, after op02 with the standing
// frames at liftExitFrames.

// liftExit is op61's test for leaving the lift at the next stop: fire in the original. With
// Improve, left and right do it too, and a short press counts (the ride does not look at
// all for the first 36 ticks of each floor).
func (vm *VM) liftExit() bool {
	joy := vm.Env.Joystick()
	if joy&JoyFire != 0 {
		return true
	}
	if !vm.Improve {
		return false
	}
	any, right := vm.Env.LiftExitRequest()
	if joy&JoyRight != 0 {
		any, right = true, true
	} else if joy&JoyLeft != 0 {
		any = true
	}
	if any {
		vm.G.LiftExitRight = right
	}
	return any
}

// cycleSound plays the next effect from a zero-terminated list (op79 $2080, op80 $20D2).
// pos 0 means the initial pointer: the original's start-up code ($3B0) sets it to start.
func (vm *VM) cycleSound(pos *int, start int) {
	if *pos == 0 {
		*pos = start
	}
	p := *pos
	*pos++
	if vm.Prog.code[p] == 0 {
		p = start
		*pos = p
	}
	vm.Env.Sound(0x64FE, vm.Prog.code[p])
}

// singleFrameOps set one sprite part's frame from one operand byte: frame = n + offset,
// plus the part's frame base times the weapon when weapon is set.
var singleFrameOps = map[int]struct {
	lower, weapon bool
	offset        uint16
}{
	6: {false, true, 0}, 7: {false, true, 0x74}, 8: {false, true, 0xFE}, 9: {false, true, 0x18A},
	10: {false, false, 0x26E}, 17: {false, false, 0x74}, 18: {false, false, 0x18A},
	11: {true, true, 0}, 12: {true, true, 0x74}, 13: {true, true, 0xFE},
	19: {true, false, 0xFE}, 20: {true, false, 0x18A},
}

func isLift(t uint8) bool { return t == liftA || t == liftB }

// dirFrame computes a frame number the way op01-03/07/13 do: n + offset + base*facing.
func (vm *VM) dirFrame(n uint8, offset uint16, base int16) uint16 {
	return uint16(n) + offset + uint16(base)*uint16(vm.G.Weapon)
}

// Raw Amiga key codes read from $69AD.
const (
	keySpace = 0x40
	keyF2    = 0x51
)

// ErrUnmapped is returned when a script reaches an original subroutine that is not ported.
type ErrUnmapped struct {
	Routine, Slot int
}

func (e ErrUnmapped) Error() string {
	return fmt.Sprintf("slot %d: original routine $%04X is not ported yet", e.Slot, e.Routine)
}

// control describes the two variants of the player's control opcode.
type control struct {
	left         bool   // facing left (op41)
	idle         [3]int // idle script per weapon after F1-F3
	upLo, downLo uint8  // first tile of the up (SensSide) and down (SensBelowSide) ranges
	resume       *int   // where the opcode stores its own address for op42/op43
}

func (vm *VM) controls(left bool) control {
	if left {
		return control{true, vm.Prog.Spec.Idle[1], 0x16, 0x13, &vm.G.ResumeLeft}
	}
	return control{false, vm.Prog.Spec.Idle[0], 0x0B, 0x1B, &vm.G.Resume}
}

// control is the player's control opcode: op40 facing right ($11DC/$1384), op41 facing
// left ($1442/$15EA). It is followed by 12 script numbers:
//
//	0 walk/turn right   1 walk/turn left   2/3 up variants   6 place charge
//	7/8/9 down variants   10 after op47 set the down flag   11 fire
//
// It returns yield=true when the slot must stop for this frame (weapon change).
func (vm *VM) control(slot int, s *Slot, c control) (yield bool, err error) {
	switch k := vm.Env.LastKey(); {
	case k == keyF1 || (k >= keyF2 && k <= keyF3 && vm.G.WeaponsOwned&(1<<(k-keyF1)) != 0):
		vm.G.Weapon = int16(k - keyF1)
		s.PC = c.idle[vm.G.Weapon]
		return true, nil
	}
	ops := s.PC // operand base (a0)
	arg := func(i int) int { return int(vm.Prog.code[ops+i]) }
	gotoN := func(i int) { s.PC = vm.Prog.Script(arg(i)) }
	vm.G.Climbing = 0
	if c.left {
		vm.G.MoveBits |= 2
	} else {
		vm.G.MoveBits &^= 2
	}
	*c.resume = ops - 1
	if vm.G.DownFlag {
		vm.G.DownFlag = false
		gotoN(10)
		return false, nil
	}
	s.PC = ops + 12
	inRange := func(v uint8, lo, n uint8) bool { return v-lo < n }
	joy := vm.Env.Joystick()
	forward, back, fwdN, backN := uint8(JoyRight), uint8(JoyLeft), 0, 1
	if c.left {
		forward, back, fwdN, backN = JoyLeft, JoyRight, 1, 0
	}
	switch {
	case vm.Env.LastKey() == keySpace:
		if inRange(s.Sensors[SensHere], 0x26, 4) && vm.G.Charges > 0 {
			vm.G.Charges--
			vm.G.HUDDirty |= 1
			gotoN(6)
		}
	case joy&forward != 0:
		if s.Sensors[SensSide] >= solidBelow {
			vm.G.MoveBits |= 1
			gotoN(fwdN)
		}
	case joy&back != 0: // turn around
		vm.G.MoveBits ^= 2
		gotoN(backN)
	case joy&JoyUp != 0:
		vm.G.MoveBits &^= 1
		if vm.G.Blocked != 0 {
			break
		}
		vm.G.Climbing = 1
		switch {
		case inRange(s.Sensors[SensHere], 0x26, 3):
			vm.Env.Door() // $23CC: enter the room behind the door
		case inRange(s.Sensors[SensSide], c.upLo, 5):
			gotoN(2)
		case inRange(s.Sensors[SensAbove2], 0x21, 5):
			gotoN(3)
		case vm.oppositeStairs(s, c, false):
		default:
			vm.callLift(s, arg(4), arg(5))
		}
	case joy&JoyDown != 0:
		vm.G.MoveBits &^= 1
		if vm.G.Blocked != 0 {
			gotoN(9)
			break
		}
		vm.G.Climbing = 1
		switch {
		case inRange(s.Sensors[SensBelowSide], c.downLo, 3):
			gotoN(7)
		case inRange(s.Sensors[SensBelow], 0x1E, 3):
			gotoN(8)
		case vm.oppositeStairs(s, c, true):
		default:
			gotoN(9)
		}
	case joy&JoyFire != 0:
		vm.G.MoveBits &^= 1
		gotoN(11)
	default:
		vm.G.MoveBits &^= 1
	}
	return false, nil
}

// oppositeStairs is not in the original. The control opcode only looks for stairs on the
// side the player faces, so up or down at the foot of stairs that go the other way did
// nothing (or ducked) until the player had turned round. With Improve it looks at the other
// side as well, with the other control's tile ranges, and if the stairs are there it turns
// the player and starts the same script that opcode would have (operand 2 up, 7 down).
func (vm *VM) oppositeStairs(s *Slot, c control, down bool) bool {
	if !vm.Improve {
		return false
	}
	other := vm.controls(!c.left)
	dir := -1 // the side the other control looks at
	if c.left {
		dir = 1
	}
	ops := other.idle[vm.G.Weapon] + 1 // the other control's operands, in this weapon's idle script
	operand := 2
	if down {
		operand = 7
		if v := vm.tile(s, 2*dir, 1); v-other.downLo >= 3 {
			return false
		}
	} else if v := vm.tile(s, dir, 0); v-other.upLo >= 5 {
		return false
	}
	if other.left {
		vm.G.MoveBits |= 2
	} else {
		vm.G.MoveBits &^= 2
	}
	*other.resume = ops - 1
	s.PC = vm.Prog.Script(int(vm.Prog.code[ops+operand]))
	return true
}

// setFrames implements op01-03: both frames from two operand bytes with a common offset.
func (vm *VM) setFrames(s *Slot, offset uint16) {
	code := vm.Prog.code
	s.Actor.Upper.Frame = vm.dirFrame(code[s.PC], offset, vm.G.FrameBaseUpper)
	s.Actor.Lower.Frame = vm.dirFrame(code[s.PC+1], offset, vm.G.FrameBaseLower)
	s.PC += 2
}

// faceSensors refreshes the sensors around the actor's own tile for facing dir (op38/39).
// Unlike step it leaves SensSide2 alone.
func (vm *VM) faceSensors(s *Slot, dir int) {
	t := func(x, y int) uint8 { return vm.tile(s, x, y) }
	s.Sensors[SensHere] = t(0, 0)
	s.Sensors[SensBelow] = t(0, 1)
	s.Sensors[SensAbove2] = t(0, -2)
	s.Sensors[SensSide] = t(dir, 0)
	s.Sensors[SensBelowSide] = t(2*dir, 1)
}

// step moves an actor one pixel (op22-25, routines $EF8-$FFA). The slot keeps one bit per
// pixel within the tile; when it runs out, the actor has entered a new tile and the
// sensors are refreshed relative to the slot's tile position.
func (vm *VM) step(s *Slot, dx, dy int) {
	s.X += int16(dx)
	s.Y += int16(dy)
	s.Actor.Upper.X += int16(dx)
	s.Actor.Lower.X += int16(dx)
	s.Actor.Upper.Y += int16(dy)
	s.Actor.Lower.Y += int16(dy)
	crossed := false
	switch {
	case dx > 0:
		if s.XMask >>= 1; s.XMask == 0 {
			s.XMask, crossed = 0x8000, true
		}
	case dx < 0:
		if s.XMask <<= 1; s.XMask == 0 {
			s.XMask, crossed = 0x0001, true
		}
	case dy < 0:
		if s.YMask <<= 1; s.YMask == 0 {
			s.YMask, crossed = 0x0001, true
		}
	default:
		if s.YMask >>= 1; s.YMask == 0 {
			s.YMask, crossed = 0x8000, true
		}
	}
	if !crossed {
		return
	}
	t := func(x, y int) uint8 { return vm.tile(s, x, y) }
	switch {
	case dx != 0: // horizontal: base is the next tile in the movement direction
		s.Sensors[SensHere] = t(dx, 0)
		s.Sensors[SensBelow] = t(dx, 1)
		s.Sensors[SensAbove2] = t(dx, -2)
		s.Sensors[SensSide] = t(2*dx, 0)
		s.Sensors[SensBelowSide] = t(3*dx, 1)
		s.Sensors[SensSide2] = t(3*dx, 0)
	default: // vertical: only the first three sensors
		s.Sensors[SensHere] = t(0, dy)
		s.Sensors[SensBelow] = t(0, dy+1)
		s.Sensors[SensAbove2] = t(0, dy-2)
	}
}

// callLift is $16A8: up while standing on a lift tile. Each third of the level (block
// columns 0-2, 3-5, 6-8) needs its card in $505; elsewhere no card is needed. A free
// cabin is placed at the actor and started with script $455E, and the actor continues
// with script a; after op58 the cabin restarts at $46A6 and the actor continues with b.
// Otherwise nothing happens.
func (vm *VM) callLift(s *Slot, a, b int) {
	if need := vm.Env.LiftCard(floorDiv(floorDiv(int(vm.G.ViewX), 16), 20)); need != 0 && vm.G.Cards&need == 0 {
		return
	}
	if !isLift(s.Sensors[SensHere]) || vm.G.LiftBusy {
		return
	}
	h := &vm.Slots[HelperSlot]
	if vm.G.LiftCalled {
		h.PC = vm.Prog.Spec.LiftRecall
		s.PC = vm.Prog.Script(b)
		return
	}
	vm.G.LiftBusy = true
	h.X, h.Y = s.X-8, s.Y-22
	h.Actor = vm.Env.HelperActor()
	h.Actor.Upper.X, h.Actor.Upper.Y = h.X, h.Y
	h.PC = vm.Prog.Spec.LiftCall
	s.PC = vm.Prog.Script(a)
}

// target finds the slot op68 (right) or op69 (left) hits: the nearest live actor in slots
// 1-3 on that side of the shooter, at any height. It returns -1 when there is none.
func (vm *VM) target(shooter int, left bool) int {
	px := vm.Slots[shooter].X
	best, bx := -1, uint16(0)
	for i := 1; i <= 3; i++ {
		t := &vm.Slots[i]
		if t.Actor == nil || t.Dead {
			continue
		}
		x := uint16(t.X)
		// The first candidate must be strictly on that side; later ones may also be level
		// with the shooter (the original's two loops use different comparisons).
		var ok bool
		switch {
		case best < 0 && !left:
			ok = t.X > px
		case best < 0:
			ok = t.X < px
		case !left:
			ok = t.X >= px && x <= bx
		default:
			ok = t.X <= px && x >= bx
		}
		if ok {
			best, bx = i, x
		}
	}
	return best
}

// Enemy is an enemy's record (+8 in its slot). Scripts are its main script (op75), the
// script after a shot or on contact, the death scripts for op68/69 and op70/71, and the
// script when hit; HP is the hits it still takes.
type Enemy struct {
	Scripts [5]int
	HP      uint16
}

// Enemy script indices.
const (
	EnemyMain = iota
	EnemyAfterShot
	EnemyDeath
	EnemyDeath2
	EnemyHurt
)

// hitEnemy is the end of op68-71 ($1CA4, $1F44): weapon 1 kills at once, otherwise the
// enemy loses a hit point while it has any. Its pending shot at the player is cancelled.
func (vm *VM) hitEnemy(t, death int) {
	e := &vm.Slots[t]
	if vm.G.Weapon != 1 && e.Enemy.HP != 0 {
		e.Enemy.HP--
		e.PC = e.Enemy.Scripts[EnemyHurt]
	} else {
		e.Dead = true
		e.PC = e.Enemy.Scripts[death]
	}
	if vm.G.Shooter == t {
		vm.G.Shooter = 0
	}
}

// op66 ($1B18) is followed by the damage and four scripts: hit from the right (a: lost a
// life, b: dead) and from the left (c, d). If an enemy has shot ($1BFC), it goes on with
// its after-shot script and the damage is taken ($1BA8): Health drops by the damage; when
// it reaches zero it is reset to 3 and a life is lost, or the player dies with no lives left.
func (vm *VM) op66(s *Slot) {
	code := vm.Prog.code
	vm.G.Damage = code[s.PC]
	ops := s.PC + 1
	s.PC += 5
	vm.G.HitResume = s.PC
	if vm.G.Shooter == 0 {
		return
	}
	sh := &vm.Slots[vm.G.Shooter]
	sh.PC = sh.Enemy.Scripts[EnemyAfterShot]
	vm.G.Shooter = 0
	if vm.G.Invulnerable {
		vm.G.HitResult = 0
		return
	}
	vm.G.Health -= int8(vm.G.Damage)
	switch {
	case vm.G.Health > 0:
		vm.G.HitResult = 0
	case vm.G.Lives != 0:
		vm.G.Health = 3
		vm.G.Lives--
		vm.G.HitResult = 1
		vm.G.HUDDirty |= 4
	default:
		vm.G.Health = 3
		vm.G.HitResult = 2
	}
	if vm.G.HitResult == 0 {
		return
	}
	k := int(vm.G.HitResult) - 1 // from the right
	if uint16(sh.X) < uint16(s.X) {
		k += 2 // from the left
	}
	s.PC = vm.Prog.Script(int(code[ops+k]))
}
