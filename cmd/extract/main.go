// Command extract prepares the game's data from your own copy of the original disk:
//
//	extract [-assets assets-local] DISK.adf|DISK.zip
//
// It reads every file off the disk image (an ADF, or a zip with an ADF in it) into
// <assets>/adf/, then reads the data tables out of the ns executable into <assets>/extracted/,
// so the game never has to parse the binary. Without a disk argument it only does the second
// step, from the files already in <assets>/adf/. Addresses are hunk-relative, as in
// docs/adf-inventory.md. Nothing of the original is kept anywhere but in <assets>, which is for
// your own use and is never part of the port.
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
	"github.com/karlbe/gnistaEngine/pkg/content"
)

const (
	addrLevel       = 0xA333 // block-id matrix
	addrLevelStride = 0x910E // word: matrix columns
	addrLevelRows   = 0x9110 // word: matrix rows (other tables follow the matrix)
	addrStartViewX  = 0x8F56 // word: initial view x in pixels
	addrStartViewY  = 0x8F58
	addrPalettePtr  = 0x7234 // long: current palette table, set with move.l #table,$7234.l
	addrGameplayPal = 0x7256 // table set when gameplay starts ($6950)
	paletteColours  = 32
	opMoveLImmAbsL  = 0x23FC
)

func main() {
	root := flag.String("assets", "assets-local", "asset root directory")
	flag.Parse()
	if flag.NArg() > 1 {
		log.Fatal("usage: extract [-assets dir] [DISK.adf|DISK.zip]")
	}
	if flag.NArg() == 1 {
		if err := installDisk(flag.Arg(0), *root); err != nil {
			log.Fatal(err)
		}
	}
	if err := run(content.Loader{Root: *root}); err != nil {
		log.Fatal(err)
	}
}

func run(l content.Loader) error {
	b, err := l.Disk("ns")
	if err != nil {
		return err
	}
	hunks, err := amiga.DecodeHunks(b)
	if err != nil {
		return err
	}
	code := hunks[0].Data
	be := binary.BigEndian

	cols := int(be.Uint16(code[addrLevelStride:]))
	rows := int(be.Uint16(code[addrLevelRows:]))
	if addrLevel+cols*rows > len(code) {
		return fmt.Errorf("level matrix %dx%d runs past the code hunk", cols, rows)
	}
	e := content.Extracted{
		LevelCols: cols,
		LevelRows: rows,
		StartX:    int(be.Uint16(code[addrStartViewX:])),
		StartY:    int(be.Uint16(code[addrStartViewY:])),
		Gameplay:  fmt.Sprintf("%04X", addrGameplayPal),
		Palettes:  map[string]amiga.Palette{},
	}
	// Every palette table the game installs: move.l #table,$7234.l
	for i := 0; i+10 <= len(code); i += 2 {
		if be.Uint16(code[i:]) == opMoveLImmAbsL && be.Uint32(code[i+6:]) == addrPalettePtr {
			t := int(be.Uint32(code[i+2:]))
			if t+paletteColours*2 <= len(code) {
				e.Palettes[fmt.Sprintf("%04X", t)] = amiga.PaletteFromWords(code[t:], paletteColours)
			}
		}
	}
	if _, ok := e.Palettes[e.Gameplay]; !ok {
		return fmt.Errorf("gameplay palette table $%s not found", e.Gameplay)
	}

	dir := filepath.Join(l.Root, "extracted")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "level.bin"), code[addrLevel:addrLevel+cols*rows], 0o644); err != nil {
		return err
	}
	// The script engine's bytecode and tables live in the code hunk itself.
	if err := os.WriteFile(filepath.Join(dir, "code.bin"), code, 0o644); err != nil {
		return err
	}
	j, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "ns.json"), j, 0o644); err != nil {
		return err
	}
	fmt.Printf("level %dx%d blocks, start view (%d,%d), %d palettes -> %s\n",
		cols, rows, e.StartX, e.StartY, len(e.Palettes), dir)
	return nil
}
