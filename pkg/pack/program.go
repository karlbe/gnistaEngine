package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/game/script/asm"
)

// programSpec is program.json: the entry points and tables of a program written in the pack's
// scripts/*.gs files. Every script or data name is looked up in the assembled image.
type programSpec struct {
	Player         string       `json:"player"`
	Idle           [2][3]string `json:"idle"`
	LiftCall       string       `json:"lift_call"`
	LiftRecall     string       `json:"lift_recall"`
	Cabin          [4]string    `json:"cabin"`
	LiftHelper     [2][2]string `json:"lift_helper"`
	LiftExitEnd    string       `json:"lift_exit_end"`
	LiftExitFrames string       `json:"lift_exit_frames"`
	Charge         string       `json:"charge"`
	BlownUp        string       `json:"blown_up"`
	StepSounds     string       `json:"step_sounds"`
	StairSounds    string       `json:"stair_sounds"`
	ContactRight   string       `json:"contact_right"`
	ContactLeft    string       `json:"contact_left"`
	PlayerFrames   [2]uint16    `json:"player_frames"`
	MagSize        [3]uint8     `json:"mag_size"`
	FirstRight     [2]tmplSpec  `json:"first_right"`
	FirstLeft      [2]tmplSpec  `json:"first_left"`
	WaveRight      [3]tmplSpec  `json:"wave_right"`
	WaveLeft       [3]tmplSpec  `json:"wave_left"`
	WaveOrder      []uint8      `json:"wave_order"`
}

type tmplSpec struct {
	Start   string    `json:"start"`   // the script the enemy starts with
	Scripts [5]string `json:"scripts"` // main, after a shot, death, second death, hurt
}

// Program assembles the pack's own scripts (scripts/*.gs) and reads program.json. It returns
// nil if the pack has no scripts. docs/script-reference.md describes the language.
func (p *Pack) Program() (*script.Program, error) {
	files, _ := filepath.Glob(p.path("scripts", "*.gs"))
	if len(files) == 0 {
		return nil, nil
	}
	sort.Strings(files)
	src := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		src[filepath.Base(f)] = string(b)
	}
	img, err := asm.Assemble(src)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p.path("program.json"))
	if err != nil {
		return nil, fmt.Errorf("scripts need a program.json: %w", err)
	}
	var ps programSpec
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ps); err != nil {
		return nil, fmt.Errorf("program.json: %w", err)
	}
	var bad []string
	at := func(name string) int {
		if name == "" {
			return 0
		}
		a, ok := img.Addr[name]
		if !ok {
			bad = append(bad, name)
		}
		return a
	}
	tmpl := func(t tmplSpec) script.Template {
		out := script.Template{PC: at(t.Start)}
		for i, s := range t.Scripts {
			out.Scripts[i] = at(s)
		}
		return out
	}
	sp := script.Spec{
		Player: at(ps.Player), LiftCall: at(ps.LiftCall), LiftRecall: at(ps.LiftRecall),
		LiftExitEnd: at(ps.LiftExitEnd), LiftExitFrames: at(ps.LiftExitFrames),
		Charge: at(ps.Charge), BlownUp: at(ps.BlownUp),
		StepSounds: at(ps.StepSounds), StairSounds: at(ps.StairSounds),
		ContactRight: at(ps.ContactRight), ContactLeft: at(ps.ContactLeft),
		PlayerFrames: ps.PlayerFrames, MagSize: ps.MagSize, WaveOrder: ps.WaveOrder,
	}
	for f := range ps.Idle {
		for w := range ps.Idle[f] {
			sp.Idle[f][w] = at(ps.Idle[f][w])
		}
	}
	for i := range ps.Cabin {
		sp.Cabin[i] = at(ps.Cabin[i])
	}
	for i := range ps.LiftHelper {
		sp.LiftHelper[i] = [2]int{at(ps.LiftHelper[i][0]), at(ps.LiftHelper[i][1])}
	}
	for i := range ps.FirstRight {
		sp.FirstRight[i], sp.FirstLeft[i] = tmpl(ps.FirstRight[i]), tmpl(ps.FirstLeft[i])
	}
	for i := range ps.WaveRight {
		sp.WaveRight[i], sp.WaveLeft[i] = tmpl(ps.WaveRight[i]), tmpl(ps.WaveLeft[i])
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("program.json names things the scripts do not define: %s", strings.Join(bad, ", "))
	}
	if sp.Player == 0 {
		return nil, fmt.Errorf("program.json: player is the first script and must be set")
	}
	if len(sp.WaveOrder) == 0 {
		sp.WaveOrder = []uint8{0}
	}
	return script.NewProgramImage(img.Code, img.Scripts, sp)
}
