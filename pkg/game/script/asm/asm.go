// Package asm assembles and disassembles the game's scripts in a text form, so that scripts
// can be written, read and kept in the repository as source (docs/script-reference.md).
//
//	; a comment
//	script walk_right
//	    frames 48 58        ; both parts' frame numbers
//	    legs_at 0 240
//	    stepr
//	    wait
//	    goto walk_right     ; scripts refer to each other by name
//
//	data steps 11 12 13 0   ; raw bytes (a zero-terminated list of sound effects, say)
//	mark here               ; inside a script: names the address of the next byte
package asm

import (
	"fmt"
	"sort"
	"strconv"
)

// Operand kinds.
const (
	kByte   = 'b' // 0-255
	kSigned = 's' // -128-127
	kScript = 'n' // a script: its name or number
)

type opInfo struct {
	op   int
	name string
	args string // one kind letter per operand byte
}

// ops is the instruction set in the order of the original's opcode numbers, which the engine
// still decodes. docs/script-reference.md explains each one.
var ops = []opInfo{
	{0, "wait", ""},
	{1, "frames", "bb"}, {2, "frames116", "bb"}, {3, "frames254", "bb"},
	{6, "legs_w", "b"}, {7, "legs_w116", "b"}, {8, "legs_w254", "b"}, {9, "legs_w394", "b"}, {10, "legs_622", "b"},
	{11, "body_w", "b"}, {12, "body_w116", "b"}, {13, "body_w254", "b"},
	{16, "both_394", "bb"}, {17, "legs_116", "b"}, {18, "legs_394", "b"}, {19, "body_254", "b"}, {20, "body_394", "b"},
	{21, "setbase", "bb"},
	{22, "stepr", ""}, {23, "stepl", ""}, {24, "stepu", ""}, {25, "stepd", ""},
	{26, "legs_at", "ss"}, {27, "body_at", "ss"},
	{28, "scroll", "ssb"}, {29, "scroll_stop", ""}, {30, "scroll_r", ""}, {31, "scroll_l", ""}, {32, "scroll_u", ""}, {33, "scroll_d", ""},
	{34, "goto", "n"}, {35, "call", "n"}, {36, "return", ""}, {37, "repeat", "bn"},
	{38, "face_right", ""}, {39, "face_left", ""},
	{40, "control_right", "nnnnnnnnnnnn"}, {41, "control_left", "nnnnnnnnnnnn"},
	{42, "resume_right", ""}, {43, "resume_left", ""},
	{44, "lift_choose", "nn"}, {45, "if_right_free", "n"}, {46, "if_left_free", "n"}, {47, "if_roll", "n"},
	{48, "cabin0", ""}, {49, "cabin1", ""}, {50, "cabin2", ""}, {51, "cabin3", ""},
	{52, "show", ""}, {53, "hide", ""}, {54, "cabin_hide", ""}, {55, "cabin_up", ""}, {56, "cabin_down", ""}, {57, "cabin_show", ""},
	{58, "lift_called", ""}, {59, "lift_busy", ""}, {60, "lift_gone", ""},
	{61, "lift_exit_check", ""}, {62, "lift_enter_up", ""}, {63, "lift_enter_down", ""},
	{64, "if_fire", "nn"}, {65, "if_fire_down", "nn"},
	{66, "take_hit", "bnnnn"}, {67, "hit_return", ""},
	{68, "shoot_right", ""}, {69, "shoot_left", ""}, {70, "shoot2_right", ""}, {71, "shoot2_left", ""},
	{72, "charge_place", ""}, {73, "charge_show", ""}, {74, "charge_blow", ""},
	{75, "revive", ""}, {76, "enemy_fire", ""}, {77, "deactivate", ""},
	{78, "fire_weapon", "nb"},
	{79, "step_sound", ""}, {80, "stair_sound", ""},
	{81, "sound0", "b"}, {82, "sound1", "b"}, {83, "sound2", "b"}, {84, "sound3", "b"},
	{85, "silence0", ""}, {86, "silence1", ""}, {87, "silence2", ""}, {88, "silence3", ""}, {89, "start1", ""},
	{90, "explode_end", ""},
}

var (
	byName = map[string]*opInfo{}
	byOp   = map[int]*opInfo{}
)

func init() {
	for i := range ops {
		byName[ops[i].name] = &ops[i]
		byOp[ops[i].op] = &ops[i]
	}
	// The original's waits 4, 5, 14 and 15 do the same as 0; the assembler never writes them.
	for _, op := range []int{4, 5, 14, 15} {
		byOp[op] = &opInfo{op, fmt.Sprintf("wait%d", op), ""}
	}
}

// Image is the result of assembling: the code, where each script starts, and the names.
type Image struct {
	Code    []byte
	Scripts []int          // address of each script, in definition order
	Number  map[string]int // script name -> script number
	Addr    map[string]int // script, data and mark names -> address in Code
}

// BaseAddress is where the code starts; address 0 is not a valid script address.
const BaseAddress = 0x10

type stmt struct {
	line   int
	fields []string
}

type scriptDef struct {
	name  string
	file  string
	stmts []stmt
}

type dataDef struct {
	name  string
	file  string
	line  int
	bytes []string
}

// item is a script or a block of data, in source order.
type item struct {
	s *scriptDef
	d *dataDef
}

