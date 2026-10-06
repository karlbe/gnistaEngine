package amiga

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildOFS makes a floppy image with the given files in its root directory (old file system).
func buildOFS(files map[string][]byte) []byte {
	img := make([]byte, adfDiskSize)
	copy(img, "DOS\x00")
	put := func(block, long int, v int32) {
		binary.BigEndian.PutUint32(img[block*adfBlock+long*4:], uint32(v))
	}
	setName := func(block int, s string) {
		b := img[block*adfBlock:]
		b[432] = byte(len(s))
		copy(b[433:], s)
	}
	root := adfDiskSize / adfBlock / 2
	put(root, 0, 2)
	put(root, 127, 1)
	put(root, 127-1+1, 1) // secondary type of the root: 1
	binary.BigEndian.PutUint32(img[root*adfBlock+508:], 1)
	setName(root, "TEST")
	next := root + 1
	slot := 6
	for name, data := range files {
		header := next
		next++
		put(header, 0, 2)
		put(header, 1, int32(header))
		put(header, 81, int32(len(data)))
		setName(header, name)
		binary.BigEndian.PutUint32(img[header*adfBlock+508:], uint32(0xFFFFFFFD)) // -3: a file
		prev := 0
		first := 0
		for off := 0; off < len(data) || (off == 0 && first == 0 && len(data) == 0); off += adfBlock - 24 {
			b := next
			next++
			n := min(adfBlock-24, len(data)-off)
			put(b, 0, 8)
			put(b, 1, int32(header))
			put(b, 3, int32(n))
			copy(img[b*adfBlock+24:], data[off:off+n])
			if prev != 0 {
				put(prev, 4, int32(b))
			} else {
				first = b
			}
			prev = b
			if len(data) == 0 {
				break
			}
		}
		put(header, 4, int32(first))
		put(root, slot, int32(header))
		slot++
	}
	return img
}

func TestReadADF(t *testing.T) {
	big := make([]byte, 5000) // more than a few data blocks
	for i := range big {
		big[i] = byte(i * 7)
	}
	d, err := ReadADF(buildOFS(map[string][]byte{"small": []byte("hello"), "big": big, "empty": nil}))
	if err != nil {
		t.Fatal(err)
	}
	if d.Volume != "TEST" || d.FFS {
		t.Errorf("volume %q ffs %v", d.Volume, d.FFS)
	}
	if got := d.File("small"); string(got) != "hello" {
		t.Errorf("small = %q", got)
	}
	if got := d.File("big"); !bytes.Equal(got, big) {
		t.Errorf("big has %d bytes, want %d, or the contents differ", len(got), len(big))
	}
	if got := d.File("empty"); got == nil || len(got) != 0 {
		t.Errorf("empty = %v", got)
	}
	if len(d.Files) != 3 {
		t.Errorf("%d files, want 3", len(d.Files))
	}
}

func TestReadADFErrors(t *testing.T) {
	good := buildOFS(map[string][]byte{"a": []byte("x")})
	short := good[:1000]
	notDOS := append([]byte(nil), good...)
	copy(notDOS, "XXXX")
	noRoot := append([]byte(nil), good...)
	binary.BigEndian.PutUint32(noRoot[(adfDiskSize/adfBlock/2)*adfBlock:], 0)
	for name, img := range map[string][]byte{"too short": short, "not DOS": notDOS, "no root": noRoot} {
		if _, err := ReadADF(img); err == nil {
			t.Errorf("%s: want an error", name)
		} else {
			t.Logf("%s: %v", name, err)
		}
	}
}

// The real disk gives the same files as the Python tool made (assets-local/adf).
func TestReadOriginalDisk(t *testing.T) {
	disks, _ := filepath.Glob(filepath.Join("..", "..", "assets-local", "raw", "*.adf"))
	if len(disks) == 0 {
		t.Skip("no disk image in assets-local/raw")
	}
	img, err := os.ReadFile(disks[0])
	if err != nil {
		t.Fatal(err)
	}
	d, err := ReadADF(img)
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, f := range d.Files {
		want, err := os.ReadFile(filepath.Join("..", "..", "assets-local", "adf", filepath.FromSlash(f.Path)))
		if err != nil {
			continue
		}
		if !bytes.Equal(f.Data, want) {
			t.Errorf("%s differs from the file extracted by the Python tool", f.Path)
		}
		compared++
	}
	if compared < 30 {
		t.Errorf("only %d files compared", compared)
	}
	t.Logf("%s: %d files, %d compared", d.Volume, len(d.Files), compared)
	if d.File("ns") == nil || d.File("RoomData") == nil {
		t.Error("ns or RoomData missing")
	}
}
