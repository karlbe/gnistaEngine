// Package content is the single asset loader. Everything the game reads goes through a
// Loader whose root directory is configuration, so switching from the original data
// (assets-local/) to our own assets (assets/) does not touch any other code.
//
// Layout under the root:
//
//	adf/        files from the game disk, unchanged
//	extracted/  tables read out of the ns executable by cmd/extract
package content

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
	"github.com/karlbe/gnistaEngine/pkg/game/script"
	"github.com/karlbe/gnistaEngine/pkg/game/world"
)

type Loader struct {
	Root string
}

// Extracted is what cmd/extract writes to extracted/ns.json.
type Extracted struct {
	LevelCols int                      `json:"level_cols"`
	LevelRows int                      `json:"level_rows"`
	StartX    int                      `json:"start_view_x"` // initial view position in pixels
	StartY    int                      `json:"start_view_y"`
	Gameplay  string                   `json:"gameplay_palette"` // key into Palettes
	Palettes  map[string]amiga.Palette `json:"palettes"`         // by original table address
}

func (l Loader) path(parts ...string) string {
	return filepath.Join(append([]string{l.Root}, parts...)...)
}

// Disk returns a file from the game disk.
func (l Loader) Disk(name string) ([]byte, error) { return os.ReadFile(l.path("adf", name)) }

// DiskFiles lists the files on the game disk (top level only).
func (l Loader) DiskFiles() ([]string, error) {
	es, err := os.ReadDir(l.path("adf"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range es {
		if !e.IsDir() && !strings.Contains(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func (l Loader) ILBM(name string) (*amiga.ILBM, error) {
	b, err := l.Disk(name)
	if err != nil {
		return nil, err
	}
	img, err := amiga.DecodeILBM(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return img, nil
}

func (l Loader) Bank(name string) (*amiga.Bank, error) {
	b, err := l.Disk(name)
	if err != nil {
		return nil, err
	}
	bank, err := amiga.DecodeJBOB(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return bank, nil
}

func (l Loader) Sample(name string) (*amiga.Sample, error) {
	b, err := l.Disk(name)
	if err != nil {
		return nil, err
	}
	return amiga.DecodeSample(b)
}

func (l Loader) Font() (*amiga.Font, error) {
	b, err := l.Disk("NSIAscii")
	if err != nil {
		return nil, err
	}
	return amiga.DecodeFont(b)
}

func (l Loader) Extracted() (*Extracted, error) {
	b, err := os.ReadFile(l.path("extracted", "ns.json"))
	if err != nil {
		return nil, fmt.Errorf("%w (run: go run ./cmd/extract YOUR-DISK.adf)", err)
	}
	var e Extracted
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// Code returns the original code hunk (unrelocated), which holds the script bytecode.
func (l Loader) Code() ([]byte, error) {
	b, err := os.ReadFile(l.path("extracted", "code.bin"))
	if err != nil {
		return nil, fmt.Errorf("%w (run: go run ./cmd/extract YOUR-DISK.adf)", err)
	}
	return b, nil
}

// Program returns the original's script program with the Spec built from its code.
func (l Loader) Program() (*script.Program, error) {
	code, err := l.Code()
	if err != nil {
		return nil, err
	}
	prog, err := script.NewProgram(code)
	if err != nil {
		return nil, err
	}
	prog.Spec = OriginalSpec(code)
	return prog, nil
}

// Level combines the extracted block matrix with the block library in RoomData.
func (l Loader) Level() (*world.Level, error) {
	e, err := l.Extracted()
	if err != nil {
		return nil, err
	}
	cells, err := os.ReadFile(l.path("extracted", "level.bin"))
	if err != nil {
		return nil, err
	}
	blocks, err := l.Disk("RoomData")
	if err != nil {
		return nil, err
	}
	level, err := world.NewLevel(e.LevelCols, e.LevelRows, cells, blocks)
	if err != nil {
		return nil, err
	}
	code, err := l.Code()
	if err != nil {
		return nil, err
	}
	level.T = OriginalTables(code, e)
	return level, nil
}
