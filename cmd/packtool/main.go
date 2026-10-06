// Command packtool helps to make a game pack (see docs/packs.md):
//
//	packtool new <dir>        start a pack: palette, placeholder tiles, a small map to walk in
//	packtool check <dir>      load a pack the way the game does and report problems
//	packtool leakcheck <dir>  compare a pack with the original and report copies (needs assets-local)
//	packtool inventory        list what the original has and a pack may replace (needs assets-local)
//	packtool guide <dir>      write guide pictures (tile numbers, sizes) into <dir>/guides
package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "new":
		err = needDir(args, newPack)
	case "check":
		err = needDir(args, check)
	case "guide":
		err = needDir(args, guide)
	case "leakcheck":
		err = leakcheck(args)
	case "inventory":
		err = inventory(args)
	default:
		usage()
	}
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: packtool new|check|guide <dir>   packtool leakcheck|inventory [-assets dir] [dir]")
	os.Exit(2)
}

func needDir(args []string, f func(string) error) error {
	if len(args) != 1 {
		usage()
	}
	return f(args[0])
}
