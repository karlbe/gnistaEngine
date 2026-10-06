// Command scriptasm works with scripts as text:
//
//	scriptasm asm <dir>          assemble dir/*.gs and print the scripts and their sizes
//	scriptasm dis N [N...]       print the original's scripts N (needs assets-local; the output
//	                             is derived from the original and must stay on this machine)
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/karlbe/gnistaEngine/pkg/content"
	"github.com/karlbe/gnistaEngine/pkg/game/script/asm"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 3 {
		log.Fatal("usage: scriptasm asm <dir> | dis N...")
	}
	switch os.Args[1] {
	case "asm":
		files, _ := filepath.Glob(filepath.Join(os.Args[2], "*.gs"))
		src := map[string]string{}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				log.Fatal(err)
			}
			src[filepath.Base(f)] = string(b)
		}
		img, err := asm.Assemble(src)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d scripts, %d bytes\n", len(img.Scripts), len(img.Code))
	case "dis":
		l := content.Loader{Root: "assets-local"}
		prog, err := l.Program()
		if err != nil {
			log.Fatal(err)
		}
		code, _ := l.Code()
		var starts []int
		for n := 0; n < 170; n++ {
			starts = append(starts, prog.Script(n))
		}
		sort.Ints(starts)
		for _, a := range os.Args[2:] {
			n, err := strconv.Atoi(a)
			if err != nil || n < 0 || n >= 170 {
				log.Fatalf("bad script %q", a)
			}
			s := prog.Script(n)
			end := s + 300
			if i := sort.SearchInts(starts, s+1); i < len(starts) {
				end = starts[i]
			}
			ins, err := asm.Decode(code, s, end)
			if err != nil {
				fmt.Println("error:", err)
			}
			fmt.Printf("script %d @%#x\n", n, s)
			for _, in := range ins {
				fmt.Printf("  %s\n", in.Format())
			}
		}
	}
}
