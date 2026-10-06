package pack

import (
	"image"
	"image/color"
	"sort"
)

// quantize reduces an image to at most n colours with median cut and returns the palette and
// the index of every pixel. It is for pictures with more colours than the renderer's 256.
func quantize(img image.Image, n int) ([]color.RGBA, []uint8) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	px := make([]color.RGBA, w*h)
	count := map[color.RGBA]int{}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := rgba(img.At(b.Min.X+x, b.Min.Y+y))
			c.A = 255
			px[y*w+x] = c
			count[c]++
		}
	}
	type box struct {
		cols []color.RGBA
		n    int // pixels
	}
	all := make([]color.RGBA, 0, len(count))
	for c := range count {
		all = append(all, c)
	}
	sort.Slice(all, func(i, j int) bool { // a fixed order, so the result does not vary from run to run
		a, b := all[i], all[j]
		if a.R != b.R {
			return a.R < b.R
		}
		if a.G != b.G {
			return a.G < b.G
		}
		return a.B < b.B
	})
	boxes := []box{{cols: all, n: len(px)}}
	for len(boxes) < n {
		// Split the box with the most pixels that still has more than one colour.
		bi := -1
		for i, bx := range boxes {
			if len(bx.cols) > 1 && (bi < 0 || bx.n > boxes[bi].n) {
				bi = i
			}
		}
		if bi < 0 {
			break
		}
		bx := boxes[bi]
		lo, hi := [3]int{255, 255, 255}, [3]int{}
		for _, c := range bx.cols {
			for k, v := range [3]int{int(c.R), int(c.G), int(c.B)} {
				lo[k], hi[k] = min(lo[k], v), max(hi[k], v)
			}
		}
		axis := 0 // the channel with the widest range
		for k := 1; k < 3; k++ {
			if hi[k]-lo[k] > hi[axis]-lo[axis] {
				axis = k
			}
		}
		val := func(c color.RGBA) int { return [3]int{int(c.R), int(c.G), int(c.B)}[axis] }
		sort.SliceStable(bx.cols, func(i, j int) bool { return val(bx.cols[i]) < val(bx.cols[j]) })
		half, acc := bx.n/2, 0
		cut := 1
		for i, c := range bx.cols {
			if acc += count[c]; acc >= half {
				cut = max(1, min(i+1, len(bx.cols)-1))
				break
			}
		}
		l, r := box{cols: bx.cols[:cut]}, box{cols: bx.cols[cut:]}
		for _, c := range l.cols {
			l.n += count[c]
		}
		r.n = bx.n - l.n
		boxes[bi] = l
		boxes = append(boxes, r)
	}
	pal := make([]color.RGBA, len(boxes))
	which := map[color.RGBA]uint8{}
	for i, bx := range boxes {
		var r, g, bl, total int
		for _, c := range bx.cols {
			k := count[c]
			r, g, bl, total = r+int(c.R)*k, g+int(c.G)*k, bl+int(c.B)*k, total+k
			which[c] = uint8(i)
		}
		pal[i] = color.RGBA{uint8(r / total), uint8(g / total), uint8(bl / total), 255}
	}
	idx := make([]uint8, len(px))
	for i, c := range px {
		idx[i] = which[c]
	}
	return pal, idx
}
