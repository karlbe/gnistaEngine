package script

import "fmt"

// Template is an enemy's starting record: the script it starts with and its five others
// (after a shot, two deaths, hurt; see Enemy).
type Template struct {
	PC      int
	Scripts [5]int
}

// Spec is everything the engine and the game need to know about a program beyond its
// opcodes: which scripts they start by themselves and where the tables are. It is data, so a
// program can be the original's (pkg/content builds its Spec from the original code) or
// our own (docs/own-scripts.md). Script fields are addresses in the code image.
type Spec struct {
	Player int       // the player's first script
	Idle   [2][3]int // the player's idle script, by facing (0 right, 1 left) and weapon

	LiftCall   int    // starts the cabin when it is called
	LiftRecall int    // restarts the cabin when it is already there
	Cabin      [4]int // the cabin's scripts for op48-51
	// LiftHelper are, for op62 and op63, the helper's script and the actor's own, when the
	// lift is called from a lift tile.
	LiftHelper [2][2]int
	// LiftExitEnd is where the exit script ends (an op43), and LiftExitFrames the two frame
	// numbers its last pose uses; both are for the improvement that exits to the right.
	LiftExitEnd, LiftExitFrames int

	Charge, BlownUp int // the helper's script when it is a charge, and the player's when it goes off under him
	StepSounds      int // zero-terminated list of effects op79 cycles through
	StairSounds     int // the same for op80

	ContactRight, ContactLeft int // the player's script when an enemy catches him

	PlayerFrames [2]uint16 // the player's first frames (legs, upper body)
	MagSize      [3]uint8  // rounds in a magazine, per weapon

	FirstRight, FirstLeft [2]Template // the first enemy of a trigger, two kinds in turn
	WaveRight, WaveLeft   [3]Template // wave enemies, three kinds
	WaveOrder             []uint8     // which wave kind comes next, in turn; the list wraps
}

// NewProgramImage makes a program from a code image and the addresses of its scripts, for
// programs that are not the original's (see the assembler in pkg/game/script/asm).
func NewProgramImage(code []byte, scripts []int, spec Spec) (*Program, error) {
	for n, a := range scripts {
		if a <= 0 || a >= len(code) {
			return nil, fmt.Errorf("script %d points outside the code image: %#x", n, a)
		}
	}
	return &Program{code: code, scripts: scripts, Spec: spec}, nil
}
