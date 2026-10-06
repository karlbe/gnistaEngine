package main

import (
	"archive/zip"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
)

// knownNS is the CRC-32 of the ns executable on the disk the port was made from (Persian Gulf
// Inferno, Innerprise 1989). Other versions of the game may have other addresses in them.
const knownNS = 0x37cd5ebb

// needed are the files the game cannot start without.
var needed = []string{
	"ns", "RoomData", "NSIBobs", "NSIIcons", "NSIMenu", "NSIAscii", "NSISound", "NSIMusicSound", "NSIA",
	"NSILoader", "IT", "TA", "PF", "ST", "ET", "EX", "WT", "HC", "EN", "BB", "IAZ", "DAS", "DBY",
}

// installDisk reads the disk image and writes its files to <assets>/adf/.
func installDisk(path, assets string) error {
	img, err := readImage(path)
	if err != nil {
		return err
	}
	d, err := amiga.ReadADF(img)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	fmt.Printf("disk %q: %d files\n", d.Volume, len(d.Files))
	var missing []string
	for _, n := range needed {
		if d.File(n) == nil {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("this is not the game's disk: %s are not on it", strings.Join(missing, ", "))
	}
	if crc := crc32.ChecksumIEEE(d.File("ns")); crc != knownNS {
		fmt.Printf("warning: the ns program on this disk is not the version the port was made for (crc %08x, expected %08x); the data it reads may be in other places\n", crc, uint32(knownNS))
	}
	out := filepath.Join(assets, "adf")
	for _, f := range d.Files {
		dst := filepath.Join(out, filepath.FromSlash(f.Path))
		if rel, err := filepath.Rel(out, dst); err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("a file on the disk is called %q: refusing to write outside %s", f.Path, out)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, f.Data, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("%d files written to %s\n", len(d.Files), out)
	return nil
}

// readImage reads an ADF, or the first ADF in a zip file.
func readImage(path string) ([]byte, error) {
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return os.ReadFile(path)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	for _, f := range z.File {
		if !strings.EqualFold(filepath.Ext(f.Name), ".adf") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(io.LimitReader(r, 4<<20))
	}
	return nil, fmt.Errorf("%s has no .adf file in it", path)
}
