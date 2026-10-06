package asm

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/content"
)

func TestAssemble(t *testing.T) {
	img, err := Assemble(map[string]string{"a.gs": `
; two scripts that call each other
script first
    frames 48 58      ; both parts
    legs_at 0 -16
    mark here
    stepr
    goto second
data steps 11 12 0x0d 0
script second
    wait
    goto first
`})
	if err != nil {
		t.Fatal(err)
	}
	if len(img.Scripts) != 2 || img.Number["first"] != 0 || img.Number["second"] != 1 {
		t.Fatalf("scripts %v numbers %v", img.Scripts, img.Number)
	}
	a := img.Scripts[0]
	want := []byte{1, 48, 58, 26, 0, 0xF0, 22, 34, 1}
	if got := img.Code[a : a+len(want)]; string(got) != string(want) {
		t.Errorf("first = %v, want %v", got, want)
	}
	if img.Addr["here"] != a+6 {
		t.Errorf("mark at %#x, want %#x", img.Addr["here"], a+6)
	}
	d := img.Addr["steps"]
	if string(img.Code[d:d+4]) != string([]byte{11, 12, 13, 0}) {
		t.Errorf("data = %v", img.Code[d:d+4])
	}
	if s2 := img.Scripts[1]; s2 != d+4 || img.Code[s2] != 0 || img.Code[s2+1] != 34 || img.Code[s2+2] != 0 {
		t.Errorf("second at %#x: %v", s2, img.Code[s2:s2+3])
	}
}

func TestAssembleErrors(t *testing.T) {
	for name, src := range map[string]string{
		"unknown instruction": "script a\n  jump 1",
		"too few operands":    "script a\n  frames 1",
		"too many operands":   "script a\n  stepr 1",
		"byte too big":        "script a\n  frames 1 300",
		"unsigned as signed":  "script a\n  legs_at 200 0",
		"no such script":      "script a\n  goto nowhere",
		"outside a script":    "wait",
		"duplicate name":      "script a\n  wait\nscript a\n  wait",
		"bad data":            "data d 1 x",
	} {
		if _, err := Assemble(map[string]string{"t.gs": src}); err == nil {
			t.Errorf("%s: want an error", name)
		} else {
			t.Logf("%s: %v", name, err)
		}
	}
}

// Every one of the original's scripts must decode, instruction by instruction, to exactly the
// start of the next one: that proves the operand counts of the instruction set, and that
// assembling what was decoded gives the same bytes back.
func TestDecodeOriginal(t *testing.T) {
	l := content.Loader{Root: filepath.Join("..", "..", "..", "..", "assets-local")}
	prog, err := l.Program()
	if err != nil {
		t.Skip(err)
	}
	code, _ := l.Code()
	var starts []int
	for n := 0; n < 170; n++ {
		starts = append(starts, prog.Script(n))
	}
	sort.Ints(starts)
	total := 0
	for i := 0; i+1 < len(starts); i++ {
		ins, err := Decode(code, starts[i], starts[i+1])
		if err != nil {
			t.Fatalf("script at %#x: %v", starts[i], err)
		}
		// Re-encode with the original opcode numbers.
		var back []byte
		for _, in := range ins {
			back = append(back, byte(in.Op))
			for k, a := range in.Args {
				_ = k
				back = append(back, byte(a))
			}
		}
		if string(back) != string(code[starts[i]:starts[i+1]]) {
			t.Fatalf("script at %#x does not round-trip", starts[i])
		}
		total += len(ins)
	}
	t.Logf("%d instructions in %d scripts decode exactly", total, len(starts)-1)
	_ = strings.TrimSpace
	_ = os.Getenv
}

func TestFriendly(t *testing.T) {
	img, err := Assemble(map[string]string{"f.gs": `
const walk = 10          ; named values
sound pistol = 3
pose stand legs=0 body=64

script walk_right
    camera follow right
    repeat 2 times as i
        hold legs=walk+i body=64 for 2 ticks moving right
    end
    play pistol
    if holding right and path_clear goto walk_right
    camera stop
    show stand
    wait 2 ticks
    resume controls right
`})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		28, 1, 0, 0, // camera follow right: scroll 1 0 0
		1, 10, 64, 22, 0, 22, 0, // hold walk+0: frames, (stepr wait) x2
		1, 11, 64, 22, 0, 22, 0,
		81, 3, 85, // play pistol: sound0 3, silence0
		45, 0, // if holding right and path_clear goto walk_right (script 0)
		29,       // camera stop
		1, 0, 64, // show stand
		0, 0, // wait 2 ticks
		42, // resume controls right
	}
	a := img.Scripts[0]
	if got := img.Code[a : a+len(want)]; string(got) != string(want) {
		t.Errorf("code = %v\nwant   %v", got, want)
	}
}

func TestFriendlyErrors(t *testing.T) {
	for name, src := range map[string]string{
		"unknown pose":     "script a\n  show nobody",
		"no end":           "script a\n  repeat 2 times\n    wait",
		"stray end":        "script a\n  end",
		"unknown name":     "script a\n  wait lots",
		"bad direction":    "script a\n  move sideways 2",
		"controls missing": "script a\n  controls facing right right=a",
		"undefined const":  "const x = y\nscript a\n  wait",
	} {
		if _, err := Assemble(map[string]string{"t.gs": src}); err == nil {
			t.Errorf("%s: want an error", name)
		} else {
			t.Logf("%s: %v", name, err)
		}
	}
}

func TestControlsKeywords(t *testing.T) {
	img, err := Assemble(map[string]string{"c.gs": `
script idle
    controls facing right right=walk left=turn fire=shoot default=idle
script walk
    wait
script turn
    wait
script shoot
    wait
`})
	if err != nil {
		t.Fatal(err)
	}
	c := img.Code[img.Scripts[0]:]
	if c[0] != 40 || c[1] != 1 || c[2] != 2 || c[12] != 3 || c[3] != 0 {
		t.Errorf("control = %v", c[:13])
	}
}
