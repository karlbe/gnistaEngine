package amiga

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// DiskFile is a file on an AmigaDOS disk.
type DiskFile struct {
	Path string // with "/" between directories
	Data []byte
}

// Disk is the contents of an AmigaDOS floppy image (ADF), old (OFS) or fast (FFS) file system.
type Disk struct {
	Volume string
	FFS    bool
	Files  []DiskFile
}

const (
	adfBlock     = 512
	adfDiskSize  = 1760 * adfBlock // a double density floppy
	secDirectory = 2
	secFile      = -3
)

// ReadADF reads the files from a floppy image. It understands the standard layout (root block in
// the middle of the disk, hash tables of 72 entries) and nothing else, which is what the game's
// disk is.
func ReadADF(img []byte) (*Disk, error) {
	if len(img) < adfDiskSize || len(img)%adfBlock != 0 {
		return nil, fmt.Errorf("an ADF image is %d bytes, this one is %d", adfDiskSize, len(img))
	}
	if len(img) > 2*adfDiskSize {
		return nil, fmt.Errorf("%d bytes is not a floppy image", len(img))
	}
	if string(img[:3]) != "DOS" {
		return nil, fmt.Errorf("not an AmigaDOS disk (it starts with %q)", img[:3])
	}
	d := &Disk{FFS: img[3]&1 != 0}
	blocks := len(img) / adfBlock
	block := func(n int) ([]byte, error) {
		if n <= 0 || n >= blocks {
			return nil, fmt.Errorf("block %d is outside the disk", n)
		}
		return img[n*adfBlock : (n+1)*adfBlock], nil
	}
	long := func(b []byte, i int) int { return int(binary.BigEndian.Uint32(b[i*4:])) }
	name := func(b []byte) string {
		n := min(int(b[432]), 30)
		return strings.ReplaceAll(string(b[433:433+n]), "/", "_")
	}

	root, err := block(blocks / 2)
	if err != nil || long(root, 0) != 2 {
		return nil, fmt.Errorf("no valid root block: not a standard AmigaDOS disk")
	}
	d.Volume = name(root)

	readFile := func(h []byte) ([]byte, error) {
		size := long(h, 81)
		if size < 0 || size > len(img) {
			return nil, fmt.Errorf("file size %d is not possible", size)
		}
		data := make([]byte, 0, size)
		seen := 0
		if d.FFS {
			for {
				for i := 0; i < long(h, 2); i++ {
					b, err := block(long(h, 77-i))
					if err != nil {
						return nil, err
					}
					data = append(data, b...)
				}
				ext := long(h, 126)
				if ext == 0 {
					break
				}
				if h, err = block(ext); err != nil {
					return nil, err
				}
				if seen++; seen > blocks {
					return nil, fmt.Errorf("the file's extension blocks go in a circle")
				}
			}
		} else {
			for n := long(h, 4); n != 0; {
				b, err := block(n)
				if err != nil {
					return nil, err
				}
				n = long(b, 4)
				length := min(long(b, 3), adfBlock-24)
				data = append(data, b[24:24+length]...)
				if seen++; seen > blocks {
					return nil, fmt.Errorf("the file's data blocks go in a circle")
				}
			}
		}
		if len(data) < size {
			return nil, fmt.Errorf("the file has %d bytes of data, its header says %d", len(data), size)
		}
		return data[:size], nil
	}

	visited := map[int]bool{}
	var walk func(dir []byte, path string) error
	walk = func(dir []byte, path string) error {
		for h := 0; h < 72; h++ {
			for n := long(dir, 6+h); n != 0; {
				if visited[n] {
					return fmt.Errorf("block %d is reached twice: the directory goes in a circle", n)
				}
				visited[n] = true
				e, err := block(n)
				if err != nil {
					return err
				}
				p := name(e)
				if path != "" {
					p = path + "/" + p
				}
				switch int(int32(binary.BigEndian.Uint32(e[508:]))) {
				case secDirectory:
					if err := walk(e, p); err != nil {
						return err
					}
				case secFile:
					data, err := readFile(e)
					if err != nil {
						return fmt.Errorf("%s: %w", p, err)
					}
					d.Files = append(d.Files, DiskFile{p, data})
				}
				n = long(e, 124)
			}
		}
		return nil
	}
	if err := walk(root, ""); err != nil {
		return nil, err
	}
	return d, nil
}

// File returns a file by path, or nil.
func (d *Disk) File(path string) []byte {
	for _, f := range d.Files {
		if f.Path == path {
			return f.Data
		}
	}
	return nil
}
