package main

import (
	"bytes"
	"flag"
	"fmt"

	"github.com/karlbe/gnistaEngine/pkg/amiga"
	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/pack"
)

// leakcheck compares a pack with the original game and reports what is the same: tiles,
// sprites and screens that are the same picture (also when recoloured, which is how a copy
// looks after a palette change), sound data that is the same samples, and script code with
// long runs of the same bytes. It finds copies, not pictures drawn over the original; for
// those, only how the art was made can vouch. It needs the original's files.
func leakcheck(args []string) error {
	fs := flag.NewFlagSet("leakcheck", flag.ExitOnError)
	root := fs.String("assets", "assets-local", "asset root directory (the original)")
	fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}
	l := content.Loader{Root: *root}
	p, err := pack.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	hits := 0
	say := func(format string, a ...any) { hits++; fmt.Printf("SAME: "+format+"\n", a...) }

	// Tiles.
	if tiles, err := p.Tiles(); err != nil {
		return err
	} else if tiles != nil {
		orig, err := l.Bank("NSIIcons")
		if err != nil {
			return err
		}
		for i := 0; i < tiles.Count(); i++ {
			if !tiles.Has(i) {
				continue
			}
			mine, _ := tiles.Tile(i)
			for j := range orig.Sprites {
				if o, err := orig.Tile(j); err == nil && samePicture(mine.Pix, o.Pix, nil, nil, 4) {
					say("pack tile %d is the original's tile %d", i, j)
				}
			}
		}
	}
	// Sprites.
	if sprites, err := p.Sprites(); err != nil {
		return err
	} else if sprites != nil {
		orig, err := l.Bank("NSIBobs")
		if err != nil {
			return err
		}
		for n := 0; n < 700; n++ {
			if !sprites.Has(n) {
				continue
			}
			mine, _ := sprites.Bob(n)
			for j, o := range orig.Sprites {
				if o != nil && o.W == mine.W && o.H == mine.H && samePicture(mine.Pix, o.Pix, mine.Mask, o.Mask, 4) {
					say("pack sprite %d is the original's sprite %d", n, j)
				}
			}
		}
	}
	// Screens.
	origScreens := map[string]*amiga.ILBM{}
	for _, n := range []string{"NSILoader", "IT", "TA", "PF", "ST", "ET", "EX", "WT", "HC", "EN", "NSIMenu", "BB"} {
		if img, err := l.ILBM(n); err == nil {
			origScreens[n] = img
		}
	}
	for i := 1; i <= 15; i++ {
		if img, err := l.ILBM(fmt.Sprintf("NSIRoom%d", i)); err == nil {
			origScreens[fmt.Sprintf("NSIRoom%d", i)] = img
		}
	}
	var mine []struct {
		name string
		img  *amiga.ILBM
	}
	for _, n := range p.Screens() {
		if img, err := p.Screen(n); err == nil && img != nil {
			mine = append(mine, struct {
				name string
				img  *amiga.ILBM
			}{"screens/" + n, img})
		}
	}
	for _, n := range p.Rooms() {
		if img, err := p.Room(n); err == nil && img != nil {
			mine = append(mine, struct {
				name string
				img  *amiga.ILBM
			}{fmt.Sprintf("rooms/%d", n), img})
		}
	}
	for _, m := range mine {
		for on, o := range origScreens {
			if o.W == m.img.W && o.H == m.img.H && samePicture(m.img.Pix, o.Pix, nil, nil, 8) {
				say("%s is the original's %s", m.name, on)
			}
		}
	}
	// Sounds: the same 64 samples in a row.
	if sounds, err := p.Sounds(); err != nil {
		return err
	} else if len(sounds) > 0 {
		seen := map[string]bool{}
		add := func(d []int8) {
			for i := 0; i+64 <= len(d); i++ {
				seen[string(asBytes(d[i:i+64]))] = true
			}
		}
		if s, err := l.Sample("NSISound"); err == nil {
			add(s.Data)
		}
		for _, n := range []string{"NSIMusicSound", "IAZ", "DAS", "DBY"} {
			if s, err := l.Sample(n); err == nil {
				add(s.Data)
			}
		}
		for id, e := range sounds {
			for i := 0; i+64 <= len(e.Data); i += 8 {
				w := asBytes(e.Data[i : i+64])
				if flat(w) {
					continue
				}
				if seen[string(w)] {
					say("sound %d has 64 samples in a row from the original at sample %d", id, i)
					break
				}
			}
		}
	}
	// Scripts: 32 bytes in a row that are not all alike.
	if prog, err := p.Program(); err != nil {
		return err
	} else if prog != nil {
		code, err := l.Code()
		if err != nil {
			return err
		}
		own := make([]byte, prog.Len())
		for i := range own {
			own[i] = prog.Byte(i)
		}
		const win = 32
		orig := map[string]bool{}
		for i := 0; i+win <= len(code); i++ {
			if w := code[i : i+win]; !lowEntropy(w) {
				orig[string(w)] = true
			}
		}
		for i := 0; i+win <= len(own); i++ {
			if w := own[i : i+win]; !lowEntropy(w) && orig[string(w)] {
				say("the scripts have %d bytes in a row from the original's code, at %#x", win, i)
				break
			}
		}
	}
	if hits == 0 {
		fmt.Println("no copies found")
	} else {
		fmt.Printf("%d things are the same as the original's\n", hits)
	}
	return nil
}

func asBytes(d []int8) []byte {
	b := make([]byte, len(d))
	for i, v := range d {
		b[i] = byte(v)
	}
	return b
}

func flat(b []byte) bool {
	for _, v := range b {
		if v != b[0] {
			return false
		}
	}
	return true
}

// lowEntropy is true for runs that hold little: waits and padding. Fewer than 8 different byte
// values, or one value for half of the bytes or more.
func lowEntropy(w []byte) bool {
	count := map[byte]int{}
	most := 0
	for _, v := range w {
		count[v]++
		most = max(most, count[v])
	}
	return len(count) < 8 || most*2 >= len(w)
}

// samePicture reports whether two pictures of the same size are the same up to a change of
// colours: pixels are equal in one exactly where they are equal in the other, and (with masks)
// the same pixels are drawn. Pictures with fewer than minColours colours are not compared,
// because flat pictures are all alike.
func samePicture(a, b []uint8, ma, mb []bool, minColours int) bool {
	if len(a) != len(b) || (ma != nil) != (mb != nil) {
		return false
	}
	ab, ba := map[uint8]uint8{}, map[uint8]uint8{}
	for i := range a {
		if ma != nil {
			if ma[i] != mb[i] {
				return false
			}
			if !ma[i] {
				continue
			}
		}
		if v, ok := ab[a[i]]; ok && v != b[i] {
			return false
		}
		if v, ok := ba[b[i]]; ok && v != a[i] {
			return false
		}
		ab[a[i]], ba[b[i]] = b[i], a[i]
	}
	return len(ab) >= minColours && !bytes.Equal(a, nil)
}
