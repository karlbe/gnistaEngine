package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/karlbe/gnistaEngine/pkg/content"
)

// Extracting from the disk image gives the same data as the files the game already reads.
func TestExtractFromDisk(t *testing.T) {
	disks, _ := filepath.Glob(filepath.Join("..", "..", "assets-local", "raw", "*.adf"))
	if len(disks) == 0 {
		t.Skip("no disk image in assets-local/raw")
	}
	out := t.TempDir()
	if err := installDisk(disks[0], out); err != nil {
		t.Fatal(err)
	}
	if err := run(content.Loader{Root: out}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"extracted/code.bin", "extracted/level.bin", "extracted/ns.json", "adf/ns", "adf/RoomData"} {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("..", "..", "assets-local", filepath.FromSlash(f)))
		if err != nil {
			t.Skip("assets-local has not been extracted")
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs", f)
		}
	}
}

func TestExtractRefusesOtherDisks(t *testing.T) {
	if err := installDisk(filepath.Join(t.TempDir(), "nothing.adf"), t.TempDir()); err == nil {
		t.Error("a missing file should be an error")
	}
	junk := filepath.Join(t.TempDir(), "junk.adf")
	if err := os.WriteFile(junk, make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installDisk(junk, t.TempDir()); err == nil {
		t.Error("a file that is not a disk should be an error")
	}
}