// Assemble reads the sources (name -> text) in sorted order of name, so that script numbers do
// not depend on the order of a directory listing.
func Assemble(sources map[string]string) (*Image, error) {
	var names []string
	for n := range sources {
		names = append(names, n)
	}
	sort.Strings(names)

	items, err := buildItems(names, sources)
	if err != nil {
		return nil, err
	}

	img := &Image{Number: map[string]int{}, Addr: map[string]int{}}
	define := func(name, where string, addr int) error {
		if _, dup := img.Addr[name]; dup {
			return fmt.Errorf("%s: %q is defined twice", where, name)
		}
		img.Addr[name] = addr
		return nil
	}
	// Pass 1: sizes and addresses.
	pc := BaseAddress
	type placed struct {
		it   item
		addr int
	}
	var all []placed
	for _, it := range items {
		all = append(all, placed{it, pc})
		switch {
		case it.s != nil:
			if err := define(it.s.name, it.s.file, pc); err != nil {
				return nil, err
			}
			img.Number[it.s.name] = len(img.Scripts)
			img.Scripts = append(img.Scripts, pc)
			for _, st := range it.s.stmts {
				switch st.fields[0] {
				case "mark":
					if len(st.fields) != 2 {
						return nil, fmt.Errorf("%s:%d: mark needs a name", it.s.file, st.line)
					}
					if err := define(st.fields[1], fmt.Sprintf("%s:%d", it.s.file, st.line), pc); err != nil {
						return nil, err
					}
				default:
					op, ok := byName[st.fields[0]]
					if !ok {
						return nil, fmt.Errorf("%s:%d: unknown instruction %q", it.s.file, st.line, st.fields[0])
					}
					pc += 1 + len(op.args)
				}
			}
		default:
			if err := define(it.d.name, fmt.Sprintf("%s:%d", it.d.file, it.d.line), pc); err != nil {
				return nil, err
			}
			pc += len(it.d.bytes)
		}
	}
	img.Code = make([]byte, pc+1) // one spare byte so that a script ending at the end is still inside
	// Pass 2: bytes.
	for _, p := range all {
		at := p.addr
		if p.it.d != nil {
			for _, tok := range p.it.d.bytes {
				v, err := number(tok, kByte, img)
				if err != nil {
					return nil, fmt.Errorf("%s:%d: %v", p.it.d.file, p.it.d.line, err)
				}
				img.Code[at] = byte(v)
				at++
			}
			continue
		}
		for _, st := range p.it.s.stmts {
			if st.fields[0] == "mark" {
				continue
			}
			op := byName[st.fields[0]]
			args := st.fields[1:]
			if len(args) != len(op.args) {
				return nil, fmt.Errorf("%s:%d: %s takes %d operands, has %d", p.it.s.file, st.line, op.name, len(op.args), len(args))
			}
			img.Code[at] = byte(op.op)
			at++
			for i, tok := range args {
				v, err := number(tok, rune(op.args[i]), img)
				if err != nil {
					return nil, fmt.Errorf("%s:%d: %s operand %d: %v", p.it.s.file, st.line, op.name, i+1, err)
				}
				img.Code[at] = byte(v)
				at++
			}
		}
	}
	return img, nil
}

// number reads an operand of a kind.
func number(tok string, kind rune, img *Image) (int, error) {
	if kind == kScript {
		if n, ok := img.Number[tok]; ok {
			return n, nil
		}
	}
	v, err := strconv.ParseInt(tok, 0, 32)
	if err != nil {
		if kind == kScript {
			return 0, fmt.Errorf("no script named %q", tok)
		}
		return 0, fmt.Errorf("%q is not a number", tok)
	}
	switch kind {
	case kSigned:
		if v < -128 || v > 127 {
			return 0, fmt.Errorf("%d is not a signed byte", v)
		}
		return int(byte(int8(v))), nil
	default:
		if v < 0 || v > 255 {
			return 0, fmt.Errorf("%d is not a byte", v)
		}
		return int(v), nil
	}
}

// Instr is one decoded instruction.
type Instr struct {
	Addr int
	Op   int
	Name string
	Args []int // as written: signed operands are negative when they are
	Size int
}

// Decode reads instructions from code[start:end]. It fails on an unknown opcode or one that
// runs past end.
func Decode(code []byte, start, end int) ([]Instr, error) {
	var out []Instr
	for pc := start; pc < end; {
		op, ok := byOp[int(code[pc])]
		if !ok {
			return out, fmt.Errorf("%#x: unknown opcode %d", pc, code[pc])
		}
		size := 1 + len(op.args)
		if pc+size > end {
			return out, fmt.Errorf("%#x: %s runs past the end of the script", pc, op.name)
		}
		in := Instr{Addr: pc, Op: op.op, Name: op.name, Size: size}
		for i, k := range op.args {
			v := int(code[pc+1+i])
			if k == kSigned {
				v = int(int8(v))
			}
			in.Args = append(in.Args, v)
		}
		out = append(out, in)
		pc += size
	}
	return out, nil
}

// Format writes an instruction as source text. Script operands stay numbers.
func (in Instr) Format() string {
	s := in.Name
	for _, a := range in.Args {
		s += " " + strconv.Itoa(a)
	}
	return s
}

// Mnemonic describes one instruction for documentation: the opcode number, the name and the
// operand kinds (b byte, s signed byte, n script).
type Mnemonic struct {
	Op   int
	Name string
	Args string
}

// Mnemonics lists the instruction set.
func Mnemonics() []Mnemonic {
	out := make([]Mnemonic, len(ops))
	for i, o := range ops {
		out[i] = Mnemonic{o.op, o.name, o.args}
	}
	return out
}
